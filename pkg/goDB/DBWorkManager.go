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
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/log"
)

const (

	// EpochDay is one day in seconds
	EpochDay int64 = 86400

	// DBWriteInterval defines the periodic write out interval of goProbe
	DBWriteInterval int64 = 300
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
func (w *DBWorkManager) grabAndProcessWorkload(workloadChan <-chan DBWorkload, mapChan chan map[ExtraKey]Val, cancel <-chan struct{}) <-chan struct{} {

	done := make(chan struct{})

	go func() {
		defer func(doneChan chan struct{}) {
			doneChan <- struct{}{}
		}(done)

		// parse conditions
		var err error

		var workload DBWorkload
		for chanOpen := true; chanOpen; {
			select {
			case <-cancel:
				return
			case workload, chanOpen = <-workloadChan:
				if chanOpen {
					// create the map in which the workload will store the aggregations
					resultMap := make(map[ExtraKey]Val)

					// if there is an error during one of the read jobs, throw a syslog message and terminate
					if err = w.readBlocksAndEvaluate(workload, resultMap); err != nil {

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
	return done
}

type workerCommunication struct {
	cancel chan struct{}
	done   <-chan struct{}
}

// ExecuteWorkerReadJobs runs the query concurrently with multiple sprocessing units
func (w *DBWorkManager) ExecuteWorkerReadJobs(mapChan chan map[ExtraKey]Val, memErrors <-chan error) error {

	workloadChan := make(chan DBWorkload, len(w.workloads))

	var controlChannels []workerCommunication

	for i := 0; i < w.numProcessingUnits; i++ {
		comms := workerCommunication{cancel: make(chan struct{}, 1)}

		// start worker up
		comms.done = w.grabAndProcessWorkload(workloadChan, mapChan, comms.cancel)

		controlChannels = append(controlChannels, comms)
	}

	// push the workloads onto the channel
	for _, workload := range w.workloads {
		workloadChan <- workload
	}
	close(workloadChan)

	// check if the workers are done and also monitor memory
	var (
		err       error
		completed int
	)
	for {
		for i, c := range controlChannels {
			select {
			case err = <-memErrors:
				if err != nil {
					// log the memory error and assign type memory breach
					// for callers of this function
					w.logger.Error(err)

					// send cancel to all workers
					for _, c := range controlChannels {
						c.cancel <- struct{}{}
					}
				}
			case <-c.done:
				completed++
				w.logger.Debugf("worker %d finished, %d/%d are done", i, completed, w.numProcessingUnits)

				// return once done with processing
				if completed == w.numProcessingUnits {
					return err
				}
			}
		}
	}
}

// Array of functions to extract a specific entry from a block (represented as a byteslice)
// to a field in the Key struct.
var copyToKeyFns = [ColIdxAttributeCount]func(int, *ExtraKey, []byte){
	func(i int, key *ExtraKey, bytes []byte) {
		copy(key.Sip[:], bytes[i*SipSizeof:i*SipSizeof+SipSizeof])
	},
	func(i int, key *ExtraKey, bytes []byte) {
		copy(key.Dip[:], bytes[i*DipSizeof:i*DipSizeof+DipSizeof])
	},
	func(i int, key *ExtraKey, bytes []byte) {
		key.Protocol = bytes[i*1]
	},
	func(i int, key *ExtraKey, bytes []byte) {
		copy(key.Dport[:], bytes[i*DportSizeof:i*DportSizeof+DportSizeof])
	},
}

// Block evaluation and aggregation -----------------------------------------------------
// this is where the actual reading and aggregation magic happens
func (w *DBWorkManager) readBlocksAndEvaluate(workload DBWorkload, resultMap map[ExtraKey]Val) error {
	var err error

	var (
		query = workload.query
		dir   = workload.workDir
	)

	var key, comparisonValue ExtraKey

	// Load the GPFiles corresponding to the columns we need for the query. Each file is loaded at most once.
	var columnFiles [ColIdxCount]*gpfile.GPFile
	for _, colIdx := range query.columnIndizes {
		if columnFiles[colIdx], err = gpfile.New(w.dbIfaceDir+"/"+dir+"/"+columnFileNames[colIdx]+".gpf", gpfile.ModeRead); err == nil {
			defer columnFiles[colIdx].Close()
		} else {
			return err
		}
	}

	// Process the workload
	// The workload consists of timestamps whose blocks we should process.
	for b, tstamp := range workload.load {

		var (
			blocks      [ColIdxCount][]byte
			blockBroken = false
		)

		for _, colIdx := range query.columnIndizes {

			// Read the block from the file
			if blocks[colIdx], err = columnFiles[colIdx].ReadBlock(tstamp); err != nil {
				blockBroken = true
				w.logger.Warnf("[D %s; B %d] Failed to read %s.gpf: %s", dir, tstamp, columnFileNames[colIdx], err.Error())
				break
			}
		}

		if query.hasAttrTime {
			key.Time = tstamp
		}

		if query.hasAttrIface {
			key.Iface = w.iface
		}

		// Check whether all blocks have matching number of entries
		numEntries := int(len(blocks[BytesRcvdColIdx]) / 8) // Each block contains another timestamp as the last 8 bytes
		for _, colIdx := range query.columnIndizes {
			l := len(blocks[colIdx])
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

				delta.NBytesRcvd = binary.BigEndian.Uint64(blocks[BytesRcvdColIdx][i*8 : i*8+8])
				delta.NBytesSent = binary.BigEndian.Uint64(blocks[BytesSentColIdx][i*8 : i*8+8])
				delta.NPktsRcvd = binary.BigEndian.Uint64(blocks[PacketsRcvdColIdx][i*8 : i*8+8])
				delta.NPktsSent = binary.BigEndian.Uint64(blocks[PacketsSentColIdx][i*8 : i*8+8])

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
