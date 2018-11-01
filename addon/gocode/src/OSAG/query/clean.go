/////////////////////////////////////////////////////////////////////////////////
//
// clean.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"OSAG/goDB"
)

type cleanIfaceResult struct {
	DeltaFlowCount uint64 // number of flows deleted
	DeltaTraffic   uint64 // traffic bytes deleted
	NewBegin       int64  // timestamp of new begin
	Gone           bool   // The interface has no entries left
}

func cleanIfaceDir(dbPath string, timestamp int64, iface string) (result cleanIfaceResult, err error) {
	dayTimestamp := goDB.DayTimestamp(timestamp)

	entries, err := ioutil.ReadDir(filepath.Join(dbPath, iface))
	if err != nil {
		return result, err
	}

	result.NewBegin = math.MaxInt64

	clean := true
	for _, entry := range entries {
		if !entry.IsDir() {
			clean = false
			continue
		}

		dirTimestamp, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil || fmt.Sprintf("%d", dirTimestamp) != entry.Name() {
			// a directory whose name isn't an int64 wasn't created by
			// goProbe; leave it untouched
			clean = false
			continue
		}

		entryPath := filepath.Join(dbPath, iface, entry.Name())
		metaFilePath := filepath.Join(entryPath, goDB.METADATA_FILE_NAME)

		if dirTimestamp < dayTimestamp {
			// delete directory

			meta := goDB.TryReadMetadata(metaFilePath)

			if err := os.RemoveAll(entryPath); err != nil {
				return result, err
			}

			for _, block := range meta.Blocks {
				result.DeltaFlowCount += block.FlowCount
				result.DeltaTraffic += block.Traffic
			}
		} else {
			clean = false
			if dirTimestamp < result.NewBegin {
				// update NewBegin
				meta := goDB.TryReadMetadata(metaFilePath)
				if len(meta.Blocks) > 0 && meta.Blocks[0].Timestamp < result.NewBegin {
					result.NewBegin = meta.Blocks[0].Timestamp
				}
			}

		}
	}

	result.Gone = result.NewBegin == math.MaxInt64

	if clean {
		if err := os.RemoveAll(filepath.Join(dbPath, iface)); err != nil {
			return result, err
		}
	}

	return
}

// Cleans up all directories that cannot contain any flow records
// recorded at timestamp or later.
func cleanOldDBDirs(dbPath string, timestamp int64) error {
	if timestamp >= time.Now().Unix() {
		return fmt.Errorf("I can only clean up database entries from the past.")
	}

	ifaces, err := ioutil.ReadDir(dbPath)
	if err != nil {
		return err
	}

	// Contains changes required to each interface's summary
	ifaceResults := make(map[string]cleanIfaceResult)

	for _, iface := range ifaces {
		if !iface.IsDir() {
			continue
		}

		result, err := cleanIfaceDir(dbPath, timestamp, iface.Name())
		if err != nil {
			return err
		}
		ifaceResults[iface.Name()] = result
	}

	return goDB.ModifyDBSummary(dbPath, 10*time.Second, func(summ *goDB.DBSummary) (*goDB.DBSummary, error) {
		if summ == nil {
			return summ, fmt.Errorf("Cannot update summary: Summary missing")
		}

		for iface, change := range ifaceResults {
			if change.Gone {
				delete(summ.Interfaces, iface)
			} else {
				ifaceSumm := summ.Interfaces[iface]
				ifaceSumm.FlowCount -= change.DeltaFlowCount
				ifaceSumm.Traffic -= change.DeltaTraffic
				ifaceSumm.Begin = change.NewBegin
				summ.Interfaces[iface] = ifaceSumm
			}
		}

		return summ, nil
	})
}
