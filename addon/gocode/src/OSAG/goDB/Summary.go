/////////////////////////////////////////////////////////////////////////////////
//
// summary.go
//
// Written by Lorenz Breidenbach lob@open.ch, January 2016
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	SUMMARY_FILE_NAME      = "summary.json"
	SUMMARY_LOCK_FILE_NAME = "summary.lock"
)

// Summary for a single interface
type InterfaceSummary struct {
	// Number of flows
	FlowCount uint64 `json:"flowcount"`
	// Total traffic volume in byte
	Traffic uint64 `json:"traffic"`
	Begin   int64  `json:"begin"`
	End     int64  `json:"end"`
}

type InterfaceSummaryUpdate struct {
	// Name of the interface. For example, "eth0".
	Interface string
	// Number of flows
	FlowCount uint64
	// Traffic volume in bytes
	Traffic   uint64
	Timestamp time.Time
}

// Summary for an entire database
type DBSummary struct {
	Interfaces map[string]InterfaceSummary `json:"interfaces"`
}

func NewDBSummary() *DBSummary {
	summ := new(DBSummary)
	interfaces := make(map[string]InterfaceSummary)
	summ.Interfaces = interfaces
	return summ
}

// LockDBSummary tries to acquire a lockfile for the database summary.
// Its return values indicate whether it successfully acquired the lock
// and whether a file system error occurred.
func LockDBSummary(dbpath string) (acquired bool, err error) {
	f, err := os.OpenFile(filepath.Join(dbpath, SUMMARY_LOCK_FILE_NAME), os.O_EXCL|os.O_CREATE, 0666)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		} else {
			return false, err
		}
	} else {
		f.Close()
		return true, nil
	}
}

// LockDBSummary removes the lockfile for the database summary.
// Its return values indicates whether a file system error occurred.
func UnlockDBSummary(dbpath string) (err error) {
	err = os.Remove(filepath.Join(dbpath, SUMMARY_LOCK_FILE_NAME))
	return
}

// Reads the summary of the given database.
// If multiple processes might be operating on
// the summary simultaneously, you should lock it first.
func ReadDBSummary(dbpath string) (*DBSummary, error) {
	result := NewDBSummary()
	var err error

	f, err := os.Open(filepath.Join(dbpath, SUMMARY_FILE_NAME))
	if err != nil {
		return result, err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(result)
	if err != nil {
		return result, err
	}
	if result.Interfaces == nil {
		result = NewDBSummary()
	}

	return result, err
}

// Writes a new summary for the given database.
// If multiple processes might be operating on
// the summary simultaneously, you should lock it first.
func WriteDBSummary(dbpath string, summ *DBSummary) error {
	f, err := os.Create(filepath.Join(dbpath, SUMMARY_FILE_NAME))
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(summ)
}

// Safely modifies the database summary when there are multiple processes accessing it.
//
// If no lock can be acquired after (roughly) timeout time, returns an error.
//
// modify is expected to obey the following contract:
// * The input summary is nil if no summary file is present.
// * modify returns the summary to be written (must be non-nil) and an error.
// * Since the summary is locked while modify is
//   running, modify shouldn't take longer than roughly half a second.
func ModifyDBSummary(dbpath string, timeout time.Duration, modify func(*DBSummary) (*DBSummary, error)) (modErr error) {
	// Back off exponentially in case of failure.
	// Retry for at most timeout time.
	wait := 50 * time.Millisecond
	waited := time.Duration(0)
	for {
		// lock
		acquired, err := LockDBSummary(dbpath)
		if err != nil {
			return err
		}

		if !acquired {
			if waited+wait <= timeout {
				time.Sleep(wait)
				waited += wait
				wait *= 2
				continue
			} else {
				break
			}
		}

		// deferred unlock
		defer func() {
			if err := UnlockDBSummary(dbpath); err != nil {
				modErr = err
			}
		}()

		// read
		summ, err := ReadDBSummary(dbpath)
		if err != nil {
			if os.IsNotExist(err) {
				summ = NewDBSummary()
			} else {
				return err
			}
		}

		// change
		summ, err = modify(summ)
		if err != nil {
			return err
		}

		// write
		return WriteDBSummary(dbpath, summ)
	}

	return fmt.Errorf("Failed to acquire database summary lockfile")
}

func (s *DBSummary) Update(u InterfaceSummaryUpdate) {
	is, exists := s.Interfaces[u.Interface]
	if !exists {
		is.Begin = u.Timestamp.Unix()
	}
	if u.Timestamp.Unix() < is.Begin {
		is.Begin = u.Timestamp.Unix()
	}
	is.FlowCount += u.FlowCount
	is.Traffic += u.Traffic
	if is.End < u.Timestamp.Unix() {
		is.End = u.Timestamp.Unix()
	}
	s.Interfaces[u.Interface] = is
}
