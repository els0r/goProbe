package goDB

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/v4/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/v4/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/v4/pkg/types/hashmap"
	"github.com/stretchr/testify/require"
)

func TestPanicDuringWrite(t *testing.T) {

	// Setup a temporary directory for the test DB
	tempDir, err := os.MkdirTemp(os.TempDir(), "dbwrite_test")
	require.Nil(t, err)
	defer require.Nil(t, os.RemoveAll(tempDir))

	timestamp := time.Now().Unix()
	dayTimestamp := gpfile.DirTimestamp(timestamp)
	dayUnix := time.Unix(dayTimestamp, 0)
	dirPath := filepath.Join(filepath.Join(tempDir, "test"), strconv.Itoa(dayUnix.Year()), fmt.Sprintf("%02d", dayUnix.Month()), strconv.FormatInt(dayTimestamp, 10))

	w := NewDBWriter(tempDir, "test", encoders.EncoderTypeNull).Permissions(0600)

	// Add a single item that will trigger a panic later
	testMap := hashmap.NewAggFlowMap()
	testMap.PrimaryMap.Set([]byte{0x0}, hashmap.Val{})

	t.Run("Write", func(t *testing.T) {
		require.Panics(t, func() {
			err := w.Write(testMap, capturetypes.CaptureStats{}, timestamp)
			_ = err
		})
		dirs, err := os.ReadDir(dirPath)
		require.Nil(t, err)
		require.Empty(t, dirs)
	})

	t.Run("WriteBulk", func(t *testing.T) {
		require.Panics(t, func() {
			err := w.WriteBulk([]BulkWorkload{
				{
					FlowMap:      testMap,
					CaptureStats: capturetypes.CaptureStats{},
					Timestamp:    timestamp,
				},
			}, timestamp)
			_ = err
		})
		dirs, err := os.ReadDir(dirPath)
		require.Nil(t, err)
		require.Empty(t, dirs)
	})

}
