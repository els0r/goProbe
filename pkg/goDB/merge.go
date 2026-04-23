package goDB

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/els0r/goProbe/v4/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/fako1024/gotools/bitpack"
)

const defaultCompleteTolerance = 300 * time.Second

// MergeOptions controls merge behavior of source and destination goDB instances.
type MergeOptions struct {
	SourcePath      string
	DestinationPath string
	Interfaces      []string

	Overwrite         bool
	DryRun            bool
	CompleteTolerance time.Duration
}

// MergeSummary reports aggregate merge results.
type MergeSummary struct {
	DryRun bool

	InterfacesProcessed int

	DaysCopied  int
	DaysRebuilt int
	DaysSkipped int

	ConflictsResolvedByDestination int
	ConflictsResolvedBySource      int
}

type mergeDayAction int

const (
	mergeDayActionSkip mergeDayAction = iota
	mergeDayActionCopy
	mergeDayActionRebuild
)

type dayDescriptor struct {
	Timestamp int64
	Suffix    string
	DirName   string
	Path      string
	Complete  bool
}

type dayPlan struct {
	Action     mergeDayAction
	UseSource  bool
	UseDest    bool
	SourceDay  dayDescriptor
	HasDestDay bool
	DestDay    dayDescriptor
}

type blockSnapshot struct {
	Timestamp int64
	Traffic   gpfile.TrafficMetadata
	Counts    types.Counters
	Data      [types.ColIdxCount][]byte
}

// MergeDatabases merges source DB contents into destination DB according to MergeOptions.
func MergeDatabases(ctx context.Context, opts MergeOptions) (summary MergeSummary, err error) {
	summary.DryRun = opts.DryRun

	sourcePath := filepath.Clean(opts.SourcePath)
	destinationPath := filepath.Clean(opts.DestinationPath)
	if sourcePath == "" || destinationPath == "" {
		return summary, errors.New("source and destination paths must be non-empty")
	}
	if sourcePath == destinationPath {
		return summary, errors.New("source and destination paths must differ")
	}

	tolerance := opts.CompleteTolerance
	if tolerance <= 0 {
		tolerance = defaultCompleteTolerance
	}

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return summary, fmt.Errorf("failed to access source path `%s`: %w", sourcePath, err)
	}
	if !sourceStat.IsDir() {
		return summary, fmt.Errorf("source path `%s` is not a directory", sourcePath)
	}

	if err := os.MkdirAll(destinationPath, 0o755); err != nil {
		return summary, fmt.Errorf("failed to create destination path `%s`: %w", destinationPath, err)
	}

	sourceIfaces, err := listSourceInterfaces(sourcePath)
	if err != nil {
		return summary, fmt.Errorf("failed to list source interfaces: %w", err)
	}

	selectedIfaces, err := selectInterfaces(sourceIfaces, opts.Interfaces)
	if err != nil {
		return summary, err
	}
	if len(selectedIfaces) == 0 {
		return summary, nil
	}

	stageRoot, err := os.MkdirTemp(destinationPath, ".gpdb-merge-stage-*")
	if err != nil {
		return summary, fmt.Errorf("failed to create stage directory in `%s`: %w", destinationPath, err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(stageRoot); cleanupErr != nil && err == nil {
			err = fmt.Errorf("failed to cleanup staging directory `%s`: %w", stageRoot, cleanupErr)
		}
	}()

	for _, iface := range selectedIfaces {
		select {
		case <-ctx.Done():
			return summary, ctx.Err()
		default:
		}

		srcIfacePath := filepath.Join(sourcePath, iface)
		dstIfacePath := filepath.Join(destinationPath, iface)

		srcDays, err := listInterfaceDays(srcIfacePath)
		if err != nil {
			return summary, fmt.Errorf("failed to list source days for interface `%s`: %w", iface, err)
		}
		if len(srcDays) == 0 {
			continue
		}

		dstDays, err := listInterfaceDays(dstIfacePath)
		if err != nil {
			return summary, fmt.Errorf("failed to list destination days for interface `%s`: %w", iface, err)
		}

		dayTimestamps := mapKeysSorted(srcDays)
		summary.InterfacesProcessed++

		for _, dayTimestamp := range dayTimestamps {
			select {
			case <-ctx.Done():
				return summary, ctx.Err()
			default:
			}

			srcDay := srcDays[dayTimestamp]
			srcComplete, err := isDayComplete(srcIfacePath, srcDay, tolerance)
			if err != nil {
				return summary, fmt.Errorf("failed to classify source day `%s` for interface `%s`: %w", srcDay.Path, iface, err)
			}
			srcDay.Complete = srcComplete

			dstDay, hasDstDay := dstDays[dayTimestamp]
			if hasDstDay {
				dstComplete, err := isDayComplete(dstIfacePath, dstDay, tolerance)
				if err != nil {
					return summary, fmt.Errorf("failed to classify destination day `%s` for interface `%s`: %w", dstDay.Path, iface, err)
				}
				dstDay.Complete = dstComplete
			}

			plan := planDayMerge(srcDay, hasDstDay, dstDay, opts.Overwrite)
			if plan.Action == mergeDayActionSkip {
				summary.DaysSkipped++
				continue
			}

			if opts.DryRun {
				switch plan.Action {
				case mergeDayActionCopy:
					summary.DaysCopied++
				case mergeDayActionRebuild:
					summary.DaysRebuilt++
				}
				continue
			}

			switch plan.Action {
			case mergeDayActionCopy:
				stagedDayPath, err := stageCopyDay(stageRoot, iface, plan.SourceDay)
				if err != nil {
					return summary, fmt.Errorf("failed to stage day copy for interface `%s` day `%d`: %w", iface, dayTimestamp, err)
				}
				var existing *dayDescriptor
				if plan.HasDestDay {
					existing = &plan.DestDay
				}
				if err := commitStagedDay(stagedDayPath, dstIfacePath, dayTimestamp, existing); err != nil {
					return summary, fmt.Errorf("failed to commit copied day for interface `%s` day `%d`: %w", iface, dayTimestamp, err)
				}
				summary.DaysCopied++
			case mergeDayActionRebuild:
				stagedDayPath, conflictsDst, conflictsSrc, err := rebuildDayToStage(ctx, stageRoot, iface, srcIfacePath, dstIfacePath, dayTimestamp, plan, opts.Overwrite)
				if err != nil {
					return summary, fmt.Errorf("failed to rebuild day for interface `%s` day `%d`: %w", iface, dayTimestamp, err)
				}
				var existing *dayDescriptor
				if plan.HasDestDay {
					existing = &plan.DestDay
				}
				if err := commitStagedDay(stagedDayPath, dstIfacePath, dayTimestamp, existing); err != nil {
					return summary, fmt.Errorf("failed to commit rebuilt day for interface `%s` day `%d`: %w", iface, dayTimestamp, err)
				}
				summary.DaysRebuilt++
				summary.ConflictsResolvedByDestination += conflictsDst
				summary.ConflictsResolvedBySource += conflictsSrc
			default:
				return summary, fmt.Errorf("unsupported merge action for interface `%s` day `%d`", iface, dayTimestamp)
			}
		}
	}

	return summary, nil
}

func listSourceInterfaces(sourcePath string) ([]string, error) {
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return nil, err
	}

	ifaces := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ifaces = append(ifaces, entry.Name())
	}
	sort.Strings(ifaces)

	return ifaces, nil
}

func selectInterfaces(available, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return available, nil
	}

	availableSet := make(map[string]struct{}, len(available))
	for _, iface := range available {
		availableSet[iface] = struct{}{}
	}

	selectedSet := make(map[string]struct{}, len(requested))
	selected := make([]string, 0, len(requested))
	for _, iface := range requested {
		iface = strings.TrimSpace(iface)
		if iface == "" {
			continue
		}
		if _, found := availableSet[iface]; !found {
			return nil, fmt.Errorf("requested interface `%s` not found in source", iface)
		}
		if _, already := selectedSet[iface]; already {
			continue
		}
		selectedSet[iface] = struct{}{}
		selected = append(selected, iface)
	}

	sort.Strings(selected)
	return selected, nil
}

func listInterfaceDays(ifacePath string) (map[int64]dayDescriptor, error) {
	result := make(map[int64]dayDescriptor)

	entries, err := os.ReadDir(ifacePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return result, nil
		}
		return nil, err
	}

	for _, yearEntry := range entries {
		if !yearEntry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(yearEntry.Name()); err != nil {
			continue
		}

		yearPath := filepath.Join(ifacePath, yearEntry.Name())
		monthEntries, err := os.ReadDir(yearPath)
		if err != nil {
			return nil, err
		}

		for _, monthEntry := range monthEntries {
			if !monthEntry.IsDir() {
				continue
			}
			if _, err := strconv.Atoi(monthEntry.Name()); err != nil {
				continue
			}

			monthPath := filepath.Join(yearPath, monthEntry.Name())
			dayEntries, err := os.ReadDir(monthPath)
			if err != nil {
				return nil, err
			}

			for _, dayEntry := range dayEntries {
				if !dayEntry.IsDir() {
					continue
				}

				dayTimestamp, suffix, err := gpfile.ExtractTimestampMetadataSuffix(dayEntry.Name())
				if err != nil {
					return nil, fmt.Errorf("failed to parse day directory `%s`: %w", filepath.Join(monthPath, dayEntry.Name()), err)
				}

				if existing, exists := result[dayTimestamp]; exists {
					return nil, fmt.Errorf("duplicate day timestamp `%d` for interface path `%s`: `%s` and `%s`", dayTimestamp, ifacePath, existing.Path, filepath.Join(monthPath, dayEntry.Name()))
				}

				result[dayTimestamp] = dayDescriptor{
					Timestamp: dayTimestamp,
					Suffix:    suffix,
					DirName:   dayEntry.Name(),
					Path:      filepath.Join(monthPath, dayEntry.Name()),
				}
			}
		}
	}

	return result, nil
}

func mapKeysSorted(m map[int64]dayDescriptor) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

func isDayComplete(ifacePath string, d dayDescriptor, tolerance time.Duration) (bool, error) {
	reader := gpfile.NewDirReader(ifacePath, d.Timestamp, d.Suffix)
	if err := reader.Open(); err != nil {
		return false, err
	}
	defer func() {
		_ = reader.Close()
	}()

	if reader.NBlocks() == 0 {
		return false, nil
	}

	first, last := reader.TimeRange()
	dayStart := gpfile.DirTimestamp(d.Timestamp)
	dayEnd := dayStart + gpfile.EpochDay - 1
	toleranceSeconds := int64(tolerance / time.Second)
	if toleranceSeconds < 0 {
		toleranceSeconds = 0
	}

	completeAtStart := first <= dayStart+toleranceSeconds
	completeAtEnd := last >= dayEnd-toleranceSeconds

	return completeAtStart && completeAtEnd, nil
}

func planDayMerge(srcDay dayDescriptor, hasDst bool, dstDay dayDescriptor, overwrite bool) dayPlan {
	plan := dayPlan{
		SourceDay:  srcDay,
		HasDestDay: hasDst,
		DestDay:    dstDay,
	}

	if !hasDst {
		if srcDay.Complete {
			plan.Action = mergeDayActionCopy
			return plan
		}

		plan.Action = mergeDayActionRebuild
		plan.UseSource = true
		return plan
	}

	if srcDay.Complete && dstDay.Complete {
		if overwrite {
			plan.Action = mergeDayActionCopy
			return plan
		}
		plan.Action = mergeDayActionSkip
		return plan
	}

	if overwrite && srcDay.Complete {
		plan.Action = mergeDayActionCopy
		return plan
	}

	plan.Action = mergeDayActionRebuild
	plan.UseSource = true
	plan.UseDest = true
	return plan
}

func stageCopyDay(stageRoot, iface string, srcDay dayDescriptor) (string, error) {
	stageMonthPath := monthPathForTimestamp(filepath.Join(stageRoot, iface), srcDay.Timestamp)
	stagedPath := filepath.Join(stageMonthPath, srcDay.DirName)

	if err := copyDir(srcDay.Path, stagedPath); err != nil {
		return "", err
	}

	return stagedPath, nil
}

func rebuildDayToStage(ctx context.Context, stageRoot, iface, sourceIfacePath, destinationIfacePath string, dayTimestamp int64, plan dayPlan, overwrite bool) (stagedDayPath string, conflictsByDest int, conflictsBySource int, err error) {
	stageIfacePath := filepath.Join(stageRoot, iface)

	sourceSnapshots := make(map[int64]blockSnapshot)
	if plan.UseSource {
		sourceSnapshots, err = readDaySnapshots(sourceIfacePath, plan.SourceDay)
		if err != nil {
			return "", 0, 0, err
		}
	}

	destinationSnapshots := make(map[int64]blockSnapshot)
	if plan.UseDest {
		destinationSnapshots, err = readDaySnapshots(destinationIfacePath, plan.DestDay)
		if err != nil {
			return "", 0, 0, err
		}
	}

	mergedSnapshots, conflictsByDest, conflictsBySource := mergeSnapshots(sourceSnapshots, destinationSnapshots, plan.UseSource, plan.UseDest, overwrite)

	writer := gpfile.NewDirWriter(stageIfacePath, dayTimestamp, gpfile.WithPermissions(DefaultPermissions))
	if err := writer.Open(); err != nil {
		return "", 0, 0, err
	}
	writerClosed := false
	defer func() {
		if writerClosed {
			return
		}
		if closeErr := writer.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	for _, snapshot := range mergedSnapshots {
		select {
		case <-ctx.Done():
			return "", 0, 0, ctx.Err()
		default:
		}

		if err := writer.WriteBlocks(snapshot.Timestamp, snapshot.Traffic, snapshot.Counts, snapshot.Data); err != nil {
			return "", 0, 0, err
		}
	}

	if err := writer.Close(); err != nil {
		return "", 0, 0, err
	}
	writerClosed = true

	stagedDayPath, err = locateDayDirectory(stageIfacePath, dayTimestamp)
	if err != nil {
		return "", 0, 0, err
	}

	return stagedDayPath, conflictsByDest, conflictsBySource, nil
}

func mergeSnapshots(sourceSnapshots, destinationSnapshots map[int64]blockSnapshot, useSource, useDest, overwrite bool) (merged []blockSnapshot, conflictsByDest int, conflictsBySource int) {
	timestampsSet := make(map[int64]struct{}, len(sourceSnapshots)+len(destinationSnapshots))
	for ts := range sourceSnapshots {
		timestampsSet[ts] = struct{}{}
	}
	for ts := range destinationSnapshots {
		timestampsSet[ts] = struct{}{}
	}

	timestamps := make([]int64, 0, len(timestampsSet))
	for ts := range timestampsSet {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})

	merged = make([]blockSnapshot, 0, len(timestamps))
	for _, ts := range timestamps {
		srcSnapshot, hasSource := sourceSnapshots[ts]
		dstSnapshot, hasDestination := destinationSnapshots[ts]

		switch {
		case hasSource && hasDestination:
			if overwrite {
				merged = append(merged, srcSnapshot)
				conflictsBySource++
			} else {
				merged = append(merged, dstSnapshot)
				conflictsByDest++
			}
		case hasSource && useSource:
			merged = append(merged, srcSnapshot)
		case hasDestination && useDest:
			merged = append(merged, dstSnapshot)
		}
	}

	return merged, conflictsByDest, conflictsBySource
}

func readDaySnapshots(ifacePath string, day dayDescriptor) (map[int64]blockSnapshot, error) {
	reader := gpfile.NewDirReader(ifacePath, day.Timestamp, day.Suffix)
	if err := reader.Open(); err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	snapshots := make(map[int64]blockSnapshot, reader.NBlocks())
	for blockIndex, block := range reader.BlockMetadata[0].Blocks() {
		var data [types.ColIdxCount][]byte
		for i := range int(types.ColIdxCount) {
			columnIndex := types.ColumnIndex(i)
			blockData, err := reader.ReadBlockAtIndex(columnIndex, blockIndex)
			if err != nil {
				return nil, err
			}
			data[i] = append([]byte(nil), blockData...)
		}

		counters, err := decodeCounters(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode counters for day `%s` block `%d`: %w", day.Path, block.Timestamp, err)
		}

		snapshots[block.Timestamp] = blockSnapshot{
			Timestamp: block.Timestamp,
			Traffic:   reader.BlockTraffic[blockIndex],
			Counts:    counters,
			Data:      data,
		}
	}

	return snapshots, nil
}

func decodeCounters(data [types.ColIdxCount][]byte) (types.Counters, error) {
	bytesRcvdValues := bitpack.UnpackInto(data[types.BytesRcvdColIdx], nil)
	bytesSentValues := bitpack.UnpackInto(data[types.BytesSentColIdx], nil)
	pktsRcvdValues := bitpack.UnpackInto(data[types.PacketsRcvdColIdx], nil)
	pktsSentValues := bitpack.UnpackInto(data[types.PacketsSentColIdx], nil)

	n := len(bytesRcvdValues)
	if len(bytesSentValues) != n || len(pktsRcvdValues) != n || len(pktsSentValues) != n {
		return types.Counters{}, errors.New("counter columns differ in length")
	}

	var counters types.Counters
	for i := 0; i < n; i++ {
		counters.BytesRcvd += bytesRcvdValues[i]
		counters.BytesSent += bytesSentValues[i]
		counters.PacketsRcvd += pktsRcvdValues[i]
		counters.PacketsSent += pktsSentValues[i]
	}

	return counters, nil
}

func commitStagedDay(stagedDayPath, destinationIfacePath string, dayTimestamp int64, existing *dayDescriptor) (err error) {
	destinationMonthPath := monthPathForTimestamp(destinationIfacePath, dayTimestamp)
	if err := os.MkdirAll(destinationMonthPath, 0o755); err != nil {
		return err
	}

	destinationDayPath := filepath.Join(destinationMonthPath, filepath.Base(stagedDayPath))

	var backupPath string
	if existing != nil {
		backupPath = fmt.Sprintf("%s.gpdb-merge-backup-%d", existing.Path, time.Now().UnixNano())
		if err := os.Rename(existing.Path, backupPath); err != nil {
			return err
		}

		defer func() {
			if err == nil {
				if backupPath != "" {
					_ = os.RemoveAll(backupPath)
				}
				return
			}
			if backupPath != "" {
				_ = os.Rename(backupPath, existing.Path)
			}
		}()
	}

	if err := os.Rename(stagedDayPath, destinationDayPath); err != nil {
		return err
	}

	return nil
}

func locateDayDirectory(ifacePath string, dayTimestamp int64) (string, error) {
	monthPath := monthPathForTimestamp(ifacePath, dayTimestamp)
	entries, err := os.ReadDir(monthPath)
	if err != nil {
		return "", err
	}

	prefix := strconv.FormatInt(dayTimestamp, 10)
	matches := make([]string, 0, 1)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), prefix) {
			matches = append(matches, filepath.Join(monthPath, entry.Name()))
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("no staged day directory found for timestamp `%d` in `%s`", dayTimestamp, monthPath)
	default:
		return "", fmt.Errorf("multiple staged day directories found for timestamp `%d` in `%s`", dayTimestamp, monthPath)
	}
}

func monthPathForTimestamp(ifacePath string, dayTimestamp int64) string {
	dayUnix := time.Unix(gpfile.DirTimestamp(dayTimestamp), 0)
	return filepath.Join(ifacePath, strconv.Itoa(dayUnix.Year()), fmt.Sprintf("%02d", dayUnix.Month()))
}

func copyDir(srcDir, dstDir string) error {
	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source `%s` is not a directory", srcDir)
	}

	if err := os.MkdirAll(dstDir, srcInfo.Mode().Perm()); err != nil {
		return err
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(srcFile, dstFile string) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = src.Close()
	}()

	srcInfo, err := src.Stat()
	if err != nil {
		return err
	}

	dst, err := os.OpenFile(dstFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() {
		_ = dst.Close()
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	return nil
}
