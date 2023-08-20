/////////////////////////////////////////////////////////////////////////////////
//
// DBWorkManager.go
//
// Helper functions that decide which files in the go database have to be written
// to or read from
//
// Written by Lennart Elsen lel@open.ch, July 2014
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder"
	"github.com/els0r/goProbe/pkg/goDB/encoder/bitpack"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
)

const (

	// DBWriteInterval defines the periodic write out interval of goProbe
	DBWriteInterval int64 = 300

	// WorkBulkSize denotes the per-worker bulk size (number of GPDirs processed before
	// transmitting the resulting map to for further reduction / aggregtion
	WorkBulkSize = 32

	// defaultEncoderType denotes the default encoder / compressor
	defaultEncoderType = encoders.EncoderTypeLZ4
)

// DBWorkload stores all relevant parameters to load a block and execute a query on it
type DBWorkload struct {
	workDirs []*gpfile.GPDir
}

// DBWorkManager schedules parallel processing of blocks relevant for a query
type DBWorkManager struct {
	query              *Query
	dbIfaceDir         string // path to interface directory in DB, e.g. /path/to/db/eth0
	iface              string
	workloadChan       chan DBWorkload
	numProcessingUnits int

	tFirstCovered, tLastCovered int64

	nWorkloads          uint64
	nWorkloadsProcessed uint64
	memPool             gpfile.MemPoolGCable
}

// NewDBWorkManager sets up a new work manager for executing queries
func NewDBWorkManager(query *Query, dbpath string, iface string, numProcessingUnits int) (*DBWorkManager, error) {

	// Explicitly handle invalid number of processing units (to avoid deadlock)
	if numProcessingUnits <= 0 {
		return nil, fmt.Errorf("invalid number of processing units: %d", numProcessingUnits)
	}

	return &DBWorkManager{
		query:              query,
		dbIfaceDir:         filepath.Join(dbpath, iface),
		iface:              iface,
		workloadChan:       make(chan DBWorkload, numProcessingUnits*64), // 64 is relatively arbitrary (but we're just sending quite basic objects)
		numProcessingUnits: numProcessingUnits,
	}, nil
}

// GetNumWorkers returns the number of workloads available to the outside world for loop bounds etc.
func (w *DBWorkManager) GetNumWorkers() uint64 {
	return w.nWorkloads
}

// GetCoveredTimeInterval can be used to determine the time span actually covered by the query
func (w *DBWorkManager) GetCoveredTimeInterval() (time.Time, time.Time) {
	return time.Unix(w.tFirstCovered-DBWriteInterval, 0), time.Unix(w.tLastCovered, 0)
}

// CreateWorkerJobs sets up all workloads for query execution
func (w *DBWorkManager) CreateWorkerJobs(tfirst int64, tlast int64) (nonempty bool, err error) {
	// Make sure the channel is closed at the end of this function no matter what to
	// ensure graceful termination of all workers
	defer close(w.workloadChan)

	// loop over directory list in order to create the timestamp pairs
	var (
		gpFileOptions []gpfile.Option
	)
	if !w.query.lowMem {
		w.memPool = gpfile.NewMemPool(w.numProcessingUnits * len(w.query.columnIndices))
		gpFileOptions = append(gpFileOptions, gpfile.WithReadAll(w.memPool))
	}

	// make sure to start with zero workloads as the number of assigned
	// workloads depends on how many directories have to be read
	var curDir *gpfile.GPDir
	workloadBulk := make([]*gpfile.GPDir, 0, WorkBulkSize)

	walkFunc := func(numDirs int, dayTimestamp int64) error {
		curDir = gpfile.NewDir(w.dbIfaceDir, dayTimestamp, gpfile.ModeRead, gpFileOptions...)

		// For the first and last item, check out the GPDir metadata for the actual first and
		// last block timestamp to cover (and adapt variables accordingly)
		// We will grab the timestamp from the first visited / valid directory that fulfils
		// the timestamp condition on directory level
		if numDirs == 0 {
			if err := curDir.Open(); err != nil {
				return fmt.Errorf("failed to open first GPDir %s to ascertain query block timing: %w", curDir.Path(), err)
			}
			dirFirst, _ := curDir.TimeRange()
			if tfirst < dirFirst {
				w.tFirstCovered = dirFirst
			}
			if err := curDir.Close(); err != nil {
				return fmt.Errorf("failed to close first GPDir %s after ascertaining query block timing: %w", curDir.Path(), err)
			}
		}

		// create new workload for the directory
		workloadBulk = append(workloadBulk, curDir)
		if len(workloadBulk) == WorkBulkSize {
			w.workloadChan <- DBWorkload{workDirs: workloadBulk}
			w.nWorkloads++
			workloadBulk = make([]*gpfile.GPDir, 0, WorkBulkSize)
		}
		return nil
	}
	numDirs, err := w.walkDB(tfirst, tlast, walkFunc)

	// Flush any remaining work
	if len(workloadBulk) > 0 {
		w.workloadChan <- DBWorkload{workDirs: workloadBulk}
		w.nWorkloads++
	}

	// For the first and last item, check out the GPDir metadata for the actual first and
	// last block timestamp to cover (and adapt variables accordingly)
	// This has to happen here because we cannot known when the last directory fulfilled the
	// timestamp condition above. If curDir is not nil then it points to the last visited / valid
	// directory
	if curDir != nil {
		if err := curDir.Open(); err != nil {
			return false, fmt.Errorf("failed to open last GPDir %s to ascertain query block timing: %w", curDir.Path(), err)
		}
		_, dirLast := curDir.TimeRange()
		if tlast > dirLast {
			w.tLastCovered = dirLast
		}
		if err := curDir.Close(); err != nil {
			return false, fmt.Errorf("failed to close last GPDir %s after ascertaining query block timing: %w", curDir.Path(), err)
		}
	}

	return 0 < numDirs, nil
}

func skipNonMatching(isDir bool, name string) bool {
	return !isDir
}

type dbWalkFunc func(numDirs int, dayTimestamp int64) error

func (w *DBWorkManager) walkDB(tfirst, tlast int64, fn dbWalkFunc) (numDirs int, err error) {
	// Get list of years in main directory (ordered by directory name, i.e. time)
	yearList, err := os.ReadDir(filepath.Clean(w.dbIfaceDir))
	if err != nil {
		return numDirs, err
	}
	w.tFirstCovered, w.tLastCovered = tfirst, tlast

	var unixFirst, unixLast = time.Unix(tfirst, 0), time.Unix(tlast+DBWriteInterval, 0)
	for _, year := range yearList {

		// Skip obvious non-matching entries
		if skipNonMatching(year.IsDir(), year.Name()) {
			continue
		}

		// Skip if outside of annual range
		yearTimestamp, err := strconv.Atoi(year.Name())
		if err != nil {
			return numDirs, fmt.Errorf("failed to parse year from directory `%s`: %w", year.Name(), err)
		}
		if yearTimestamp < unixFirst.Year() || yearTimestamp > unixLast.Year() {
			continue
		}

		// Get list of months in year directory (ordered by directory name, i.e. time)
		monthList, err := os.ReadDir(filepath.Clean(filepath.Join(w.dbIfaceDir, year.Name())))
		if err != nil {
			return numDirs, err
		}
		for _, month := range monthList {
			// Skip obvious non-matching entries
			if skipNonMatching(month.IsDir(), month.Name()) {
				continue
			}

			// Skip if outside of month range (only considering the "edge" years)
			monthTimestamp, err := strconv.Atoi(month.Name())
			if err != nil {
				return numDirs, fmt.Errorf("failed to parse month from directory `%s`: %w", year.Name(), err)
			}
			if (yearTimestamp == unixFirst.Year() && time.Month(monthTimestamp) < unixFirst.Month()) ||
				(yearTimestamp == unixLast.Year() && time.Month(monthTimestamp) > unixLast.Month()) {
				continue
			}

			// Get list of days in month directory (ordered by directory name, i.e. time)
			dirList, err := os.ReadDir(filepath.Clean(filepath.Join(w.dbIfaceDir, year.Name(), month.Name())))
			if err != nil {
				return numDirs, err
			}

			for _, file := range dirList {
				if skipNonMatching(file.IsDir(), file.Name()) {
					continue
				}
				dayTimestamp, err := strconv.ParseInt(file.Name(), 10, 64)
				if err != nil {
					return numDirs, fmt.Errorf("failed to parse epoch timestamp from directory `%s`: %w", file.Name(), err)
				}

				// check if the directory is within time frame of interest
				if tfirst < dayTimestamp+gpfile.EpochDay && dayTimestamp < tlast+DBWriteInterval {
					// actual processing upon a match
					err := fn(numDirs, dayTimestamp)
					if err != nil {
						return numDirs, err
					}
					numDirs++
				}
			}
		}
	}
	return numDirs, nil
}

func (w *DBWorkManager) ReadMetadata(tfirst int64, tlast int64) (*InterfaceMetadata, error) {
	aggMetadata := &InterfaceMetadata{Iface: w.iface}

	query := NewMetadataQuery()

	// loop over directory list in order to create the timestamp pairs
	var gpFileOptions []gpfile.Option
	if !query.lowMem {
		w.memPool = gpfile.NewMemPool(w.numProcessingUnits * len(query.columnIndices))
		gpFileOptions = append(gpFileOptions, gpfile.WithReadAll(w.memPool))
	}

	// make sure to start with zero workloads as the number of assigned
	// workloads depends on how many directories have to be read
	var curDir *gpfile.GPDir

	var currentTimestamp int64

	walkFunc := func(numDirs int, dayTimestamp int64) error {
		currentTimestamp = dayTimestamp
		curDir = gpfile.NewDir(w.dbIfaceDir, dayTimestamp, gpfile.ModeRead, gpFileOptions...)

		err := curDir.Open()
		if err != nil {
			return fmt.Errorf("failed to open first GPDir %s to ascertain query block timing: %w", curDir.Path(), err)
		}

		// do the metadata compuation based on the metadata
		aggMetadata.Stats = aggMetadata.Stats.Add(curDir.Stats)

		// compute the metadata for the first day. If a "first" time argument is given,
		// the partial day has to be computed
		if numDirs == 0 {
			dirFirst, _ := curDir.TimeRange()
			if tfirst >= dirFirst {
				// subtract all entries that are smaller than w.tFirstCovered because they were added in the day loop
				var (
					tFirstBlockInd = len(curDir.BlockMetadata[0].BlockList) - 1
					blocks         = curDir.BlockMetadata[0].BlocksBefore(tfirst)
				)

				// only assign the first covered time if there are blocks to subtract, e.g. if BlocksBefore(tfirst) != BlockList
				if len(blocks) < tFirstBlockInd {
					tFirstBlockInd = len(blocks)
					w.tFirstCovered = curDir.BlockMetadata[0].BlockList[tFirstBlockInd].Timestamp
				}

				aggMetadata, err = w.readMetadataAndEvaluate(curDir,
					blocks, 0,
					aggMetadata, func(metadata *InterfaceMetadata, stats gpfile.Stats) gpfile.Stats {
						return metadata.Stats.Sub(stats)
					},
				)
				if err != nil {
					return fmt.Errorf("failed to read block metadata: %w", err)
				}
			} else {
				w.tFirstCovered = dirFirst
			}
		}
		if err := curDir.Close(); err != nil {
			return fmt.Errorf("failed to close first GPDir %s after ascertaining query block timing: %w", curDir.Path(), err)
		}
		return nil
	}

	_, err := w.walkDB(tfirst, tlast, walkFunc)

	// compute the metadata for the last block. This will be partial if the last timestamp is smaller than the last
	// block captured for the day
	if curDir != nil {
		curDir = gpfile.NewDir(w.dbIfaceDir, currentTimestamp, gpfile.ModeRead, gpFileOptions...)

		if err := curDir.Open(); err != nil {
			return nil, fmt.Errorf("failed to open last GPDir %s to ascertain query block timing: %w", curDir.Path(), err)
		}
		_, dirLast := curDir.TimeRange()

		if tlast <= dirLast {
			// subtract all entries that are smaller than w.tLastCovered because they were added in the day loop
			var (
				blocks, offset = curDir.BlockMetadata[0].BlocksAfter(tlast)
				tLastBlockInd  = len(curDir.BlockMetadata[0].BlockList) - len(blocks) - 1
			)

			aggMetadata, err = w.readMetadataAndEvaluate(curDir,
				blocks, offset,
				aggMetadata, func(metadata *InterfaceMetadata, stats gpfile.Stats) gpfile.Stats {
					return metadata.Stats.Sub(stats)
				},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to read block metadata: %w", err)
			}
			w.tLastCovered = curDir.BlockMetadata[0].BlockList[tLastBlockInd].Timestamp
		} else {
			w.tLastCovered = dirLast
		}

		if err := curDir.Close(); err != nil {
			return nil, fmt.Errorf("failed to close last GPDir %s after ascertaining query block timing: %w", curDir.Path(), err)
		}
	}

	// assign time range of interface
	//
	// IMPORTANT: in the case of the first timestamp, the DB write interval is subtracted to
	// show that flows are taken into account _up to_ the timestamp of the effective write out.
	//
	// This will retain consistency with what is presented in the summary after a normal query
	aggMetadata.First, aggMetadata.Last = w.GetCoveredTimeInterval()

	return aggMetadata, nil
}

// NOTE: contrary to it's bigger sister readBlocksAndEvaluate, the function assumes that the workDir is already open.
// This is owed to the nature of its calling function
func (w *DBWorkManager) readMetadataAndEvaluate(workDir *gpfile.GPDir, blocks []storage.BlockAtTime, offset int, aggMetadata *InterfaceMetadata,
	statsOpFunc func(*InterfaceMetadata, gpfile.Stats) gpfile.Stats,
) (*InterfaceMetadata, error) {
	logger := logging.Logger().With("iface", w.iface, "day", workDir.Path())

	var (
		bytesRcvdValues, bytesSentValues, pktsRcvdValues, pktsSentValues []uint64
		err                                                              error
	)

	for b, block := range blocks {
		ind := b + offset

		var (
			colBlocks   [types.ColIdxCount][]byte
			blockBroken bool
			stats       = gpfile.Stats{}
		)

		// Read the blocks from their files
		for _, colIdx := range w.query.columnIndices {
			// Read the block from the file
			if colBlocks[colIdx], err = workDir.ReadBlockAtIndex(colIdx, ind); err != nil {
				blockBroken = true
				logger.With(
					"block", block.Timestamp,
					"column", types.ColumnFileNames[colIdx],
				).Warnf("Failed to read column: %s", err)
				break
			}
		}

		// Check whether all blocks have matching number of entries
		stats.Traffic.NumV4Entries = uint64(workDir.NumIPv4EntriesAtIndex(ind))
		stats.Traffic.NumV6Entries = uint64(workDir.NumIPv6EntriesAtIndex(ind))

		numEntries := bitpack.Len(colBlocks[types.BytesRcvdColIdx])
		for _, colIdx := range w.query.columnIndices {
			if colIdx.IsCounterCol() {
				if bitpack.Len(colBlocks[colIdx]) != numEntries {
					blockBroken = true
					logger.With(
						"block", ind,
						"column", types.ColumnFileNames[colIdx],
					).Warnf("Incorrect number of entries in column file. Expected %d, found %d", numEntries, bitpack.Len(colBlocks[colIdx]))
					break
				}
			}
		}

		// In case any error was observed during above sanity checks, skip this whole block
		if blockBroken {
			continue
		}

		bytesRcvdValues = bitpack.UnpackInto(colBlocks[types.BytesRcvdColIdx], bytesRcvdValues)
		bytesSentValues = bitpack.UnpackInto(colBlocks[types.BytesSentColIdx], bytesSentValues)
		pktsRcvdValues = bitpack.UnpackInto(colBlocks[types.PacketsRcvdColIdx], pktsRcvdValues)
		pktsSentValues = bitpack.UnpackInto(colBlocks[types.PacketsSentColIdx], pktsSentValues)

		for i := 0; i < numEntries; i++ {
			stats.Counts = stats.Counts.Add(types.Counters{
				BytesRcvd:   bytesRcvdValues[i],
				BytesSent:   bytesSentValues[i],
				PacketsRcvd: pktsRcvdValues[i],
				PacketsSent: pktsSentValues[i],
			})
		}

		// perform operation for stats gathered from blocks
		aggMetadata.Stats = statsOpFunc(aggMetadata, stats)

	}

	return aggMetadata, nil
}

// main query processing
func (w *DBWorkManager) grabAndProcessWorkload(ctx context.Context, wg *sync.WaitGroup, workloadChan <-chan DBWorkload, mapChan chan hashmap.AggFlowMapWithMetadata) {
	go func() {
		defer wg.Done()

		logger := logging.FromContext(ctx)

		enc, err := encoder.New(defaultEncoderType)
		if err != nil {
			logger.Error(err)
			mapChan <- hashmap.NilAggFlowMapWithMetadata
		}
		defer enc.Close()

		var workload DBWorkload
		for chanOpen := true; chanOpen; {
			select {
			case <-ctx.Done():
				// query was cancelled, exit
				logger.Infof("Query cancelled (workload %d / %d)...", atomic.LoadUint64(&w.nWorkloadsProcessed), w.nWorkloads)
				return
			case workload, chanOpen = <-workloadChan:
				if chanOpen {
					resultMap := hashmap.NewAggFlowMapWithMetadata()
					for _, workDir := range workload.workDirs {

						// if there is an error during one of the read jobs, throw a syslog message and terminate
						err := w.readBlocksAndEvaluate(workDir, enc, &resultMap)
						if err != nil {
							logger.Error(err)
							mapChan <- hashmap.NilAggFlowMapWithMetadata
							return
						}
					}

					// Workload is counted, but we only add it to the final result if we got any entries
					atomic.AddUint64(&w.nWorkloadsProcessed, 1)
					if resultMap.Len() > 0 {
						mapChan <- resultMap
					}
				}
			}
		}
	}()
}

// ExecuteWorkerReadJobs runs the query concurrently with multiple sprocessing units
func (w *DBWorkManager) ExecuteWorkerReadJobs(ctx context.Context, mapChan chan hashmap.AggFlowMapWithMetadata) {

	var wg = new(sync.WaitGroup)
	wg.Add(w.numProcessingUnits)
	for i := 0; i < w.numProcessingUnits; i++ {
		// start worker up
		w.grabAndProcessWorkload(ctx, wg, w.workloadChan, mapChan)
	}

	// check if all workers are done
	wg.Wait()
}

// Block evaluation and aggregation -----------------------------------------------------
// this is where the actual reading and aggregation magic happens
func (w *DBWorkManager) readBlocksAndEvaluate(workDir *gpfile.GPDir, enc encoder.Encoder, resultMap *hashmap.AggFlowMapWithMetadata) (err error) {
	logger := logging.Logger()

	var (
		v4Key, v4ComparisonValue                                         = types.NewEmptyV4Key().ExtendEmpty(), types.NewEmptyV4Key().ExtendEmpty()
		v6Key, v6ComparisonValue                                         = types.NewEmptyV6Key().ExtendEmpty(), types.NewEmptyV6Key().ExtendEmpty()
		bytesRcvdValues, bytesSentValues, pktsRcvdValues, pktsSentValues []uint64
	)

	// Open GPDir (reading metadata in the process)
	if err := workDir.Open(gpfile.WithEncoder(enc)); err != nil {
		return err
	}
	defer func() {
		if cerr := workDir.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// Set map metadata (and cross-check consistency for consecutive workloads)
	if resultMap.Interface == "" {
		resultMap.Interface = w.iface
	} else if resultMap.Interface != w.iface {
		return fmt.Errorf("discovered invalid workload for mismatching interfaces, want `%s`, have `%s`", resultMap.Interface, w.iface)
	}

	// Process the workload, looping over all blocks in this directory
	for b, block := range workDir.BlockMetadata[0].Blocks() {

		// If this block is outside of the rannge, skip it (only happens at the very first
		// and /or very last directory)
		if block.Timestamp < w.tFirstCovered || block.Timestamp > w.tLastCovered {
			continue
		}

		var (
			blocks      [types.ColIdxCount][]byte
			blockBroken bool
		)

		// Read the blocks from their files
		for _, colIdx := range w.query.columnIndices {

			// Read the block from the file
			if blocks[colIdx], err = workDir.ReadBlockAtIndex(colIdx, b); err != nil {
				blockBroken = true
				logger.With("day", workDir, "block", block.Timestamp, "column", types.ColumnFileNames[colIdx]).Warnf("Failed to read column: %s", err)
				break
			}
		}

		// Check whether all blocks have matching number of entries
		numV4Entries := int(workDir.NumIPv4EntriesAtIndex(b))
		numEntries := bitpack.Len(blocks[types.BytesRcvdColIdx])
		for _, colIdx := range w.query.columnIndices {
			l := len(blocks[colIdx])
			if colIdx.IsCounterCol() {
				if bitpack.Len(blocks[colIdx]) != numEntries {
					blockBroken = true
					logger.With("block", b, "column", types.ColumnFileNames[colIdx]).Warnf("Incorrect number of entries in column file. Expected %d, found %d", numEntries, bitpack.Len(blocks[colIdx]))
					break
				}
			} else {
				if types.ColumnSizeofs[colIdx] == types.IPSizeOf {
					if l != (numEntries-int(numV4Entries))*types.IPv6Width+int(numV4Entries)*types.IPv4Width {
						blockBroken = true
						logger.With("block", b, "column", types.ColumnFileNames[colIdx]).Warnf("Incorrect number of entries in variable block size file. Expected file length %d, have %d", (numEntries-int(numV4Entries))*types.IPv6Width+int(numV4Entries)*types.IPv4Width, l)
						break
					}
				} else {
					if l/types.ColumnSizeofs[colIdx] != numEntries {
						blockBroken = true
						logger.With("block", b, "column", types.ColumnFileNames[colIdx]).Warnf("Incorrect number of entries in column file. Expected %d, found %d", numEntries, l/types.ColumnSizeofs[colIdx])
						break
					}
					if l%types.ColumnSizeofs[colIdx] != 0 {
						blockBroken = true
						logger.With("block", b, "column", types.ColumnFileNames[colIdx]).Warn("Entry size does not evenly divide block size in column file")
						break
					}
				}
			}
		}

		// In case any error was observed during above sanity checks, skip this whole block
		if blockBroken {
			continue
		}

		// Initialize any (static) key extensions potentially present in the query
		if w.query.hasAttrTime {
			v4Key = types.NewEmptyV4Key().Extend(block.Timestamp)
			v6Key = types.NewEmptyV6Key().Extend(block.Timestamp)
			if w.query.Conditional == nil {
				v4ComparisonValue = types.NewEmptyV4Key().Extend(block.Timestamp)
				v6ComparisonValue = types.NewEmptyV6Key().Extend(block.Timestamp)
			}
		}

		bytesRcvdValues = bitpack.UnpackInto(blocks[types.BytesRcvdColIdx], bytesRcvdValues)
		bytesSentValues = bitpack.UnpackInto(blocks[types.BytesSentColIdx], bytesSentValues)
		pktsRcvdValues = bitpack.UnpackInto(blocks[types.PacketsRcvdColIdx], pktsRcvdValues)
		pktsSentValues = bitpack.UnpackInto(blocks[types.PacketsSentColIdx], pktsSentValues)

		sipBlocks := blocks[types.SipColIdx]
		dipBlocks := blocks[types.DipColIdx]
		dportBlocks := blocks[types.DportColIdx]
		protoBlocks := blocks[types.ProtoColIdx]

		// Determine start / end of block perusal - If the query is limited to either IPv4 or IPv6, adjust
		// accordingly to skip irrelevant data that wouldn't satisfy the condition anyway
		key, comparisonValue := v4Key, v4ComparisonValue
		startEntry, isIPv4, condIsIPv4 := 0, true, true
		if w.query.ipVersion == types.IPVersionV6 {
			startEntry = numV4Entries
		} else if w.query.ipVersion == types.IPVersionV4 {
			numEntries = numV4Entries
		}
		for i := startEntry; i < numEntries; i++ {

			// If / when reaching the v4/v6 mark, switch to the IPv6 key / submap
			if i == numV4Entries {

				// Skip switching to secondary map if IPs are not part of the query attributes
				if w.query.hasAttrSip || w.query.hasAttrDip {
					key = v6Key
					isIPv4 = false
				}

				// But always switch the comparison value to allow for proper filtering
				comparisonValue = v6ComparisonValue
				condIsIPv4 = false
			}

			// Populate key for current entry
			if w.query.hasAttrSip {
				if isIPv4 {
					key.PutSip(sipBlocks[i*4 : i*4+4])
				} else {
					key.PutSip(sipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
				}
			}
			if w.query.hasAttrDip {
				if isIPv4 {
					key.PutDipV4(dipBlocks[i*4 : i*4+4])
				} else {
					key.PutDipV6(dipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
				}
			}
			if w.query.hasAttrProto {
				key.PutProtoV(protoBlocks[i], isIPv4)
			}
			if w.query.hasAttrDport {
				key.PutDportV(dportBlocks[i*types.DportSizeof:i*types.DportSizeof+types.DportSizeof], isIPv4)
			}

			// Check whether conditional is satisfied for current entry
			var conditionalSatisfied = (w.query.Conditional == nil)
			if !conditionalSatisfied {

				// Populate comparison value for current entry
				if w.query.hasCondSip {
					if condIsIPv4 {
						comparisonValue.PutSip(sipBlocks[i*4 : i*4+4])
					} else {
						comparisonValue.PutSip(sipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
					}
				}
				if w.query.hasCondDip {
					if condIsIPv4 {
						comparisonValue.PutDipV4(dipBlocks[i*4 : i*4+4])
					} else {
						comparisonValue.PutDipV6(dipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
					}
				}
				if w.query.hasCondProto {
					comparisonValue.PutProtoV(protoBlocks[i], condIsIPv4)
				}
				if w.query.hasCondDport {
					comparisonValue.PutDportV(dportBlocks[i*types.DportSizeof:i*types.DportSizeof+types.DportSizeof], condIsIPv4)
				}

				conditionalSatisfied = w.query.Conditional.Evaluate(comparisonValue.Key())
			}

			if conditionalSatisfied {
				resultMap.SetOrUpdate(key,
					isIPv4,
					bytesRcvdValues[i],
					bytesSentValues[i],
					pktsRcvdValues[i],
					pktsSentValues[i],
				)
			}
		}
	}

	return nil
}

// Close releases all resources claimed by the DBWorkManager
func (w *DBWorkManager) Close() {
	if w.memPool != nil {
		w.memPool.Clear()
	}
}
