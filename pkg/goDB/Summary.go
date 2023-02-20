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
	"fmt"
	"os"
	"path/filepath"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// Expose lockfile and summary file names
const (
	SummaryFileName     = "summary.json"
	SummaryLockFileName = "summary.lock"
)

// InterfaceSummary stores the summary for a single interface
type InterfaceSummary struct {
	// Number of flows
	FlowCount uint64 `json:"flowcount"`
	// Total traffic volume in byte
	Traffic uint64 `json:"traffic"`
	Begin   int64  `json:"begin"`
	End     int64  `json:"end"`
}

// InterfaceSummaryUpdate stores recent metrics for a given interface. It is used for incremental updates to the full summary
type InterfaceSummaryUpdate struct {
	// Name of the interface. For example, "eth0".
	Interface string
	// Number of flows
	FlowCount uint64
	// Traffic volume in bytes
	Traffic uint64
	// Number of IPv4 entries
	NumIPV4Entries uint64

	Timestamp time.Time
}

// DBSummary stores the summary for an entire database
type DBSummary struct {
	Interfaces map[string]InterfaceSummary `json:"interfaces"`
}

// NewDBSummary creates a new DBSummary for updates and reads
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
	f, err := os.OpenFile(filepath.Join(dbpath, SummaryLockFileName), os.O_EXCL|os.O_CREATE, 0666)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	f.Close()
	return true, nil
}

// UnlockDBSummary removes the lockfile for the database summary.
// Its return values indicates whether a file system error occurred.
func UnlockDBSummary(dbpath string) (err error) {
	err = os.Remove(filepath.Join(dbpath, SummaryLockFileName))
	return
}

// ReadDBSummary reads the summary of the given database.
// If multiple processes might be operating on
// the summary simultaneously, you should lock it first.
func ReadDBSummary(dbpath string) (*DBSummary, error) {
	result := NewDBSummary()
	var err error

	f, err := os.Open(filepath.Join(dbpath, SummaryFileName))
	if err != nil {
		return result, err
	}
	defer f.Close()

	err = jsoniter.NewDecoder(f).Decode(result)
	if err != nil {
		return result, err
	}
	if result.Interfaces == nil {
		result = NewDBSummary()
	}

	return result, err
}

// WriteDBSummary writes a new summary for the given database.
// If multiple processes might be operating on
// the summary simultaneously, you should lock it first.
func WriteDBSummary(dbpath string, summ *DBSummary) error {
	f, err := os.Create(filepath.Join(dbpath, SummaryFileName))
	if err != nil {
		return err
	}
	defer f.Close()

	return jsoniter.NewEncoder(f).Encode(summ)
}

// ModifyDBSummary safely modifies the database summary when there are multiple processes accessing it.
//
// If no lock can be acquired after (roughly) timeout time, returns an error.
//
// modify is expected to obey the following contract:
//   - The input summary is nil if no summary file is present.
//   - modify returns the summary to be written (must be non-nil) and an error.
//   - Since the summary is locked while modify is
//     running, modify shouldn't take longer than roughly half a second.
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

// Update updates the in-memory summary with the latest info
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
