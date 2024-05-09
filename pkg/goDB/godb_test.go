package goDB

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/stretchr/testify/require"
)

const (
	testNv4 = 100
	testNv6 = 50
)

type testCase struct {
	name                 string
	path                 string
	iface                string
	queryStart, queryEnd time.Time
	numWorkers           int
	nExpectedWorkloads   uint64
	nExpectedDays        int

	expectedErr error
}

func TestWorkload(t *testing.T) {

	// Initialize temporary test directory
	testPath, err := os.MkdirTemp("/tmp", "goDB")
	require.Nil(t, err)
	defer func(t *testing.T) {
		require.Nil(t, os.RemoveAll(testPath))
	}(t)
	require.Nil(t, os.Mkdir(filepath.Join(testPath, "eth0"), 0700))

	t.Run("invalid_numWorkers", func(t *testing.T) {
		testWorkload(t, testCase{
			path:        testPath,
			iface:       "eth0",
			queryStart:  time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
			queryEnd:    time.Date(1999, time.December, 31, 0, 0, 0, 0, time.UTC),
			numWorkers:  0,
			expectedErr: errors.New("invalid number of processing units: 0"),
		}, true)
	})

	t.Run("empty", func(t *testing.T) {
		testWorkload(t, testCase{
			path:       testPath,
			iface:      "eth0",
			queryStart: time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
			queryEnd:   time.Date(1999, time.December, 31, 0, 0, 0, 0, time.UTC),
			numWorkers: 1,
		}, true)
	})

	// Create some dummy data / directories
	nDays := 26
	for day := 1; day <= nDays; day++ {
		populateTestDir(t, testPath, "eth0", time.Date(2000, time.January, day, 0, 0, 0, 0, time.UTC))
		populateTestDir(t, testPath, "eth0", time.Date(2001, time.January, day, 0, 0, 0, 0, time.UTC))
		populateTestDir(t, testPath, "eth0", time.Date(2001, time.February, day, 0, 0, 0, 0, time.UTC))
		populateTestDir(t, testPath, "eth0", time.Date(2001, time.March, day, 0, 0, 0, 0, time.UTC))
		populateTestDir(t, testPath, "eth0", time.Date(2004, time.January, day, 0, 0, 0, 0, time.UTC))
	}

	var testCases = []testCase{
		{
			path:       testPath,
			name:       "before range",
			iface:      "eth0",
			queryStart: time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
			queryEnd:   time.Date(1999, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			path:       testPath,
			name:       "after range",
			iface:      "eth0",
			queryStart: time.Date(2005, time.February, 1, 0, 0, 0, 0, time.UTC),
			queryEnd:   time.Date(2200, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			path:               testPath,
			name:               "year exclusion",
			iface:              "eth0",
			queryStart:         time.Date(1990, time.February, 1, 0, 0, 0, 0, time.UTC),
			queryEnd:           time.Date(2000, time.October, 15, 0, 0, 0, 0, time.UTC),
			nExpectedWorkloads: 1,
			nExpectedDays:      nDays,
		},
		{
			path:               testPath,
			name:               "month exclusion",
			iface:              "eth0",
			queryStart:         time.Date(1990, time.February, 1, 0, 0, 0, 0, time.UTC),
			queryEnd:           time.Date(2001, time.February, 28, 0, 0, 0, 0, time.UTC),
			nExpectedWorkloads: 3,
			nExpectedDays:      3 * nDays,
		},
		{
			path:               testPath,
			name:               "month+day exclusion",
			iface:              "eth0",
			queryStart:         time.Date(1990, time.February, 1, 0, 0, 0, 0, time.UTC),
			queryEnd:           time.Date(2001, time.February, 15, 0, 0, 0, 0, time.UTC),
			nExpectedWorkloads: 3,
			nExpectedDays:      2*nDays + 15,
		},
	}

	for _, c := range testCases {
		t.Run(fmt.Sprintf("%s_1worker", c.name), func(t *testing.T) {
			c.numWorkers = 1
			testWorkload(t, c, true)  // dry-run (to ascertain correct number of workloads / directories)
			testWorkload(t, c, false) // actual processing
		})
		t.Run(fmt.Sprintf("%s_4workers", c.name), func(t *testing.T) {
			c.numWorkers = 4
			testWorkload(t, c, true)  // dry-run (to ascertain correct number of workloads / directories)
			testWorkload(t, c, false) // actual processing
		})
	}
}

func populateTestDir(t *testing.T, basePath, iface string, timestamp time.Time) {

	testPath := filepath.Join(basePath, iface)

	f := gpfile.NewDirWriter(testPath, timestamp.Unix())
	require.Nil(t, f.Open())

	data, update := dbData(generateFlows())
	require.Nil(t, f.WriteBlocks(timestamp.Unix()+300, gpfile.TrafficMetadata{
		NumV4Entries: update.Traffic.NumV4Entries,
		NumV6Entries: update.Traffic.NumV6Entries,
	}, update.Counts, data))
	require.Nil(t, f.Close())
}

func generateFlows() *hashmap.AggFlowMap {
	m := hashmap.AggFlowMap{
		PrimaryMap:   hashmap.New(),
		SecondaryMap: hashmap.New(),
	}

	for i := byte(0); i < testNv4; i++ {
		m.PrimaryMap.Set(types.NewV4KeyStatic(
			[4]byte{i, i, i, i},
			[4]byte{i, i, i, i},
			[]byte{i, i}, i), types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: uint64(i), PacketsSent: 0})
	}
	for i := byte(0); i < testNv6; i++ {
		m.SecondaryMap.Set(types.NewV6KeyStatic(
			[16]byte{i, i, i, i, i, i, i, i, i, i, i, i, i, i, i, i},
			[16]byte{i, i, i, i, i, i, i, i, i, i, i, i, i, i, i, i},
			[]byte{i, i}, i), types.Counters{BytesRcvd: 0, BytesSent: uint64(i), PacketsRcvd: 0, PacketsSent: uint64(i)})
	}

	return &m
}

func testWorkload(t *testing.T, c testCase, dryRun bool) {

	// Instantiate a new DBWorkManager
	workMgr, err := NewDBWorkManager(NewQuery([]types.Attribute{
		types.SIPAttribute{},
		types.DIPAttribute{},
		types.DportAttribute{},
		types.ProtoAttribute{}}, nil, types.LabelSelector{}), c.path, c.iface, c.numWorkers)
	if c.expectedErr == nil {
		require.Nil(t, err)
	} else {
		require.EqualError(t, err, c.expectedErr.Error())
		return
	}

	// Create the workloads
	nonempty, err := workMgr.CreateWorkerJobs(
		c.queryStart.Unix(),
		c.queryEnd.Unix(),
	)
	require.Nil(t, err)

	// Perform sanity checks
	if c.nExpectedDays == 0 && c.nExpectedWorkloads == 0 {
		require.False(t, nonempty)

		tFirst, tLast := workMgr.GetCoveredTimeInterval()
		require.Equal(t, c.queryStart.Add(-time.Duration(DBWriteInterval)*time.Second), tFirst.UTC())
		require.Equal(t, c.queryEnd, tLast.UTC())
	} else {
		require.True(t, nonempty)
		require.Equal(t, c.nExpectedWorkloads, workMgr.nWorkloads)

		if dryRun {
			var numDirs int
			for i := uint64(0); i < workMgr.nWorkloads; i++ {
				workload := <-workMgr.workloadChan
				numDirs += len(workload.workDirs)
			}
			require.Equal(t, c.nExpectedDays, numDirs)
		} else {

			// Run the workloads
			mapChan := make(chan hashmap.AggFlowMapWithMetadata, 1024)
			workMgr.ExecuteWorkerReadJobs(context.Background(), mapChan)
			close(mapChan)

			// Perform sanity checks on aggregated data
			require.Equal(t, int(c.nExpectedWorkloads), len(mapChan))
			for aggMap := range mapChan {
				require.Equal(t, testNv4, aggMap.PrimaryMap.Len())
				require.Equal(t, testNv6, aggMap.SecondaryMap.Len())
			}
		}
	}
}
