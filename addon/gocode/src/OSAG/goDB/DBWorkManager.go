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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"OSAG/goDB/bigendian"
)

const (
	EPOCH_DAY         int64 = 86400 // one day in seconds
	DB_WRITE_INTERVAL int64 = 300   // write out interval of capture probe
)

type DBWorkload struct {
	query    *Query
	work_dir string
	load     []int64
}

type DBWorkManager struct {
	dbIfaceDir         string // path to interface directory in DB, e.g. /path/to/db/eth0
	iface              string
	workloads          []DBWorkload
	numProcessingUnits int
}

func NewDBWorkManager(dbpath string, iface string, numProcessingUnits int) (*DBWorkManager, error) {
	// whenever a new workload is created the logging facility is set up
	if err := InitDBLog(); err != nil {
		return nil, err
	}

	return &DBWorkManager{filepath.Join(dbpath, iface), iface, []DBWorkload{}, numProcessingUnits}, nil
}

// make number of workloads available to the outside world for loop bounds etc.
func (w *DBWorkManager) GetNumWorkers() int {
	return len(w.workloads)
}

// used to determine the time span actually covered by the query
func (w *DBWorkManager) GetCoveredTimeInterval() (time.Time, time.Time) {

	numWorkers := len(w.workloads)
	lenLoad := len(w.workloads[numWorkers-1].load)

	first := w.workloads[0].load[0] - DB_WRITE_INTERVAL
	last := w.workloads[numWorkers-1].load[lenLoad-1]

	return time.Unix(first, 0), time.Unix(last, 0)
}

func (w *DBWorkManager) CreateWorkerJobs(tfirst int64, tlast int64, query *Query) (nonempty bool, err error) {
	// Get list of files in directory
	var dirList []os.FileInfo

	if dirList, err = ioutil.ReadDir(w.dbIfaceDir); err != nil {
		return false, err
	}

	// loop over directory list in order to create the timestamp pairs
	var (
		info_file *GPFile
		dir_name  string
	)

	// make sure to start with zero workloads as the number of assigned
	// workloads depends on how many directories have to be read
	numDirs := 0
	for _, file := range dirList {
		if file.IsDir() && (file.Name() != "./" || file.Name() != "../") {
			dir_name = file.Name()
			temp_dir_tstamp, _ := strconv.ParseInt(dir_name, 10, 64)

			// check if the directory is within time frame of interest
			if tfirst < temp_dir_tstamp+EPOCH_DAY && temp_dir_tstamp < tlast+DB_WRITE_INTERVAL {
				numDirs++

				// create new workload for the directory
				workload := DBWorkload{query: query, work_dir: dir_name, load: []int64{}}

				// retrieve all the relevant timestamps from one of the database files.
				if info_file, err = NewGPFile(w.dbIfaceDir + "/" + dir_name + "/bytes_rcvd.gpf"); err != nil {
					return false, errors.New("could not read file: " + w.dbIfaceDir + "/" + dir_name + "/bytes_rcvd.gpf")
				}

				// add the relevant timestamps to the workload's list
				for _, stamp := range info_file.GetTimestamps() {
					if stamp != 0 && tfirst < stamp && stamp < tlast+DB_WRITE_INTERVAL {
						workload.load = append(workload.load, stamp)
					}
				}
				info_file.Close()

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

// Processing units ---------------------------------------------------------------------
func (w *DBWorkManager) grabAndProcessWorkload(workloadChan <-chan DBWorkload, mapChan chan map[ExtraKey]Val, wg *sync.WaitGroup) {
	// parse conditions
	var err error

	for workload := range workloadChan {
		// create the map in which the workload will store the aggregations
		resultMap := make(map[ExtraKey]Val)

		// if there is an error during one of the read jobs, throw a syslog message and terminate
		if err = w.readBlocksAndEvaluate(workload, resultMap); err != nil {
			SysLog.Err(err.Error())
			mapChan <- nil
			wg.Done()
		}

		mapChan <- resultMap
	}

	wg.Done()
}

// Spawning of processing units and pushing of workload onto factory channel -----------
func (w *DBWorkManager) ExecuteWorkerReadJobs(mapChan chan map[ExtraKey]Val) {
	procsWG := sync.WaitGroup{}
	procsWG.Add(w.numProcessingUnits)

	workloadChan := make(chan DBWorkload, 128)
	for i := 0; i < w.numProcessingUnits; i++ {
		go w.grabAndProcessWorkload(workloadChan, mapChan, &procsWG)
	}

	// push the workloads onto the channel
	for _, workload := range w.workloads {
		workloadChan <- workload
	}
	close(workloadChan)

	procsWG.Wait()
}

// Array of functions to extract a specific entry from a block (represented as a byteslice)
// to a field in the Key struct.
var copyToKeyFns = [COLIDX_ATTRIBUTE_COUNT]func(int, *ExtraKey, []byte){
	func(i int, key *ExtraKey, bytes []byte) {
		copy(key.Sip[:], bytes[i*SIP_SIZEOF:i*SIP_SIZEOF+SIP_SIZEOF])
	},
	func(i int, key *ExtraKey, bytes []byte) {
		copy(key.Dip[:], bytes[i*DIP_SIZEOF:i*DIP_SIZEOF+DIP_SIZEOF])
	},
	func(i int, key *ExtraKey, bytes []byte) {
		key.Protocol = bytes[i*1]
	},
	func(i int, key *ExtraKey, bytes []byte) {
		copy(key.Dport[:], bytes[i*DPORT_SIZEOF:i*DPORT_SIZEOF+DPORT_SIZEOF])
	},
}

// Block evaluation and aggregation -----------------------------------------------------
// this is where the actual reading and aggregation magic happens
func (w *DBWorkManager) readBlocksAndEvaluate(workload DBWorkload, resultMap map[ExtraKey]Val) error {
	var err error

	var (
		query = workload.query
		dir   = workload.work_dir
	)

	var key, comparisonValue ExtraKey

	// Load the GPFiles corresponding to the columns we need for the query. Each file is loaded at most once.
	var columnFiles [COLIDX_COUNT]*GPFile
	for _, colIdx := range query.columnIndizes {
		if columnFiles[colIdx], err = NewGPFile(w.dbIfaceDir + "/" + dir + "/" + columnFileNames[colIdx] + ".gpf"); err == nil {
			defer columnFiles[colIdx].Close()
		} else {
			return err
		}
	}

	// Process the workload
	// The workload consists of timestamps whose blocks we should process.
	for b, tstamp := range workload.load {

		var (
			blocks      [COLIDX_COUNT][]byte
			blockBroken = false
		)

		for _, colIdx := range query.columnIndizes {

			// Read the block from the file
			if blocks[colIdx], err = columnFiles[colIdx].ReadTimedBlock(tstamp); err != nil {
				blockBroken = true
				SysLog.Warning(fmt.Sprintf("[D %s; B %d] Failed to read %s.gpf: %s", dir, tstamp, columnFileNames[colIdx], err.Error()))
				break
			}

			// Check whether timestamps contained in headers match
			blockTstamp := bigendian.ReadInt64At(blocks[colIdx], 0) // The timestamp header is 8 bytes
			if tstamp != blockTstamp {
				blockBroken = true
				SysLog.Warning(fmt.Sprintf("[Bl %d] Mismatch between timestamp in header [%d] of file [%s.gpf] and in block [%d]\n", b, tstamp, columnFileNames[colIdx], blockTstamp))
				break
			}

			// Cut off headers so we don't need to offset all later index calculations by 8
			blocks[colIdx] = blocks[colIdx][8:]
		}

		if query.hasAttrTime {
			key.Time = tstamp
		}

		if query.hasAttrIface {
			key.Iface = w.iface
		}

		// Check whether all blocks have matching number of entries
		numEntries := int((len(blocks[BYTESRCVD_COLIDX]) - 8) / 8) // Each block contains another timestamp as the last 8 bytes
		for _, colIdx := range query.columnIndizes {
			l := len(blocks[colIdx]) - 8 // subtract timestamp
			if l/columnSizeofs[colIdx] != numEntries {
				blockBroken = true
				SysLog.Warning(fmt.Sprintf("[Bl %d] Incorrect number of entries in file [%s.gpf]. Expected %d, found %d.\n", b, columnFileNames[colIdx], numEntries, l/columnSizeofs[colIdx]))
				break
			}
			if l%columnSizeofs[colIdx] != 0 {
				blockBroken = true
				SysLog.Warning(fmt.Sprintf("[Bl %d] Entry size does not evenly divide block size in file [%s.gpf]\n", b, columnFileNames[colIdx]))
				break
			}
		}

		// In case any error was observed during above sanity checks, skip this whole block
		if blockBroken {
			continue
		}

		// Iterate over block entries
		for i := 0; i < numEntries; i++ {
			// Populate key for current entry
			for _, colIdx := range query.queryAttributeIndizes {
				copyToKeyFns[colIdx](i, &key, blocks[colIdx])
			}

			// Check whether conditional is satisfied for current entry
			var conditionalSatisfied bool
			if query.Conditional == nil {
				conditionalSatisfied = true
			} else {
				// Populate comparison value for current entry
				for _, colIdx := range query.conditionalAttributeIndizes {
					copyToKeyFns[colIdx](i, &comparisonValue, blocks[colIdx])
				}

				conditionalSatisfied = query.Conditional.evaluate(&comparisonValue)
			}

			if conditionalSatisfied {
				// Update aggregates
				var delta Val
				delta.NBytesRcvd = bigendian.UnsafeReadUint64At(blocks[BYTESRCVD_COLIDX], i)
				delta.NBytesSent = bigendian.UnsafeReadUint64At(blocks[BYTESSENT_COLIDX], i)
				delta.NPktsRcvd = bigendian.UnsafeReadUint64At(blocks[PKTSRCVD_COLIDX], i)
				delta.NPktsSent = bigendian.UnsafeReadUint64At(blocks[PKTSSENT_COLIDX], i)

				if val, exists := resultMap[key]; exists {
					val.NBytesRcvd += delta.NBytesRcvd
					val.NBytesSent += delta.NBytesSent
					val.NPktsRcvd += delta.NPktsRcvd
					val.NPktsSent += delta.NPktsSent
					resultMap[key] = val
				} else {
					resultMap[key] = delta
				}
			}
		}
	}
	return nil
}
