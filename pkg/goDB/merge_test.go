package goDB

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/v4/pkg/types"
	"github.com/els0r/goProbe/v4/pkg/types/hashmap"
	"github.com/fako1024/gotools/bitpack"
	"github.com/stretchr/testify/require"
)

const testIface = "dummy0"

func TestMergeDatabasesCopiesCompleteSourceDay(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.January, 10, 0, 0, 0, 0, time.UTC).Unix()

	sourceBlocks := map[int64]uint64{
		dayStart + 300:   10,
		dayStart + 86399: 20,
	}
	writeDay(t, sourceDir, iface, dayStart, sourceBlocks)

	summary, err := MergeDatabases(context.Background(), MergeOptions{
		SourcePath:        sourceDir,
		DestinationPath:   destinationDir,
		CompleteTolerance: 300 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.InterfacesProcessed)
	require.Equal(t, 1, summary.DaysCopied)
	require.Equal(t, 0, summary.DaysRebuilt)
	require.Equal(t, 0, summary.DaysSkipped)

	days, err := listInterfaceDays(filepath.Join(destinationDir, iface))
	require.NoError(t, err)
	require.Len(t, days, 1)
	desc := firstDay(days)
	require.True(t, strings.Contains(desc.DirName, "_"), "expected copied complete day to carry metadata suffix")

	snapshots, err := readDaySnapshots(filepath.Join(destinationDir, iface), desc)
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	require.EqualValues(t, 10, snapshots[dayStart+300].Counts.BytesRcvd)
	require.EqualValues(t, 20, snapshots[dayStart+86399].Counts.BytesRcvd)
}

func TestMergeDatabasesRebuildUsesDestinationByDefault(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC).Unix()

	ts1 := dayStart + 300
	ts2 := dayStart + 600
	ts3 := dayStart + 900

	writeDay(t, sourceDir, iface, dayStart, map[int64]uint64{
		ts1: 10,
		ts2: 111,
	})
	writeDay(t, destinationDir, iface, dayStart, map[int64]uint64{
		ts2: 222,
		ts3: 333,
	})

	summary, err := MergeDatabases(context.Background(), MergeOptions{
		SourcePath:        sourceDir,
		DestinationPath:   destinationDir,
		CompleteTolerance: 300 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.DaysRebuilt)
	require.Equal(t, 0, summary.DaysCopied)
	require.Equal(t, 1, summary.ConflictsResolvedByDestination)
	require.Equal(t, 0, summary.ConflictsResolvedBySource)

	days, err := listInterfaceDays(filepath.Join(destinationDir, iface))
	require.NoError(t, err)
	require.Len(t, days, 1)
	desc := firstDay(days)

	snapshots, err := readDaySnapshots(filepath.Join(destinationDir, iface), desc)
	require.NoError(t, err)
	require.Len(t, snapshots, 3)
	require.EqualValues(t, 10, snapshots[ts1].Counts.BytesRcvd)
	require.EqualValues(t, 222, snapshots[ts2].Counts.BytesRcvd)
	require.EqualValues(t, 333, snapshots[ts3].Counts.BytesRcvd)
}

func TestMergeDatabasesRebuildUsesSourceWhenOverwriteEnabled(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC).Unix()

	ts1 := dayStart + 300
	ts2 := dayStart + 600
	ts3 := dayStart + 900

	writeDay(t, sourceDir, iface, dayStart, map[int64]uint64{
		ts1: 10,
		ts2: 111,
	})
	writeDay(t, destinationDir, iface, dayStart, map[int64]uint64{
		ts2: 222,
		ts3: 333,
	})

	summary, err := MergeDatabases(context.Background(), MergeOptions{
		SourcePath:        sourceDir,
		DestinationPath:   destinationDir,
		CompleteTolerance: 300 * time.Second,
		Overwrite:         true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.DaysRebuilt)
	require.Equal(t, 0, summary.DaysCopied)
	require.Equal(t, 0, summary.ConflictsResolvedByDestination)
	require.Equal(t, 1, summary.ConflictsResolvedBySource)

	days, err := listInterfaceDays(filepath.Join(destinationDir, iface))
	require.NoError(t, err)
	require.Len(t, days, 1)
	desc := firstDay(days)

	snapshots, err := readDaySnapshots(filepath.Join(destinationDir, iface), desc)
	require.NoError(t, err)
	require.Len(t, snapshots, 3)
	require.EqualValues(t, 10, snapshots[ts1].Counts.BytesRcvd)
	require.EqualValues(t, 111, snapshots[ts2].Counts.BytesRcvd)
	require.EqualValues(t, 333, snapshots[ts3].Counts.BytesRcvd)
}

func TestMergeDatabasesDryRunDoesNotMutateDestination(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()

	writeDay(t, sourceDir, iface, dayStart, map[int64]uint64{
		dayStart + 300:   10,
		dayStart + 86399: 20,
	})

	summary, err := MergeDatabases(context.Background(), MergeOptions{
		SourcePath:        sourceDir,
		DestinationPath:   destinationDir,
		DryRun:            true,
		CompleteTolerance: 300 * time.Second,
	})
	require.NoError(t, err)
	require.True(t, summary.DryRun)
	require.Equal(t, 1, summary.DaysCopied)

	days, err := listInterfaceDays(filepath.Join(destinationDir, iface))
	require.NoError(t, err)
	require.Empty(t, days)
}

func TestMergeDatabasesAppliesInterfaceFilter(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	dayStart := time.Date(2024, time.May, 1, 0, 0, 0, 0, time.UTC).Unix()

	writeDay(t, sourceDir, testIface, dayStart, map[int64]uint64{dayStart + 300: 10, dayStart + 86399: 20})
	writeDay(t, sourceDir, "eth1", dayStart, map[int64]uint64{dayStart + 300: 30, dayStart + 86399: 40})

	summary, err := MergeDatabases(context.Background(), MergeOptions{
		SourcePath:        sourceDir,
		DestinationPath:   destinationDir,
		Interfaces:        []string{"eth1"},
		CompleteTolerance: 300 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.InterfacesProcessed)
	require.Equal(t, 1, summary.DaysCopied)

	eth0Days, err := listInterfaceDays(filepath.Join(destinationDir, testIface))
	require.NoError(t, err)
	require.Empty(t, eth0Days)

	eth1Days, err := listInterfaceDays(filepath.Join(destinationDir, "eth1"))
	require.NoError(t, err)
	require.Len(t, eth1Days, 1)
}

func TestMergeDatabasesSkipsWhenBothDaysAreCompleteAndNoOverwrite(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.June, 1, 0, 0, 0, 0, time.UTC).Unix()

	conflictTS := dayStart + 300
	writeDay(t, sourceDir, iface, dayStart, map[int64]uint64{conflictTS: 10, dayStart + 86399: 20})
	writeDay(t, destinationDir, iface, dayStart, map[int64]uint64{conflictTS: 99, dayStart + 86399: 88})

	summary, err := MergeDatabases(context.Background(), MergeOptions{
		SourcePath:        sourceDir,
		DestinationPath:   destinationDir,
		CompleteTolerance: 300 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.DaysSkipped)
	require.Equal(t, 0, summary.DaysCopied)
	require.Equal(t, 0, summary.DaysRebuilt)

	desc := mustSingleDayDescriptor(t, filepath.Join(destinationDir, iface))
	snapshots, err := readDaySnapshots(filepath.Join(destinationDir, iface), desc)
	require.NoError(t, err)
	require.EqualValues(t, 99, snapshots[conflictTS].Counts.BytesRcvd)
}

func TestMergeDatabasesCopiesWhenBothDaysAreCompleteAndOverwrite(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.July, 1, 0, 0, 0, 0, time.UTC).Unix()

	conflictTS := dayStart + 300
	writeDay(t, sourceDir, iface, dayStart, map[int64]uint64{conflictTS: 10, dayStart + 86399: 20})
	writeDay(t, destinationDir, iface, dayStart, map[int64]uint64{conflictTS: 99, dayStart + 86399: 88})

	summary, err := MergeDatabases(context.Background(), MergeOptions{
		SourcePath:        sourceDir,
		DestinationPath:   destinationDir,
		Overwrite:         true,
		CompleteTolerance: 300 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.DaysCopied)
	require.Equal(t, 0, summary.DaysRebuilt)

	desc := mustSingleDayDescriptor(t, filepath.Join(destinationDir, iface))
	snapshots, err := readDaySnapshots(filepath.Join(destinationDir, iface), desc)
	require.NoError(t, err)
	require.EqualValues(t, 10, snapshots[conflictTS].Counts.BytesRcvd)
}

func TestMergeDatabasesRebuildsSourceOnlyPartialDay(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.August, 1, 0, 0, 0, 0, time.UTC).Unix()

	ts := dayStart + 300
	writeDay(t, sourceDir, iface, dayStart, map[int64]uint64{ts: 55})

	summary, err := MergeDatabases(context.Background(), MergeOptions{
		SourcePath:        sourceDir,
		DestinationPath:   destinationDir,
		CompleteTolerance: 300 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.DaysRebuilt)
	require.Equal(t, 0, summary.DaysCopied)

	desc := mustSingleDayDescriptor(t, filepath.Join(destinationDir, iface))
	snapshots, err := readDaySnapshots(filepath.Join(destinationDir, iface), desc)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.EqualValues(t, 55, snapshots[ts].Counts.BytesRcvd)
}

func TestMergeDatabasesContextCanceled(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.September, 1, 0, 0, 0, 0, time.UTC).Unix()

	writeDay(t, sourceDir, iface, dayStart, map[int64]uint64{dayStart + 300: 10, dayStart + 86399: 20})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := MergeDatabases(ctx, MergeOptions{SourcePath: sourceDir, DestinationPath: destinationDir})
	require.ErrorIs(t, err, context.Canceled)
}

func TestListSourceInterfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "z_if"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a_if"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "ignore.txt"), []byte("x"), 0o644))

	ifaces, err := listSourceInterfaces(root)
	require.NoError(t, err)
	require.Equal(t, []string{"a_if", "z_if"}, ifaces)
}

func TestSelectInterfaces(t *testing.T) {
	t.Parallel()

	available := []string{testIface, "eth1", "eth2"}

	selected, err := selectInterfaces(available, nil)
	require.NoError(t, err)
	require.Equal(t, available, selected)

	selected, err = selectInterfaces(available, []string{" eth1 ", testIface, "eth1"})
	require.NoError(t, err)
	require.Equal(t, []string{testIface, "eth1"}, selected)

	_, err = selectInterfaces(available, []string{"eth3"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requested interface")
}

func TestListInterfaceDays(t *testing.T) {
	t.Parallel()

	ifacePath := filepath.Join(t.TempDir(), testIface)
	missing, err := listInterfaceDays(ifacePath)
	require.NoError(t, err)
	require.Empty(t, missing)

	day1 := int64(1704067200)
	day2 := int64(1704153600)

	monthPath := monthPathForTimestamp(ifacePath, day1)
	require.NoError(t, os.MkdirAll(filepath.Join(monthPath, strconvFormatInt(day1)+"_meta"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(monthPath, strconvFormatInt(day2)), 0o755))

	days, err := listInterfaceDays(ifacePath)
	require.NoError(t, err)
	require.Len(t, days, 2)
	require.Equal(t, "meta", days[day1].Suffix)
	require.Equal(t, "", days[day2].Suffix)
}

func TestListInterfaceDaysDuplicateAndInvalid(t *testing.T) {
	t.Parallel()

	ifacePath := filepath.Join(t.TempDir(), testIface)
	day := int64(1704067200)
	monthPath := monthPathForTimestamp(ifacePath, day)

	require.NoError(t, os.MkdirAll(filepath.Join(monthPath, strconvFormatInt(day)), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(monthPath, strconvFormatInt(day)+"_suffix"), 0o755))

	_, err := listInterfaceDays(ifacePath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate day timestamp")

	ifacePathInvalid := filepath.Join(t.TempDir(), "eth1")
	invalidMonthPath := monthPathForTimestamp(ifacePathInvalid, day)
	require.NoError(t, os.MkdirAll(filepath.Join(invalidMonthPath, "not-a-day"), 0o755))

	_, err = listInterfaceDays(ifacePathInvalid)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse day directory")
}

func TestMapKeysSorted(t *testing.T) {
	t.Parallel()

	m := map[int64]dayDescriptor{
		5: {},
		1: {},
		3: {},
	}

	require.Equal(t, []int64{1, 3, 5}, mapKeysSorted(m))
}

func TestIsDayComplete(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	iface := testIface
	ifacePath := filepath.Join(base, iface)
	dayStart := time.Date(2024, time.October, 1, 0, 0, 0, 0, time.UTC).Unix()

	writeDay(t, base, iface, dayStart, map[int64]uint64{dayStart + 300: 1, dayStart + 86399: 1})
	desc := mustSingleDayDescriptor(t, ifacePath)

	complete, err := isDayComplete(ifacePath, desc, 300*time.Second)
	require.NoError(t, err)
	require.True(t, complete)

	basePartial := t.TempDir()
	ifacePartial := testIface
	ifacePartialPath := filepath.Join(basePartial, ifacePartial)
	writeDay(t, basePartial, ifacePartial, dayStart, map[int64]uint64{dayStart + 300: 1})
	descPartial := mustSingleDayDescriptor(t, ifacePartialPath)

	complete, err = isDayComplete(ifacePartialPath, descPartial, 300*time.Second)
	require.NoError(t, err)
	require.False(t, complete)
}

func TestPlanDayMergeMatrix(t *testing.T) {
	t.Parallel()

	day := dayDescriptor{}

	tests := []struct {
		name      string
		src       dayDescriptor
		hasDst    bool
		dst       dayDescriptor
		overwrite bool
		action    mergeDayAction
		useSource bool
		useDest   bool
	}{
		{name: "source complete only", src: dayDescriptor{Complete: true}, action: mergeDayActionCopy},
		{name: "source partial only", src: dayDescriptor{Complete: false}, action: mergeDayActionRebuild, useSource: true},
		{name: "both complete keep dest", src: dayDescriptor{Complete: true}, hasDst: true, dst: dayDescriptor{Complete: true}, action: mergeDayActionSkip},
		{name: "both complete overwrite", src: dayDescriptor{Complete: true}, hasDst: true, dst: dayDescriptor{Complete: true}, overwrite: true, action: mergeDayActionCopy},
		{name: "source complete dst partial keep", src: dayDescriptor{Complete: true}, hasDst: true, dst: dayDescriptor{Complete: false}, action: mergeDayActionRebuild, useSource: true, useDest: true},
		{name: "source complete dst partial overwrite", src: dayDescriptor{Complete: true}, hasDst: true, dst: dayDescriptor{Complete: false}, overwrite: true, action: mergeDayActionCopy},
		{name: "source partial dst complete", src: dayDescriptor{Complete: false}, hasDst: true, dst: dayDescriptor{Complete: true}, action: mergeDayActionRebuild, useSource: true, useDest: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			plan := planDayMerge(tc.src, tc.hasDst, tc.dst, tc.overwrite)
			require.Equal(t, tc.action, plan.Action)
			require.Equal(t, tc.useSource, plan.UseSource)
			require.Equal(t, tc.useDest, plan.UseDest)
			require.Equal(t, tc.src, plan.SourceDay)
			if tc.hasDst {
				require.Equal(t, tc.dst, plan.DestDay)
			} else {
				require.Equal(t, day, plan.DestDay)
			}
		})
	}
}

func TestStageCopyDay(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC).Unix()
	writeDay(t, base, iface, dayStart, map[int64]uint64{dayStart + 300: 1, dayStart + 86399: 2})

	ifacePath := filepath.Join(base, iface)
	srcDay := mustSingleDayDescriptor(t, ifacePath)

	stageRoot := t.TempDir()
	stagedPath, err := stageCopyDay(stageRoot, iface, srcDay)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(stagedPath, ".blockmeta"))
	require.NoError(t, err)

	origMeta, err := os.ReadFile(filepath.Join(srcDay.Path, ".blockmeta"))
	require.NoError(t, err)
	stagedMeta, err := os.ReadFile(filepath.Join(stagedPath, ".blockmeta"))
	require.NoError(t, err)
	require.Equal(t, origMeta, stagedMeta)
}

func TestRebuildDayToStage(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	iface := testIface
	dayStart := time.Date(2024, time.December, 1, 0, 0, 0, 0, time.UTC).Unix()

	ts1 := dayStart + 300
	ts2 := dayStart + 600
	ts3 := dayStart + 900

	writeDay(t, sourceDir, iface, dayStart, map[int64]uint64{ts1: 10, ts2: 111})
	writeDay(t, destinationDir, iface, dayStart, map[int64]uint64{ts2: 222, ts3: 333})

	srcDay := mustSingleDayDescriptor(t, filepath.Join(sourceDir, iface))
	dstDay := mustSingleDayDescriptor(t, filepath.Join(destinationDir, iface))

	stageRoot := t.TempDir()
	stagedPath, conflictsDst, conflictsSrc, err := rebuildDayToStage(
		context.Background(),
		stageRoot,
		iface,
		filepath.Join(sourceDir, iface),
		filepath.Join(destinationDir, iface),
		dayStart,
		dayPlan{UseSource: true, UseDest: true, SourceDay: srcDay, DestDay: dstDay},
		false,
	)
	require.NoError(t, err)
	require.Equal(t, 1, conflictsDst)
	require.Equal(t, 0, conflictsSrc)

	stagedDesc := dayDescriptor{
		Timestamp: dayStart,
		Suffix:    mustExtractSuffix(t, filepath.Base(stagedPath)),
		DirName:   filepath.Base(stagedPath),
		Path:      stagedPath,
	}
	snapshots, err := readDaySnapshots(filepath.Join(stageRoot, iface), stagedDesc)
	require.NoError(t, err)
	require.EqualValues(t, 222, snapshots[ts2].Counts.BytesRcvd)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err = rebuildDayToStage(
		ctx,
		stageRoot,
		iface,
		filepath.Join(sourceDir, iface),
		filepath.Join(destinationDir, iface),
		dayStart,
		dayPlan{UseSource: true, UseDest: true, SourceDay: srcDay, DestDay: dstDay},
		false,
	)
	require.ErrorIs(t, err, context.Canceled)
}

func TestMergeSnapshots(t *testing.T) {
	t.Parallel()

	src := map[int64]blockSnapshot{1: {Timestamp: 1, Counts: types.Counters{BytesRcvd: 10}}, 2: {Timestamp: 2, Counts: types.Counters{BytesRcvd: 20}}}
	dst := map[int64]blockSnapshot{2: {Timestamp: 2, Counts: types.Counters{BytesRcvd: 200}}, 3: {Timestamp: 3, Counts: types.Counters{BytesRcvd: 300}}}

	merged, conflictsDst, conflictsSrc := mergeSnapshots(src, dst, false)
	require.Len(t, merged, 3)
	require.Equal(t, 1, conflictsDst)
	require.Equal(t, 0, conflictsSrc)
	require.EqualValues(t, 200, merged[1].Counts.BytesRcvd)

	merged, conflictsDst, conflictsSrc = mergeSnapshots(src, dst, true)
	require.Len(t, merged, 3)
	require.Equal(t, 0, conflictsDst)
	require.Equal(t, 1, conflictsSrc)
	require.EqualValues(t, 20, merged[1].Counts.BytesRcvd)
}

func TestReadDaySnapshots(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	iface := testIface
	dayStart := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

	ts1 := dayStart + 300
	ts2 := dayStart + 600
	writeDay(t, base, iface, dayStart, map[int64]uint64{ts1: 10, ts2: 20})

	desc := mustSingleDayDescriptor(t, filepath.Join(base, iface))
	snapshots, err := readDaySnapshots(filepath.Join(base, iface), desc)
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	require.EqualValues(t, 10, snapshots[ts1].Counts.BytesRcvd)
	require.EqualValues(t, 20, snapshots[ts2].Counts.BytesRcvd)
}

func TestDecodeCounters(t *testing.T) {
	t.Parallel()

	flowMap := hashmap.NewAggFlowMap()
	flowMap.PrimaryMap.Set(types.NewV4KeyStatic(
		[4]byte{10, 0, 0, 1},
		[4]byte{10, 0, 0, 2},
		[]byte{0, 80},
		6,
	), types.Counters{BytesRcvd: 5, BytesSent: 7, PacketsRcvd: 11, PacketsSent: 13})

	data, _ := dbData(flowMap)
	counters, err := decodeCounters(data)
	require.NoError(t, err)
	require.EqualValues(t, 5, counters.BytesRcvd)
	require.EqualValues(t, 7, counters.BytesSent)
	require.EqualValues(t, 11, counters.PacketsRcvd)
	require.EqualValues(t, 13, counters.PacketsSent)

	data[types.BytesSentColIdx] = bitpack.Pack([]uint64{1, 2})
	_, err = decodeCounters(data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "counter columns differ")
}

func TestCommitStagedDay(t *testing.T) {
	t.Parallel()

	destinationIfacePath := filepath.Join(t.TempDir(), testIface)
	dayStart := time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC).Unix()
	monthPath := monthPathForTimestamp(destinationIfacePath, dayStart)
	require.NoError(t, os.MkdirAll(monthPath, 0o755))

	stagedRoot := t.TempDir()
	stagedPath := filepath.Join(stagedRoot, strconvFormatInt(dayStart)+"_new")
	require.NoError(t, os.MkdirAll(stagedPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stagedPath, "marker"), []byte("new"), 0o644))

	require.NoError(t, commitStagedDay(stagedPath, destinationIfacePath, dayStart, nil))

	newDestPath := filepath.Join(monthPath, filepath.Base(stagedPath))
	_, err := os.Stat(newDestPath)
	require.NoError(t, err)

	stagedPathExisting := filepath.Join(t.TempDir(), strconvFormatInt(dayStart)+"_replacement")
	require.NoError(t, os.MkdirAll(stagedPathExisting, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stagedPathExisting, "marker"), []byte("replacement"), 0o644))

	existingPath := filepath.Join(monthPath, strconvFormatInt(dayStart)+"_old")
	require.NoError(t, os.MkdirAll(existingPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(existingPath, "marker"), []byte("old"), 0o644))

	existing := dayDescriptor{Path: existingPath}
	require.NoError(t, commitStagedDay(stagedPathExisting, destinationIfacePath, dayStart, &existing))

	_, err = os.Stat(existingPath)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))

	replacedPath := filepath.Join(monthPath, filepath.Base(stagedPathExisting))
	data, err := os.ReadFile(filepath.Join(replacedPath, "marker"))
	require.NoError(t, err)
	require.Equal(t, "replacement", string(data))
}

func TestCommitStagedDayRollbackOnFailure(t *testing.T) {
	t.Parallel()

	destinationIfacePath := filepath.Join(t.TempDir(), testIface)
	dayStart := time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC).Unix()
	monthPath := monthPathForTimestamp(destinationIfacePath, dayStart)
	require.NoError(t, os.MkdirAll(monthPath, 0o755))

	existingPath := filepath.Join(monthPath, strconvFormatInt(dayStart)+"_old")
	require.NoError(t, os.MkdirAll(existingPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(existingPath, "marker"), []byte("old"), 0o644))

	stagedPath := filepath.Join(t.TempDir(), strconvFormatInt(dayStart)+"_new")
	require.NoError(t, os.MkdirAll(stagedPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stagedPath, "marker"), []byte("new"), 0o644))

	blockingPath := filepath.Join(monthPath, filepath.Base(stagedPath))
	require.NoError(t, os.MkdirAll(filepath.Join(blockingPath, "non-empty"), 0o755))

	existing := dayDescriptor{Path: existingPath}
	err := commitStagedDay(stagedPath, destinationIfacePath, dayStart, &existing)
	require.Error(t, err)

	data, readErr := os.ReadFile(filepath.Join(existingPath, "marker"))
	require.NoError(t, readErr)
	require.Equal(t, "old", string(data))
}

func TestLocateDayDirectory(t *testing.T) {
	t.Parallel()

	ifacePath := filepath.Join(t.TempDir(), testIface)
	dayStart := time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()
	monthPath := monthPathForTimestamp(ifacePath, dayStart)
	require.NoError(t, os.MkdirAll(monthPath, 0o755))

	_, err := locateDayDirectory(ifacePath, dayStart)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no staged day directory")

	one := filepath.Join(monthPath, strconvFormatInt(dayStart)+"_a")
	require.NoError(t, os.MkdirAll(one, 0o755))

	match, err := locateDayDirectory(ifacePath, dayStart)
	require.NoError(t, err)
	require.Equal(t, one, match)

	two := filepath.Join(monthPath, strconvFormatInt(dayStart)+"_b")
	require.NoError(t, os.MkdirAll(two, 0o755))

	_, err = locateDayDirectory(ifacePath, dayStart)
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple staged day directories")
}

func TestMonthPathForTimestamp(t *testing.T) {
	t.Parallel()

	ifacePath := "/tmp/example"
	dayStart := time.Date(2025, time.May, 12, 0, 0, 0, 0, time.UTC).Unix()

	path := monthPathForTimestamp(ifacePath, dayStart)
	require.Equal(t, filepath.Join(ifacePath, "2025", "05"), path)
}

func TestCopyFileAndCopyDir(t *testing.T) {
	t.Parallel()

	srcFile := filepath.Join(t.TempDir(), "src.txt")
	dstFile := filepath.Join(t.TempDir(), "dst.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("hello"), 0o644))
	require.NoError(t, copyFile(srcFile, dstFile))

	dstData, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	require.Equal(t, "hello", string(dstData))

	srcDirRoot := t.TempDir()
	srcDir := filepath.Join(srcDirRoot, "src")
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "nested", "child.txt"), []byte("child"), 0o644))

	dstDir := filepath.Join(t.TempDir(), "dst")
	require.NoError(t, copyDir(srcDir, dstDir))

	rootData, err := os.ReadFile(filepath.Join(dstDir, "root.txt"))
	require.NoError(t, err)
	require.Equal(t, "root", string(rootData))

	childData, err := os.ReadFile(filepath.Join(dstDir, "nested", "child.txt"))
	require.NoError(t, err)
	require.Equal(t, "child", string(childData))

	fileInsteadOfDir := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(fileInsteadOfDir, []byte("x"), 0o644))
	err = copyDir(fileInsteadOfDir, filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not a directory")
}

func writeDay(t *testing.T, dbPath, iface string, dayTimestamp int64, bytesByTimestamp map[int64]uint64) {
	t.Helper()

	writer := gpfile.NewDirWriter(filepath.Join(dbPath, iface), dayTimestamp)
	require.NoError(t, writer.Open())

	timestamps := make([]int64, 0, len(bytesByTimestamp))
	for ts := range bytesByTimestamp {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})

	for _, ts := range timestamps {
		writeBlock(t, writer, ts, bytesByTimestamp[ts])
	}

	require.NoError(t, writer.Close())
}

func writeBlock(t *testing.T, writer *gpfile.GPDir, timestamp int64, bytesRcvd uint64) {
	t.Helper()

	flowMap := hashmap.NewAggFlowMap()
	flowMap.PrimaryMap.Set(types.NewV4KeyStatic(
		[4]byte{10, 0, 0, 1},
		[4]byte{10, 0, 0, 2},
		[]byte{0, 80},
		6,
	), types.Counters{
		BytesRcvd:   bytesRcvd,
		BytesSent:   1,
		PacketsRcvd: 1,
		PacketsSent: 1,
	})

	data, update := dbData(flowMap)
	require.NoError(t, writer.WriteBlocks(timestamp, gpfile.TrafficMetadata{
		NumV4Entries: update.Traffic.NumV4Entries,
		NumV6Entries: update.Traffic.NumV6Entries,
		NumDrops:     0,
	}, update.Counts, data))
}

func firstDay(days map[int64]dayDescriptor) dayDescriptor {
	for _, day := range days {
		return day
	}
	return dayDescriptor{}
}

func mustSingleDayDescriptor(t *testing.T, ifacePath string) dayDescriptor {
	t.Helper()

	days, err := listInterfaceDays(ifacePath)
	require.NoError(t, err)
	require.Len(t, days, 1)

	return firstDay(days)
}

func mustExtractSuffix(t *testing.T, dayDirName string) string {
	t.Helper()

	_, suffix, err := gpfile.ExtractTimestampMetadataSuffix(dayDirName)
	require.NoError(t, err)
	return suffix
}

func strconvFormatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
