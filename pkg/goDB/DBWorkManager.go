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
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder/bitpack"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/log"
)

const (

	// EpochDay is one day in seconds
	EpochDay int64 = 86400

	// DBWriteInterval defines the periodic write out interval of goProbe
	DBWriteInterval int64 = 300

	// MetaInfoFileName exposes the name of the file from which timestamp information is
	// obtained for the query plan
	MetaInfoFileName = "bytes_rcvd.gpf"
)

// DBWorkload stores all relevant parameters to load a block and execute a query on it
type DBWorkload struct {
	query   *Query
	workDir string
	load    []int64
}

// DBWorkManager schedules parallel processing of blocks relevant for a query
type DBWorkManager struct {
	dbIfaceDir         string // path to interface directory in DB, e.g. /path/to/db/eth0
	iface              string
	workloads          []DBWorkload
	numProcessingUnits int

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

	return &DBWorkManager{filepath.Join(dbpath, iface), iface, []DBWorkload{}, numProcessingUnits, l}, nil
}

// GetNumWorkers returns the number of workloads available to the outside world for loop bounds etc.
func (w *DBWorkManager) GetNumWorkers() int {
	return len(w.workloads)
}

// GetCoveredTimeInterval can be used to determine the time span actually covered by the query
func (w *DBWorkManager) GetCoveredTimeInterval() (time.Time, time.Time) {

	numWorkers := len(w.workloads)
	lenLoad := len(w.workloads[numWorkers-1].load)

	first := w.workloads[0].load[0] - DBWriteInterval
	last := w.workloads[numWorkers-1].load[lenLoad-1]

	return time.Unix(first, 0), time.Unix(last, 0)
}

// CreateWorkerJobs sets up all workloads for query execution
func (w *DBWorkManager) CreateWorkerJobs(tfirst int64, tlast int64, query *Query) (nonempty bool, err error) {
	// Get list of files in directory
	var dirList []os.FileInfo

	if dirList, err = ioutil.ReadDir(w.dbIfaceDir); err != nil {
		return false, err
	}

	// loop over directory list in order to create the timestamp pairs
	var (
		infoFile *gpfile.GPFile
		dirName  string
	)

	// make sure to start with zero workloads as the number of assigned
	// workloads depends on how many directories have to be read
	numDirs := 0
	for _, file := range dirList {
		if file.IsDir() && (file.Name() != "./" || file.Name() != "../") {
			dirName = file.Name()
			tempdirTstamp, _ := strconv.ParseInt(dirName, 10, 64)

			// check if the directory is within time frame of interest
			if tfirst < tempdirTstamp+EpochDay && tempdirTstamp < tlast+DBWriteInterval {
				numDirs++

				// create new workload for the directory
				workload := DBWorkload{query: query, workDir: dirName, load: []int64{}}

				// retrieve all the relevant timestamps from one of the database files.
				path := filepath.Join(w.dbIfaceDir, dirName, "bytes_rcvd.gpf")
				if infoFile, err = gpfile.New(path, gpfile.ModeRead); err != nil {
					return false, fmt.Errorf("Could not read file: %s: %s", path, err)
				}

				// add the relevant timestamps to the workload's list
				blockHeader, err := infoFile.Blocks()
				if err != nil {
					return false, fmt.Errorf("Could not get blocks from file: %s: %s", path, err)
				}
				for _, block := range blockHeader.OrderedList() {
					if tfirst < block.Timestamp && block.Timestamp < tlast+DBWriteInterval {
						workload.load = append(workload.load, block.Timestamp)
					}
				}
				infoFile.Close()

				// Assume we have a directory with timestamp td.
				// Assume that the first block in the directory has timestamp td + 10.
				// When tlast = td + 5, we have to scan the directory for blocks and create
				// a workload that has an empty load list. The rest of the code assumes
				// that the load isn't empty, so we check for this case here.
				if len(workload.load) > 0 {
					w.workloads = append(w.workloads, workload)
				}
			}
		}
	}

	return 0 < len(w.workloads), err
}

// main query processing
func (w *DBWorkManager) grabAndProcessWorkload(ctx context.Context, wg *sync.WaitGroup, workloadChan <-chan DBWorkload, mapChan chan map[string]Val) {
	go func() {
		defer wg.Done()

		// parse conditions
		var err error

		var workload DBWorkload
		for chanOpen := true; chanOpen; {
			select {
			case <-ctx.Done():
				// query was cancelled, exit
				return
			case workload, chanOpen = <-workloadChan:
				if chanOpen {
					// create the map in which the workload will store the aggregations
					resultMap := make(map[string]Val)

					// if there is an error during one of the read jobs, throw a syslog message and terminate
					if err = w.readBlocksAndEvaluate(ctx, workload, resultMap); err != nil {
						w.logger.Error(err.Error())
						mapChan <- nil
						return
					}

					mapChan <- resultMap
				}
			}
		}
		return
	}()
}

// ExecuteWorkerReadJobs runs the query concurrently with multiple sprocessing units
func (w *DBWorkManager) ExecuteWorkerReadJobs(ctx context.Context, mapChan chan map[string]Val) {
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

// Array of functions to extract a specific entry from a block (represented as a byteslice)
// to a field in the Key struct.
func genCopyToKeyFns(numV4 int) [ColIdxAttributeCount]func(int, ExtendedKey, []byte) {
	return [ColIdxAttributeCount]func(int, ExtendedKey, []byte){
		func(i int, key ExtendedKey, bytes []byte) {
			if key.IsIPv4() {
				key.Key().PutSip(bytes[i*4 : i*4+4])
			} else {
				key.Key().PutSip(bytes[numV4*4+(i-numV4)*16 : numV4*4+(i-numV4)*16+16])
			}
		},
		func(i int, key ExtendedKey, bytes []byte) {
			if key.IsIPv4() {
				key.Key().PutDip(bytes[i*4 : i*4+4])
			} else {
				key.Key().PutDip(bytes[numV4*4+(i-numV4)*16 : numV4*4+(i-numV4)*16+16])
			}
		},
		func(i int, key ExtendedKey, bytes []byte) {
			key.Key().PutProto(bytes[i])
		},
		func(i int, key ExtendedKey, bytes []byte) {
			key.Key().PutDport(bytes[i*DportSizeof : i*DportSizeof+DportSizeof])
		},
	}
}

// Block evaluation and aggregation -----------------------------------------------------
// this is where the actual reading and aggregation magic happens
func (w *DBWorkManager) readBlocksAndEvaluate(ctx context.Context, workload DBWorkload, resultMap map[string]Val) error {
	var err error

	var (
		query = workload.query
		dir   = workload.workDir
	)

	var (
		v4Key, v4ComparisonValue ExtendedKey
		v6Key, v6ComparisonValue ExtendedKey
	)

	// Load the GPFiles corresponding to the columns we need for the query. Each file is loaded at most once.
	var columnFiles [ColIdxCount]*gpfile.GPFile
	for _, colIdx := range query.columnIndices {
		if columnFiles[colIdx], err = gpfile.New(filepath.Join(w.dbIfaceDir, dir, columnFileNames[colIdx]+".gpf"), gpfile.ModeRead); err == nil {
			defer columnFiles[colIdx].Close()
		} else {
			return err
		}
	}

	// Process the workload
	// The workload consists of timestamps whose blocks we should process.
	for b, tstamp := range workload.load {
		select {
		case <-ctx.Done():
			w.logger.Infof("[D %s; B %d] Query cancelled. %d/%d blocks processed", dir, tstamp, b, len(workload.load))
			return nil
		default:
			var (
				blocks      [ColIdxCount][]byte
				blockBroken bool
				ts          int64
				iface       string
			)

			for _, colIdx := range query.columnIndices {

				// Read the block from the file
				if blocks[colIdx], err = columnFiles[colIdx].ReadBlock(tstamp); err != nil {
					blockBroken = true
					w.logger.Warnf("[D %s; B %d] Failed to read %s.gpf: %s", dir, tstamp, columnFileNames[colIdx], err.Error())
					break
				}
			}

			// Initialize any (static) key extensions potentially present in the query
			if query.hasAttrTime {
				ts = tstamp
			}
			if query.hasAttrIface {
				iface = w.iface
			}
			v4Key = NewEmptyV4Key().Extend(ts, iface)
			v6Key = NewEmptyV6Key().Extend(ts, iface)

			// Check whether all blocks have matching number of entries
			// TODO: Quick-shot, this information should be stored in the metadata for this directory instead !!!
			numV4Entries := binary.BigEndian.Uint64(blocks[BytesRcvdColIdx][:8])
			blocks[BytesRcvdColIdx] = blocks[BytesRcvdColIdx][8:]

			numEntries := bitpack.Len(blocks[BytesRcvdColIdx])
			for _, colIdx := range query.columnIndices {
				l := len(blocks[colIdx])
				if colIdx.IsCounterCol() {
					if bitpack.Len(blocks[colIdx]) != numEntries {
						blockBroken = true
						w.logger.Warnf("[Bl %d] Incorrect number of entries in file [%s.gpf]. Expected %d, found %d", b, columnFileNames[colIdx], numEntries, bitpack.Len(blocks[colIdx]))
						break
					}
				} else {
					if columnSizeofs[colIdx] == ipSizeOf {
						if l != (numEntries-int(numV4Entries))*types.IPv6Width+int(numV4Entries)*types.IPv4Width {
							blockBroken = true
							w.logger.Warnf("[Bl %d] Incorrect number of entries in variable block size file [%s.gpf]. Expected file length %d, have %d", b, columnFileNames[colIdx], (numEntries-int(numV4Entries))*types.IPv6Width+int(numV4Entries)*types.IPv4Width, l)
							break
						}
					} else {
						if l/columnSizeofs[colIdx] != numEntries {
							blockBroken = true
							w.logger.Warnf("[Bl %d] Incorrect number of entries in file [%s.gpf]. Expected %d, found %d", b, columnFileNames[colIdx], numEntries, l/columnSizeofs[colIdx])
							break
						}
						if l%columnSizeofs[colIdx] != 0 {
							blockBroken = true
							w.logger.Warnf("[Bl %d] Entry size does not evenly divide block size in file [%s.gpf]", b, columnFileNames[colIdx])
							break
						}
					}
				}
			}

			// In case any error was observed during above sanity checks, skip this whole block
			if blockBroken {
				continue
			}

			// Iterate over block entries
			byteWidthBytesRcvd := bitpack.ByteWidth(blocks[BytesRcvdColIdx])
			byteWidthBytesSent := bitpack.ByteWidth(blocks[BytesSentColIdx])
			byteWidthPktsRcvd := bitpack.ByteWidth(blocks[PacketsRcvdColIdx])
			byteWidthPktsSent := bitpack.ByteWidth(blocks[PacketsSentColIdx])

			key, comparisonValue := v4Key, v4ComparisonValue
			attrFns := genCopyToKeyFns(int(numV4Entries))
			for i := 0; i < numEntries; i++ {

				// When reaching the v4/v6 mark, we switch to the IPv6 key
				if i == int(numV4Entries) {
					key, comparisonValue = v6Key, v6ComparisonValue
				}

				// Populate key for current entry
				for _, colIdx := range query.queryAttributeIndices {
					attrFns[colIdx](i, key, blocks[colIdx])
				}

				// Check whether conditional is satisfied for current entry
				var conditionalSatisfied bool
				if query.Conditional == nil {
					conditionalSatisfied = true
				} else {
					// Populate comparison value for current entry
					for _, colIdx := range query.conditionalAttributeIndices {
						attrFns[colIdx](i, comparisonValue, blocks[colIdx])
					}

					conditionalSatisfied = query.Conditional.evaluate(comparisonValue.Key())
				}

				if conditionalSatisfied {
					// Update aggregates
					var delta Val

					// Unpack counters using bit packing
					delta.NBytesRcvd = bitpack.Uint64At(blocks[BytesRcvdColIdx], i, byteWidthBytesRcvd)
					delta.NBytesSent = bitpack.Uint64At(blocks[BytesSentColIdx], i, byteWidthBytesSent)
					delta.NPktsRcvd = bitpack.Uint64At(blocks[PacketsRcvdColIdx], i, byteWidthPktsRcvd)
					delta.NPktsSent = bitpack.Uint64At(blocks[PacketsSentColIdx], i, byteWidthPktsSent)

					if val, exists := resultMap[string(key)]; exists {
						val.NBytesRcvd += delta.NBytesRcvd
						val.NBytesSent += delta.NBytesSent
						val.NPktsRcvd += delta.NPktsRcvd
						val.NPktsSent += delta.NPktsSent
						resultMap[string(key)] = val
					} else {
						resultMap[string(key)] = delta
					}
				}
			}
		}
	}
	return nil
}
