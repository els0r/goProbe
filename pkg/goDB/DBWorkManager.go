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
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder"
	"github.com/els0r/goProbe/pkg/goDB/encoder/bitpack"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
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
	query    *Query
	workDirs []*gpfile.GPDir
}

// DBWorkManager schedules parallel processing of blocks relevant for a query
type DBWorkManager struct {
	dbIfaceDir         string // path to interface directory in DB, e.g. /path/to/db/eth0
	iface              string
	workloadChan       chan DBWorkload
	numProcessingUnits int

	tFirstCovered, tLastCovered int64

	nWorkloads          int
	nWorkloadsProcessed int
	memPool             gpfile.MemPoolGCable
}

// NewDBWorkManager sets up a new work manager for executing queries
func NewDBWorkManager(dbpath string, iface string, numProcessingUnits int) (*DBWorkManager, error) {

	// Explicitly handle invalid number of processing units (to avoid deadlock)
	if numProcessingUnits <= 0 {
		return nil, fmt.Errorf("invalid number of processing units: %d", numProcessingUnits)
	}

	return &DBWorkManager{
		dbIfaceDir:         filepath.Join(dbpath, iface),
		iface:              iface,
		workloadChan:       make(chan DBWorkload, numProcessingUnits*64), // 64 is relatively arbitrary (but we're just sending quite basic objects)
		numProcessingUnits: numProcessingUnits,
	}, nil
}

// GetNumWorkers returns the number of workloads available to the outside world for loop bounds etc.
func (w *DBWorkManager) GetNumWorkers() int {
	return w.nWorkloads
}

// GetCoveredTimeInterval can be used to determine the time span actually covered by the query
func (w *DBWorkManager) GetCoveredTimeInterval() (time.Time, time.Time) {
	return time.Unix(w.tFirstCovered-DBWriteInterval, 0), time.Unix(w.tLastCovered, 0)
}

// CreateWorkerJobs sets up all workloads for query execution
func (w *DBWorkManager) CreateWorkerJobs(tfirst int64, tlast int64, query *Query) (nonempty bool, err error) {

	// Make sure the channel is closed at the end of this function no matter what to
	// ensure graceful termination of all workers
	defer close(w.workloadChan)

	// Get list of years in main directory (ordered by directory name, i.e. time)
	yearList, err := os.ReadDir(w.dbIfaceDir)
	if err != nil {
		return false, err
	}

	// loop over directory list in order to create the timestamp pairs
	var (
		gpFileOptions       []gpfile.Option
		unixFirst, unixLast = time.Unix(tfirst, 0), time.Unix(tlast+DBWriteInterval, 0)
	)
	if !query.lowMem {
		w.memPool = gpfile.NewMemPool(w.numProcessingUnits * len(query.columnIndices))
		gpFileOptions = append(gpFileOptions, gpfile.WithReadAll(w.memPool))
	}
	w.tFirstCovered, w.tLastCovered = tfirst, tlast

	// make sure to start with zero workloads as the number of assigned
	// workloads depends on how many directories have to be read
	var (
		numDirs int
		curDir  *gpfile.GPDir
	)
	workloadBulk := make([]*gpfile.GPDir, 0, WorkBulkSize)
	for _, year := range yearList {

		// Skip obvious non-matching entries
		if !year.IsDir() || year.Name() == "./" || year.Name() == "../" {
			continue
		}

		// Skip if outside of annual range
		yearTimestamp, err := strconv.Atoi(year.Name())
		if err != nil {
			return false, fmt.Errorf("failed to parse year from directory `%s`: %w", year.Name(), err)
		}
		if yearTimestamp < unixFirst.Year() || yearTimestamp > unixLast.Year() {
			continue
		}

		// Get list of months in year directory (ordered by directory name, i.e. time)
		monthList, err := os.ReadDir(filepath.Join(w.dbIfaceDir, year.Name()))
		if err != nil {
			return false, err
		}
		for _, month := range monthList {

			// Skip obvious non-matching entries
			if !month.IsDir() || month.Name() == "./" || month.Name() == "../" {
				continue
			}

			// Skip if outside of month range (only considering the "edge" years)
			monthTimestamp, err := strconv.Atoi(month.Name())
			if err != nil {
				return false, fmt.Errorf("failed to parse month from directory `%s`: %w", year.Name(), err)
			}
			if (yearTimestamp == unixFirst.Year() && time.Month(monthTimestamp) < unixFirst.Month()) ||
				(yearTimestamp == unixLast.Year() && time.Month(monthTimestamp) > unixLast.Month()) {
				continue
			}

			// Get list of days in month directory (ordered by directory name, i.e. time)
			dirList, err := os.ReadDir(filepath.Join(w.dbIfaceDir, year.Name(), month.Name()))
			if err != nil {
				return false, err
			}

			for _, file := range dirList {
				if file.IsDir() && (file.Name() != "./" || file.Name() != "../") {
					dayTimestamp, err := strconv.ParseInt(file.Name(), 10, 64)
					if err != nil {
						return false, fmt.Errorf("failed to parse epoch timestamp from directory `%s`: %w", file.Name(), err)
					}

					// check if the directory is within time frame of interest
					if tfirst < dayTimestamp+gpfile.EpochDay && dayTimestamp < tlast+DBWriteInterval {
						curDir = gpfile.NewDir(w.dbIfaceDir, dayTimestamp, gpfile.ModeRead, gpFileOptions...)

						// For the first and last item, check out the GPDir metadata for the actual first and
						// last block timestamp to cover (and adapt variables accordingly)
						// We will grab the timestamp from the first visited / valid directory that fulfils
						// the timestamp condition on directory level
						if numDirs == 0 {
							if err := curDir.Open(); err != nil {
								return false, fmt.Errorf("failed to open first GPDir %s to ascertain query block timing: %w", curDir.Path(), err)
							}
							dirFirst, _ := curDir.TimeRange()
							if tfirst < dirFirst {
								w.tFirstCovered = dirFirst
							}
							if err := curDir.Close(); err != nil {
								return false, fmt.Errorf("failed to close first GPDir %s after ascertaining query block timing: %w", curDir.Path(), err)
							}
						}
						numDirs++

						// create new workload for the directory
						workloadBulk = append(workloadBulk, curDir)
						if len(workloadBulk) == WorkBulkSize {
							w.workloadChan <- DBWorkload{query: query, workDirs: workloadBulk}
							w.nWorkloads++
							workloadBulk = make([]*gpfile.GPDir, 0, WorkBulkSize)
						}
					}
				}
			}
		}
	}

	// Flush any remaining work
	if len(workloadBulk) > 0 {
		w.workloadChan <- DBWorkload{query: query, workDirs: workloadBulk}
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

// main query processing
func (w *DBWorkManager) grabAndProcessWorkload(ctx context.Context, wg *sync.WaitGroup, workloadChan <-chan DBWorkload, mapChan chan hashmap.AggFlowMapWithMetadata) {
	go func() {
		defer wg.Done()

		logger := logging.WithContext(ctx)

		enc, err := encoder.New(defaultEncoderType)
		if err != nil {
			logger.Error(err.Error())
			mapChan <- hashmap.NilAggFlowMapWithMetadata
		}
		defer enc.Close()

		var workload DBWorkload
		for chanOpen := true; chanOpen; {
			select {
			case <-ctx.Done():
				// query was cancelled, exit
				logger.Infof("Query cancelled (workload %d / %d)...", w.nWorkloadsProcessed, w.nWorkloads)
				return
			case workload, chanOpen = <-workloadChan:
				if chanOpen {
					resultMap := hashmap.NewAggFlowMapWithMetadata()
					for _, workDir := range workload.workDirs {

						// if there is an error during one of the read jobs, throw a syslog message and terminate
						err := w.readBlocksAndEvaluate(workDir, workload.query, enc, &resultMap)
						if err != nil {
							logger.Error(err.Error())
							mapChan <- hashmap.NilAggFlowMapWithMetadata
							return
						}
					}

					// Workload is counted, but we only add it to the final result if we got any entries
					w.nWorkloadsProcessed++
					if resultMap.Len() > 0 {
						mapChan <- resultMap
					}
				}
			}
		}
		return
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
func (w *DBWorkManager) readBlocksAndEvaluate(workDir *gpfile.GPDir, query *Query, enc encoder.Encoder, resultMap *hashmap.AggFlowMapWithMetadata) (err error) {
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
	defer workDir.Close()

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
		for _, colIdx := range query.columnIndices {

			// Read the block from the file
			if blocks[colIdx], err = workDir.ReadBlockAtIndex(colIdx, b); err != nil {
				blockBroken = true
				logger.Warnf("[D %s; B %d] Failed to read column %s: %s", workDir, block.Timestamp, types.ColumnFileNames[colIdx], err)
				break
			}
		}

		// Check whether all blocks have matching number of entries
		numV4Entries := int(workDir.NumIPv4EntriesAtIndex(b))
		numEntries := bitpack.Len(blocks[types.BytesRcvdColIdx])
		for _, colIdx := range query.columnIndices {
			l := len(blocks[colIdx])
			if colIdx.IsCounterCol() {
				if bitpack.Len(blocks[colIdx]) != numEntries {
					blockBroken = true
					logger.Warnf("[Bl %d] Incorrect number of entries in file [%s.gpf]. Expected %d, found %d", b, types.ColumnFileNames[colIdx], numEntries, bitpack.Len(blocks[colIdx]))
					break
				}
			} else {
				if types.ColumnSizeofs[colIdx] == types.IPSizeOf {
					if l != (numEntries-int(numV4Entries))*types.IPv6Width+int(numV4Entries)*types.IPv4Width {
						blockBroken = true
						logger.Warnf("[Bl %d] Incorrect number of entries in variable block size file [%s.gpf]. Expected file length %d, have %d", b, types.ColumnFileNames[colIdx], (numEntries-int(numV4Entries))*types.IPv6Width+int(numV4Entries)*types.IPv4Width, l)
						break
					}
				} else {
					if l/types.ColumnSizeofs[colIdx] != numEntries {
						blockBroken = true
						logger.Warnf("[Bl %d] Incorrect number of entries in column [%s.gpf]. Expected %d, found %d", b, types.ColumnFileNames[colIdx], numEntries, l/types.ColumnSizeofs[colIdx])
						break
					}
					if l%types.ColumnSizeofs[colIdx] != 0 {
						blockBroken = true
						logger.Warnf("[Bl %d] Entry size does not evenly divide block size in file [%s.gpf]", b, types.ColumnFileNames[colIdx])
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
		if query.hasAttrTime {
			v4Key = types.NewEmptyV4Key().Extend(block.Timestamp)
			v6Key = types.NewEmptyV6Key().Extend(block.Timestamp)
			if query.Conditional == nil {
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

		key, comparisonValue := v4Key, v4ComparisonValue
		startEntry, isIPv4 := 0, true // TODO: Support traversal of IPv4 / IPv6 only if there's a matching condition
		for i := startEntry; i < numEntries; i++ {

			// When reaching the v4/v6 mark, we switch to the IPv6 key / submap
			if i == int(numV4Entries) {
				key, comparisonValue = v6Key, v6ComparisonValue
				isIPv4 = false
			}

			// Populate key for current entry
			if query.hasAttrSip {
				if isIPv4 {
					key.PutSip(sipBlocks[i*4 : i*4+4])
				} else {
					key.PutSip(sipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
				}
			}
			if query.hasAttrDip {
				if isIPv4 {
					key.PutDipV4(dipBlocks[i*4 : i*4+4])
				} else {
					key.PutDipV6(dipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
				}
			}
			if query.hasAttrProto {
				key.PutProtoV(protoBlocks[i], isIPv4)
			}
			if query.hasAttrDport {
				key.PutDportV(dportBlocks[i*types.DportSizeof:i*types.DportSizeof+types.DportSizeof], isIPv4)
			}

			// Check whether conditional is satisfied for current entry
			var conditionalSatisfied bool
			if query.Conditional == nil {
				conditionalSatisfied = true
			} else {

				// Populate comparison value for current entry
				if query.hasCondSip {
					if isIPv4 {
						comparisonValue.PutSip(sipBlocks[i*4 : i*4+4])
					} else {
						comparisonValue.PutSip(sipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
					}
				}
				if query.hasCondDip {
					if isIPv4 {
						comparisonValue.PutDipV4(dipBlocks[i*4 : i*4+4])
					} else {
						comparisonValue.PutDipV6(dipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
					}
				}
				if query.hasCondProto {
					comparisonValue.PutProtoV(protoBlocks[i], isIPv4)
				}
				if query.hasCondDport {
					comparisonValue.PutDportV(dportBlocks[i*types.DportSizeof:i*types.DportSizeof+types.DportSizeof], isIPv4)
				}

				conditionalSatisfied = query.Conditional.Evaluate(comparisonValue.Key())
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
