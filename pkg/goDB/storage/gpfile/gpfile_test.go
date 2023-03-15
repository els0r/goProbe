package gpfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage"
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

const (
	testBasePath = "/tmp"
)

var (
	testFilePath    = filepath.Join(testBasePath, "test.gpf")
	invalidFilePath = filepath.Join(testBasePath, "invalid.gpf")

	testEncoders = []encoders.Type{
		encoders.EncoderTypeLZ4,
		encoders.EncoderTypeNull,
	}
)

func TestFailedRead(t *testing.T) {
	_, err := New(testFilePath, nil, ModeRead)
	require.Error(t, err, "expected an error trying to open a non-existing GPFile for reading, got none")
}

func TestCreateFile(t *testing.T) {
	gpf, err := New(testFilePath, newMetadata().BlockMetadata[0], ModeWrite)
	require.Nil(t, err, "failed to create new GPFile")
	defer gpf.Delete()

	require.Nil(t, gpf.validateBlocks(0), "failed to validate blocks")
	require.Nil(t, gpf.Close(), "failed to close test file")
}

func TestWriteFile(t *testing.T) {
	gpf, err := New(testFilePath, newMetadata().BlockMetadata[0], ModeWrite)
	require.Nil(t, err, "failed to create new GPFile")
	defer gpf.Delete()

	timestamp := time.Now()
	require.Nil(t, gpf.writeBlock(timestamp.Unix(), []byte{1, 2, 3, 4}), "failed to write block")
	require.Nil(t, gpf.validateBlocks(1), "failed to validate block")
	require.Nil(t, gpf.Close(), "failed to close test file")
}

func TestRoundtrip(t *testing.T) {
	for _, enc := range testEncoders {
		testRoundtrip(t, enc)
	}
}

func testRoundtrip(t *testing.T, encType encoders.Type) {

	m := newMetadata()

	enc, err := encoder.New(encType)
	require.Nilf(t, err, "failed to create encoder of type %s", encType)

	gpf, err := New(testFilePath, m.BlockMetadata[0], ModeWrite, WithEncoder(enc))
	require.Nil(t, err, "failed to create new GPFile")
	defer gpf.Delete()

	for i := 0; i < 1001; i++ {

		data := []byte{}
		if i != 1000 {
			data = make([]byte, 8)
			binary.BigEndian.PutUint64(data, uint64(i))
		}

		require.Nil(t, gpf.writeBlock(int64(i), data), "failed to write block")
		require.Nilf(t, gpf.validateBlocks(i+1), "failed to validate block %d", i)
	}
	require.Nil(t, gpf.Close(), "failed to close test file")

	gpf, err = New(testFilePath, m.BlockMetadata[0], ModeRead)
	require.Nil(t, err, "failed to read GPFile")
	require.Nil(t, gpf.validateBlocks(1001), "failed to validate block")

	blocks, err := gpf.Blocks()
	require.Nil(t, err, "failed to get blocks")

	// Read ordered
	for i, block := range blocks.Blocks() {
		require.Equalf(t, block.Timestamp, int64(i), "unexpected timestamp at block %d: %d", i, block.Timestamp)
		require.Equalf(t, block.Timestamp, int64(i), "unexpected timestamp at block %d: %d", i, block.Timestamp)
		if block.Len > 0 && block.EncoderType != encType && block.EncoderType != encoders.EncoderTypeNull {
			t.Fatalf("unexpected encoder at block %d: %v", i, gpf.defaultEncoderType)
		}

		blockData, err := gpf.ReadBlock(block.Timestamp)
		require.Nilf(t, err, "failed to read block %d", i)

		expectedData := []byte{}
		if i != 1000 {
			expectedData = make([]byte, 8)
			binary.BigEndian.PutUint64(expectedData, uint64(i))
		}
		require.Equalf(t, blockData, expectedData, "unexpected data at block %d", i)
	}

	// Read from loookup map
	for _, blockItem := range blocks.Blocks() {
		block, found := blocks.BlockAtTime(blockItem.Timestamp)
		require.Truef(t, found, "missing block for timestamp %d in lookup map", blockItem.Timestamp)
		if block.Len > 0 && block.EncoderType != encType && block.EncoderType != encoders.EncoderTypeNull {
			t.Fatalf("Unexpected encoder at block %d: %v (want %v)", blockItem.Timestamp, block.EncoderType, enc)
		}

		blockData, err := gpf.ReadBlock(blockItem.Timestamp)
		require.Nilf(t, err, "failed to read block at timestamp %v", blockItem.Timestamp)

		expectedData := []byte{}
		if blockItem.Timestamp != 1000 {
			expectedData = make([]byte, 8)
			binary.BigEndian.PutUint64(expectedData, uint64(blockItem.Timestamp))
		}
		require.Equalf(t, blockData, expectedData, "unexpected data at block timetamp %v", blockItem.Timestamp)
	}

	require.Error(t, gpf.open(ModeRead), "expected error trying to re-open already open file, got none")
	require.Nil(t, gpf.Close(), "failed to close test file")
}

func TestInvalidMetadata(t *testing.T) {

	require.Nil(t, os.RemoveAll("/tmp/test_db"))
	require.Nil(t, os.MkdirAll("/tmp/test_db/1970/01/0", 0755), "error creating test dir for reading")
	require.Nil(t, os.WriteFile("/tmp/test_db/1970/01/0/.blockmeta", []byte{0x1}, 0644), "error creating test metdadata for reading")

	testDir := NewDir("/tmp/test_db", 1000, ModeRead)
	require.ErrorIs(t, testDir.Open(), ErrInputSizeTooSmall)
}

func TestEmptyMetadata(t *testing.T) {

	require.Nil(t, os.RemoveAll("/tmp/test_db"))

	testDir := NewDir("/tmp/test_db", 1000, ModeWrite)
	require.Nil(t, testDir.Open(), "error opening test dir for writing")
	require.Nil(t, testDir.Close(), "error writing test dir")

	testDir = NewDir("/tmp/test_db", 1000, ModeRead)
	require.Nil(t, testDir.Open(), "error opening test dir for reading")

	for i := 0; i < int(types.ColIdxCount); i++ {
		require.Equalf(t, len(testDir.Metadata.BlockMetadata[i].BlockList), 0, "BlockList for column %d should be empty", i)
	}

	require.Equal(t, len(testDir.Metadata.BlockTraffic), 0, "BlockTraffic should be empty")
}

func TestMetadataRoundTrip(t *testing.T) {

	require.Nil(t, os.RemoveAll("/tmp/test_db"))

	testDir := NewDir("/tmp/test_db", 1000, ModeWrite)
	require.Nil(t, testDir.Open(), "error opening test dir for writing")

	for i := 0; i < int(types.ColIdxCount); i++ {
		testDir.BlockMetadata[i].AddBlock(1575244800, storage.Block{
			Offset:      0,
			Len:         10001,
			RawLen:      100,
			EncoderType: 0,
		})
		testDir.BlockMetadata[i].AddBlock(1575245000, storage.Block{
			Offset:      10001,
			Len:         100,
			RawLen:      74,
			EncoderType: 0,
		})
		testDir.BlockMetadata[i].AddBlock(1575245500, storage.Block{
			Offset:      10101,
			Len:         10,
			RawLen:      5,
			EncoderType: 0,
		})
	}
	testDir.BlockTraffic = append(testDir.BlockTraffic, TrafficMetadata{
		NumV4Entries: 10,
		NumV6Entries: 5,
		NumDrops:     0,
	})
	testDir.BlockTraffic = append(testDir.BlockTraffic, TrafficMetadata{
		NumV4Entries: 0,
		NumV6Entries: 30,
		NumDrops:     1,
	})
	testDir.BlockTraffic = append(testDir.BlockTraffic, TrafficMetadata{
		NumV4Entries: 3,
		NumV6Entries: 3,
		NumDrops:     10000,
	})
	for _, blockTraffic := range testDir.BlockTraffic {
		testDir.Metadata.Traffic = testDir.Metadata.Traffic.Add(blockTraffic)
	}

	// Need to jump through hoops here in order to create a real deep copy of the metadata
	buf := bytes.NewBuffer(nil)
	require.Nil(t, jsoniter.NewEncoder(buf).Encode(testDir.Metadata), "error encoding reference data for later comparison")
	var refMetadata Metadata
	require.Nil(t, jsoniter.NewDecoder(buf).Decode(&refMetadata), "error decoding reference data for later comparison")
	require.Nil(t, testDir.Close(), "error writing test dir")

	testDir = NewDir("/tmp/test_db", 1000, ModeRead)
	require.Nil(t, testDir.Open(), "error opening test dir for reading")

	require.Equal(t, testDir.Metadata.BlockTraffic, refMetadata.BlockTraffic, "mismatched global block metadata")
	for i := 0; i < int(types.ColIdxCount); i++ {
		require.Equal(t, testDir.Metadata.BlockMetadata[i], refMetadata.BlockMetadata[i], "mismatched block metadata")
	}

	var (
		sumNumV4Entries, sumNumV6Entries, sumDrops int
	)
	for i := 0; i < testDir.NBlocks(); i++ {
		sumNumV4Entries += int(testDir.BlockTraffic[i].NumV4Entries)
		sumNumV6Entries += int(testDir.BlockTraffic[i].NumV6Entries)
		sumDrops += int(testDir.BlockTraffic[i].NumDrops)
	}
	require.Equal(t, sumNumV4Entries, int(testDir.Metadata.Traffic.NumV4Entries), "mismatched number of total IPv4 entries vs. computed")
	require.Equal(t, sumNumV6Entries, int(testDir.Metadata.Traffic.NumV6Entries), "mismatched number of total IPv6 entries vs. computed")
	require.Equal(t, sumDrops, int(testDir.Metadata.Traffic.NumDrops), "mismatched number of total packet drops vs. computed")
}

func TestBrokenWrite(t *testing.T) {

	require.Nil(t, os.RemoveAll("/tmp/test_db"))

	// Write some blocks and flush the data to disk
	testDir := NewDir("/tmp/test_db", 1000, ModeWrite)
	require.Nil(t, testDir.Open(), "error opening test dir for writing")
	require.Nil(t, writeDummyBlock(1, testDir, 1), "failed to write blocks")
	require.Nil(t, writeDummyBlock(2, testDir, 2), "failed to write blocks")
	expectedOffset := testDir.BlockMetadata[0].CurrentOffset
	require.Nil(t, testDir.Close(), "error writing test dir")

	// Append another block, flush the GPFiles but "fail" to write the metadata
	testDir = NewDir("/tmp/test_db", 1000, ModeWrite)
	require.Nil(t, testDir.Open(), "error opening test dir for writing")
	require.Nil(t, writeDummyBlock(3, testDir, 3), "failed to write blocks")
	require.NotEqual(t, expectedOffset, testDir.BlockMetadata[0].CurrentOffset)
	for i := 0; i < int(types.ColIdxCount); i++ {
		if testDir.gpFiles[i] != nil {
			require.Nil(t, testDir.gpFiles[i].Close())
		}
	}

	// Read the directory and validate that we only "see" two blocks on metadata level
	testDir = NewDir("/tmp/test_db", 1000, ModeRead)
	require.Nil(t, testDir.Open(), "error opening test dir for reading")
	require.Equal(t, expectedOffset, testDir.BlockMetadata[0].CurrentOffset)
	require.Equal(t, testDir.BlockMetadata[0].NBlocks(), 2)
	require.Nil(t, testDir.Close(), "error closing test dir")

	// Append another two blocks and write normally
	testDir = NewDir("/tmp/test_db", 1000, ModeWrite)
	require.Nil(t, testDir.Open(), "error opening test dir for writing")
	require.Equal(t, expectedOffset, testDir.BlockMetadata[0].CurrentOffset)
	require.Nil(t, writeDummyBlock(4, testDir, 4), "failed to write blocks")
	require.Nil(t, writeDummyBlock(5, testDir, 5), "failed to write blocks")
	expectedOffset = testDir.BlockMetadata[0].CurrentOffset
	require.Equal(t, testDir.BlockMetadata[0].NBlocks(), 4)
	require.Nil(t, testDir.Close(), "error writing test dir")

	// Read the directory and validate that we only "see" four blocks on metadata level
	testDir = NewDir("/tmp/test_db", 1000, ModeRead)
	require.Nil(t, testDir.Open(), "error opening test dir for reading")
	require.Equal(t, expectedOffset, testDir.BlockMetadata[0].CurrentOffset)
	require.Equal(t, testDir.BlockMetadata[0].NBlocks(), 4)
	for i := types.ColumnIndex(0); i < types.ColIdxCount; i++ {
		for j := 0; j < testDir.BlockMetadata[0].NBlocks(); j++ {
			data, err := testDir.ReadBlockAtIndex(i, j)
			require.Nilf(t, err, "error fetching block data at index %d, block %d", i, j)
			require.Equalf(t, int64(data[0]), testDir.BlockMetadata[i].Blocks()[j].Timestamp, "unexpected block data at index %d / block %d, want %d, have %d", i, j, testDir.BlockMetadata[i].Blocks()[j].Timestamp, data[0])
		}
	}
	require.Nil(t, testDir.Close(), "error closing test dir")
}

func TestDailyDirectoryGeneration(t *testing.T) {
	for year := 1970; year < 2200; year++ {
		for month := time.January; month <= time.December; month++ {
			for day := 1; day <= time.Date(year, month, 0, 0, 0, 0, 0, time.UTC).Day(); day++ {
				require.Equal(t, time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix(), DirTimestamp(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix()))
				require.Equal(t, time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix(), DirTimestamp(time.Date(year, month, day, 1, 0, 0, 0, time.UTC).Unix()))
				require.Equal(t, time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix(), DirTimestamp(time.Date(year, month, day, 23, 59, 59, 999999999, time.UTC).Unix()))
			}
		}
	}
}

func TestDailyDirectoryPathLayers(t *testing.T) {
	for year := 1970; year < 2200; year++ {
		for month := time.January; month <= time.December; month++ {
			for day := 1; day <= time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day(); day++ {
				gpDir := NewDir("/tmp/test_db", time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix(), ModeRead)
				require.Equal(t, gpDir.dirPath, filepath.Join("/tmp/test_db", fmt.Sprintf("%d", year), fmt.Sprintf("%02d", month), fmt.Sprintf("%d", DirTimestamp(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix()))))
			}
		}
	}
}

func (g *GPFile) validateBlocks(nExpected int) error {
	blocks, err := g.Blocks()
	if err != nil {
		return fmt.Errorf("failed to get blocks: %w", err)
	}
	if len(blocks.Blocks()) != nExpected {
		return fmt.Errorf("unexpected number of blocks, want %d, have %d", nExpected, len(blocks.Blocks()))
	}

	return nil
}

func writeDummyBlock(timestamp int64, dir *GPDir, dummyByte byte) error {
	return dir.WriteBlocks(timestamp, TrafficMetadata{
		NumV4Entries: uint64(dummyByte),
		NumV6Entries: uint64(dummyByte),
		NumDrops:     int(dummyByte),
	}, types.Counters{
		BytesRcvd:   uint64(dummyByte),
		BytesSent:   uint64(dummyByte),
		PacketsRcvd: uint64(dummyByte),
		PacketsSent: uint64(dummyByte),
	}, [types.ColIdxCount][]byte{{dummyByte}, {dummyByte}, {dummyByte}, {dummyByte}, {dummyByte}, {dummyByte}, {dummyByte}, {dummyByte}})
}
