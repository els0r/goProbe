package goDB

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestWorkloadCreation(t *testing.T) {

	// Initialize temporary test directory
	testPath, err := os.MkdirTemp("/tmp/test_db", "goDB")
	require.Nil(t, err)
	defer os.RemoveAll(testPath)
	require.Nil(t, os.Mkdir(filepath.Join(testPath, "eth0"), 0700))

	t.Run("empty", func(t *testing.T) {
		workMgr, err := NewDBWorkManager(testPath, "eth0", 1)
		require.Nil(t, err)

		nonempty, err := workMgr.CreateWorkerJobs(
			time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC).Unix(),
			time.Date(1999, time.December, 31, 0, 0, 0, 0, time.UTC).Unix(),
			&Query{},
		)
		require.Nil(t, err)
		require.False(t, nonempty)
	})

	// Create some dummy data / directories
	for day := 1; day < 27; day++ {
		f := gpfile.NewDir(filepath.Join(testPath, "eth0"), time.Date(2000, time.January, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.ModeWrite)
		require.Nil(t, f.Open())
		require.Nil(t, f.WriteBlocks(time.Date(2000, time.January, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.TrafficMetadata{}, types.Counters{}, [types.ColIdxCount][]byte{}))
		require.Nil(t, f.Close())

		f = gpfile.NewDir(filepath.Join(testPath, "eth0"), time.Date(2001, time.January, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.ModeWrite)
		require.Nil(t, f.Open())
		require.Nil(t, f.WriteBlocks(time.Date(2001, time.January, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.TrafficMetadata{}, types.Counters{}, [types.ColIdxCount][]byte{}))
		require.Nil(t, f.Close())

		f = gpfile.NewDir(filepath.Join(testPath, "eth0"), time.Date(2001, time.February, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.ModeWrite)
		require.Nil(t, f.Open())
		require.Nil(t, f.WriteBlocks(time.Date(2001, time.February, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.TrafficMetadata{}, types.Counters{}, [types.ColIdxCount][]byte{}))
		require.Nil(t, f.Close())

		f = gpfile.NewDir(filepath.Join(testPath, "eth0"), time.Date(2001, time.March, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.ModeWrite)
		require.Nil(t, f.Open())
		require.Nil(t, f.WriteBlocks(time.Date(2001, time.March, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.TrafficMetadata{}, types.Counters{}, [types.ColIdxCount][]byte{}))
		require.Nil(t, f.Close())

		f = gpfile.NewDir(filepath.Join(testPath, "eth0"), time.Date(2004, time.January, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.ModeWrite)
		require.Nil(t, f.Open())
		require.Nil(t, f.WriteBlocks(time.Date(2004, time.January, day, 0, 0, 0, 0, time.UTC).Unix(), gpfile.TrafficMetadata{}, types.Counters{}, [types.ColIdxCount][]byte{}))
		require.Nil(t, f.Close())
	}

	t.Run("before range", func(t *testing.T) {
		workMgr, err := NewDBWorkManager(testPath, "eth0", 1)
		require.Nil(t, err)

		nonempty, err := workMgr.CreateWorkerJobs(
			time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC).Unix(),
			time.Date(1999, time.December, 31, 0, 0, 0, 0, time.UTC).Unix(),
			&Query{},
		)
		require.Nil(t, err)
		require.False(t, nonempty)
	})

	t.Run("after range", func(t *testing.T) {
		workMgr, err := NewDBWorkManager(testPath, "eth0", 1)
		require.Nil(t, err)

		nonempty, err := workMgr.CreateWorkerJobs(
			time.Date(2005, time.February, 1, 0, 0, 0, 0, time.UTC).Unix(),
			time.Date(2200, time.December, 31, 0, 0, 0, 0, time.UTC).Unix(),
			&Query{},
		)
		require.Nil(t, err)
		require.False(t, nonempty)
	})

	t.Run("year exclusion", func(t *testing.T) {
		workMgr, err := NewDBWorkManager(testPath, "eth0", 1)
		require.Nil(t, err)

		nonempty, err := workMgr.CreateWorkerJobs(
			time.Date(1990, time.February, 1, 0, 0, 0, 0, time.UTC).Unix(),
			time.Date(2000, time.October, 15, 0, 0, 0, 0, time.UTC).Unix(),
			&Query{},
		)
		require.Nil(t, err)
		require.True(t, nonempty)
		require.Equal(t, 1, workMgr.nWorkloads)

		var numDirs int
		for i := 0; i < workMgr.nWorkloads; i++ {
			workload := <-workMgr.workloadChan
			numDirs += len(workload.workDirs)
		}
		require.Equal(t, 26, numDirs)
	})

	t.Run("month exclusion", func(t *testing.T) {
		workMgr, err := NewDBWorkManager(testPath, "eth0", 1)
		require.Nil(t, err)

		nonempty, err := workMgr.CreateWorkerJobs(
			time.Date(1990, time.February, 1, 0, 0, 0, 0, time.UTC).Unix(),
			time.Date(2001, time.February, 28, 0, 0, 0, 0, time.UTC).Unix(),
			&Query{},
		)
		require.Nil(t, err)
		require.True(t, nonempty)
		require.Equal(t, 3, workMgr.nWorkloads)

		var numDirs int
		for i := 0; i < workMgr.nWorkloads; i++ {
			workload := <-workMgr.workloadChan
			numDirs += len(workload.workDirs)
		}
		require.Equal(t, 78, numDirs)
	})

	t.Run("month+day exclusion", func(t *testing.T) {
		workMgr, err := NewDBWorkManager(testPath, "eth0", 1)
		require.Nil(t, err)

		nonempty, err := workMgr.CreateWorkerJobs(
			time.Date(1990, time.February, 1, 0, 0, 0, 0, time.UTC).Unix(),
			time.Date(2001, time.February, 15, 0, 0, 0, 0, time.UTC).Unix(),
			&Query{},
		)
		require.Nil(t, err)
		require.True(t, nonempty)
		require.Equal(t, 3, workMgr.nWorkloads)

		var numDirs int
		for i := 0; i < workMgr.nWorkloads; i++ {
			workload := <-workMgr.workloadChan
			numDirs += len(workload.workDirs)
		}
		require.Equal(t, 67, numDirs)
	})
}
