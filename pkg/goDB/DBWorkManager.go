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

	"github.com/els0r/goProbe/pkg/goDB/encoder/bitpack"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/els0r/log"
)

const (

	// DBWriteInterval defines the periodic write out interval of goProbe
	DBWriteInterval int64 = 300
)

// DBWorkload stores all relevant parameters to load a block and execute a query on it
type DBWorkload struct {
	query   *Query
	workDir *gpfile.GPDir
}

// DBWorkManager schedules parallel processing of blocks relevant for a query
type DBWorkManager struct {
	dbIfaceDir         string // path to interface directory in DB, e.g. /path/to/db/eth0
	iface              string
	workloads          []DBWorkload
	numProcessingUnits int

	tFirstCovered, tLastCovered int64
	nWorkloadsProcessed         int
	memPool                     *gpfile.MemPool

	logger log.Logger
}

// NewDBWorkManager sets up a new work manager for executing queries
func NewDBWorkManager(dbpath string, iface string, numProcessingUnits int) (*DBWorkManager, error) {
	// whenever a new workload is created the logging facility is set up. Make sure to honor environments where syslog may not be available
	loggerStr := os.Getenv("GODB_LOGGER")
	if !(loggerStr == "devnull" || loggerStr == "console") {
		loggerStr = "syslog"
	}
	l, err := log.NewFromString(loggerStr)
	if err != nil {
		return nil, err
	}

	return &DBWorkManager{
		dbIfaceDir:         filepath.Join(dbpath, iface),
		iface:              iface,
		workloads:          []DBWorkload{},
		numProcessingUnits: numProcessingUnits,
		logger:             l,
	}, nil
}

// GetNumWorkers returns the number of workloads available to the outside world for loop bounds etc.
func (w *DBWorkManager) GetNumWorkers() int {
	return len(w.workloads)
}

// GetCoveredTimeInterval can be used to determine the time span actually covered by the query
func (w *DBWorkManager) GetCoveredTimeInterval() (time.Time, time.Time) {
	return time.Unix(w.tFirstCovered-DBWriteInterval, 0), time.Unix(w.tLastCovered, 0)
}

// CreateWorkerJobs sets up all workloads for query execution
func (w *DBWorkManager) CreateWorkerJobs(tfirst int64, tlast int64, query *Query) (nonempty bool, err error) {

	// Get list of files in directory (ordered by file name, i.e. time)
	dirList, err := os.ReadDir(w.dbIfaceDir)
	if err != nil {
		return false, err
	}

	// loop over directory list in order to create the timestamp pairs
	var (
		gpFileOptions []gpfile.Option
		dirName       string
	)
	if !query.lowMem {
		w.memPool = gpfile.NewMemPool(w.numProcessingUnits * len(query.columnIndices))
		gpFileOptions = append(gpFileOptions, gpfile.WithReadAll(w.memPool))
	}
	w.tFirstCovered, w.tLastCovered = tfirst, tlast

	// make sure to start with zero workloads as the number of assigned
	// workloads depends on how many directories have to be read
	numDirs := 0
	for i, file := range dirList {
		if file.IsDir() && (file.Name() != "./" || file.Name() != "../") {
			dirName = file.Name()
			tempdirTstamp, _ := strconv.ParseInt(dirName, 10, 64)

			// check if the directory is within time frame of interest
			if tfirst < tempdirTstamp+gpfile.EpochDay && tempdirTstamp < tlast+DBWriteInterval {
				numDirs++

				dir := gpfile.NewDir(w.dbIfaceDir, tempdirTstamp, gpfile.ModeRead, gpFileOptions...)

				// For the first and last item, check out the GPDir metadata for the actual first and
				// last block timestamp to cover (and adapt variables accordingly)
				if i == 0 || i == len(dirList)-1 {
					if err := dir.Open(); err != nil {
						return false, fmt.Errorf("failed to open GPDir %s to ascertain query block timing: %w", dir.Path(), err)
					}
					defer dir.Close()

					dirFirst, dirLast := dir.TimeRange()
					if i == 0 && tfirst < dirFirst {
						w.tFirstCovered = dirFirst
					} else if i == len(dirList)-1 && tlast > dirLast {
						w.tLastCovered = dirLast
					}
				}

				// create new workload for the directory
				w.workloads = append(w.workloads, DBWorkload{query: query, workDir: dir})
			}
		}
	}

	return 0 < len(w.workloads), err
}

// main query processing
func (w *DBWorkManager) grabAndProcessWorkload(ctx context.Context, wg *sync.WaitGroup, workloadChan <-chan DBWorkload, mapChan chan hashmap.AggFlowMapWithMetadata) {
	go func() {
		defer wg.Done()

		// parse conditions
		var workload DBWorkload
		for chanOpen := true; chanOpen; {
			select {
			case <-ctx.Done():
				// query was cancelled, exit
				w.logger.Infof("Query cancelled (workload %d / %d)...", w.nWorkloadsProcessed, len(w.workloads))
				return
			case workload, chanOpen = <-workloadChan:
				if chanOpen {
					// create the map in which the workload will store the aggregations
					resultMap := hashmap.AggFlowMapWithMetadata{
						Map: hashmap.New(),
					}

					// if there is an error during one of the read jobs, throw a syslog message and terminate
					err := w.readBlocksAndEvaluate(workload, resultMap)
					if err != nil {
						w.logger.Error(err.Error())
						mapChan <- hashmap.AggFlowMapWithMetadata{Map: nil}
						return
					}

					w.nWorkloadsProcessed++
					mapChan <- resultMap
				}
			}
		}
		return
	}()
}

// ExecuteWorkerReadJobs runs the query concurrently with multiple sprocessing units
func (w *DBWorkManager) ExecuteWorkerReadJobs(ctx context.Context, mapChan chan hashmap.AggFlowMapWithMetadata) {
	workloadChan := make(chan DBWorkload, len(w.workloads))

	var wg = new(sync.WaitGroup)
	wg.Add(w.numProcessingUnits)
	for i := 0; i < w.numProcessingUnits; i++ {
		// start worker up
		w.grabAndProcessWorkload(ctx, wg, workloadChan, mapChan)
	}

	// push the workloads onto the channel
	for _, workload := range w.workloads {
		workloadChan <- workload
	}
	close(workloadChan)

	// check if all workers are done
	wg.Wait()
}

// Block evaluation and aggregation -----------------------------------------------------
// this is where the actual reading and aggregation magic happens
func (w *DBWorkManager) readBlocksAndEvaluate(workload DBWorkload, resultMap hashmap.AggFlowMapWithMetadata) error {
	var err error

	var (
		query = workload.query
		dir   = workload.workDir
	)

	var (
		v4Key, v4ComparisonValue                                         = types.NewEmptyV4Key().ExtendEmpty(), types.NewEmptyV4Key().ExtendEmpty()
		v6Key, v6ComparisonValue                                         = types.NewEmptyV6Key().ExtendEmpty(), types.NewEmptyV6Key().ExtendEmpty()
		bytesRcvdValues, bytesSentValues, pktsRcvdValues, pktsSentValues []uint64
	)

	// Open GPDir (reading metadata in the process)
	if err := workload.workDir.Open(); err != nil {
		return err
	}
	defer workload.workDir.Close()

	// Process the workload, looping over all blocks in this directory
	for b, block := range dir.BlockMetadata[0].Blocks() {

		// If this block is outside of the rannge, skip it (only happens at the very first
		// and /or very last directory)
		if block.Timestamp < w.tFirstCovered || block.Timestamp > w.tLastCovered {
			continue
		}

		var (
			blocks      [types.ColIdxCount][]byte
			blockBroken bool
			ts          int64
			iface       string
		)

		// Read the blocks from their files
		for _, colIdx := range query.columnIndices {

			// Read the block from the file
			if blocks[colIdx], err = dir.ReadBlockAtIndex(colIdx, b); err != nil {
				blockBroken = true
				w.logger.Warnf("[D %s; B %d] Failed to read column %s: %s", dir, block.Timestamp, types.ColumnFileNames[colIdx], err.Error())
				break
			}
		}

		// Check whether all blocks have matching number of entries
		numV4Entries := int(dir.NumIPv4EntriesAtIndex(b))
		numEntries := bitpack.Len(blocks[types.BytesRcvdColIdx])
		for _, colIdx := range query.columnIndices {
			l := len(blocks[colIdx])
			if colIdx.IsCounterCol() {
				if bitpack.Len(blocks[colIdx]) != numEntries {
					blockBroken = true
					w.logger.Warnf("[Bl %d] Incorrect number of entries in file [%s.gpf]. Expected %d, found %d", b, types.ColumnFileNames[colIdx], numEntries, bitpack.Len(blocks[colIdx]))
					break
				}
			} else {
				if types.ColumnSizeofs[colIdx] == types.IPSizeOf {
					if l != (numEntries-int(numV4Entries))*types.IPv6Width+int(numV4Entries)*types.IPv4Width {
						blockBroken = true
						w.logger.Warnf("[Bl %d] Incorrect number of entries in variable block size file [%s.gpf]. Expected file length %d, have %d", b, types.ColumnFileNames[colIdx], (numEntries-int(numV4Entries))*types.IPv6Width+int(numV4Entries)*types.IPv4Width, l)
						break
					}
				} else {
					if l/types.ColumnSizeofs[colIdx] != numEntries {
						blockBroken = true
						w.logger.Warnf("[Bl %d] Incorrect number of entries in column [%s.gpf]. Expected %d, found %d", b, types.ColumnFileNames[colIdx], numEntries, l/types.ColumnSizeofs[colIdx])
						break
					}
					if l%types.ColumnSizeofs[colIdx] != 0 {
						blockBroken = true
						w.logger.Warnf("[Bl %d] Entry size does not evenly divide block size in file [%s.gpf]", b, types.ColumnFileNames[colIdx])
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
		if query.hasAttrTime || query.hasAttrIface {
			if query.hasAttrTime {
				ts = block.Timestamp
			}
			if query.hasAttrIface {
				iface = w.iface
			}
			v4Key = types.NewEmptyV4Key().Extend(ts, iface)
			v6Key = types.NewEmptyV6Key().Extend(ts, iface)
			if query.Conditional == nil {
				v4ComparisonValue = types.NewEmptyV4Key().Extend(ts, iface)
				v6ComparisonValue = types.NewEmptyV6Key().Extend(ts, iface)
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

			// When reaching the v4/v6 mark, we switch to the IPv6 key
			if i == int(numV4Entries) {
				key, comparisonValue = v6Key, v6ComparisonValue
				isIPv4 = false
			}

			// Populate key for current entry
			if query.hasAttrSip {
				if isIPv4 {
					key.PutSipV4(sipBlocks[i*4 : i*4+4])
				} else {
					key.PutSipV6(sipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
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
				key.PutProto(protoBlocks[i])
			}
			if query.hasAttrDport {
				key.PutDport(dportBlocks[i*types.DportSizeof : i*types.DportSizeof+types.DportSizeof])
			}

			// Check whether conditional is satisfied for current entry
			var conditionalSatisfied bool
			if query.Conditional == nil {
				conditionalSatisfied = true
			} else {

				// Populate comparison value for current entry
				if query.hasCondSip {
					if isIPv4 {
						comparisonValue.PutSipV4(sipBlocks[i*4 : i*4+4])
					} else {
						comparisonValue.PutSipV6(sipBlocks[numV4Entries*4+(i-numV4Entries)*16 : numV4Entries*4+(i-numV4Entries)*16+16])
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
					comparisonValue.PutProto(protoBlocks[i])
				}
				if query.hasCondDport {
					comparisonValue.PutDport(dportBlocks[i*types.DportSizeof : i*types.DportSizeof+types.DportSizeof])
				}

				conditionalSatisfied = query.Conditional.evaluate(comparisonValue.Key())
			}

			if conditionalSatisfied {
				resultMap.SetOrUpdate(key,
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
