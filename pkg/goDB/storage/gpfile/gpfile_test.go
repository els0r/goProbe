package gpfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage"
	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
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
	if err == nil {
		t.Fatalf("Expected an error trying to open a non-existing GPFile for reading, got none")
	}
}

func TestCreateFile(t *testing.T) {
	gpf, err := New(testFilePath, newMetadata().BlockMetadata[0], ModeWrite)
	if err != nil {
		t.Fatalf("Failed to create new GPFile: %s", err)
	}
	defer gpf.Delete()

	if err := gpf.validateBlocks(0); err != nil {
		t.Fatalf("Failed to validate blocks: %s", err)
	}

	if err := gpf.Close(); err != nil {
		t.Fatalf("Failed to close test file: %s", err)
	}
}

func TestWriteFile(t *testing.T) {
	gpf, err := New(testFilePath, newMetadata().BlockMetadata[0], ModeWrite)
	if err != nil {
		t.Fatalf("Failed to create new GPFile: %s", err)
	}
	defer gpf.Delete()

	timestamp := time.Now()
	if err := gpf.writeBlock(timestamp.Unix(), []byte{1, 2, 3, 4}); err != nil {
		t.Fatalf("Failed to write block: %s", err)
	}

	if err := gpf.validateBlocks(1); err != nil {
		t.Fatalf("Failed to validate blocks: %s", err)
	}

	if err := gpf.Close(); err != nil {
		t.Fatalf("Failed to close test file: %s", err)
	}
}

func TestRoundtrip(t *testing.T) {
	for _, enc := range testEncoders {
		testRoundtrip(t, enc)
	}
}

func testRoundtrip(t *testing.T, enc encoders.Type) {

	m := newMetadata()

	gpf, err := New(testFilePath, m.BlockMetadata[0], ModeWrite, WithEncoder(enc))
	if err != nil {
		t.Fatalf("Failed to create new GPFile: %s", err)
	}
	defer gpf.Delete()

	for i := 0; i < 1001; i++ {

		data := []byte{}
		if i != 1000 {
			data = make([]byte, 8)
			binary.BigEndian.PutUint64(data, uint64(i))
		}

		if err := gpf.writeBlock(int64(i), data); err != nil {
			t.Fatalf("Failed to write block: %s", err)
		}

		if err := gpf.validateBlocks(i + 1); err != nil {
			t.Fatalf("Failed to validate blocks: %s", err)
		}
	}
	if err := gpf.Close(); err != nil {
		t.Fatalf("Failed to close test file: %s", err)
	}

	gpf, err = New(testFilePath, m.BlockMetadata[0], ModeRead)
	if err != nil {
		t.Fatalf("Failed to read GPFile: %s", err)
	}
	if err := gpf.validateBlocks(1001); err != nil {
		t.Fatalf("Failed to validate blocks: %s", err)
	}
	blocks, err := gpf.Blocks()
	if err != nil {
		t.Fatalf("Failed to get blocks: %s", err)
	}

	// Read ordered
	for i, block := range blocks.Blocks() {
		if block.Timestamp != int64(i) {
			t.Fatalf("Unexpected timestamp at block %d: %d", i, block.Timestamp)
		}
		if block.Len > 0 && block.EncoderType != enc && block.EncoderType != encoders.EncoderTypeNull {
			t.Fatalf("Unexpected encoder at block %d: %v", i, gpf.defaultEncoderType)
		}

		blockData, err := gpf.ReadBlock(block.Timestamp)
		if err != nil {
			t.Fatalf("Failed to read block %d: %s", i, err)
		}

		expectedData := []byte{}
		if i != 1000 {
			expectedData = make([]byte, 8)
			binary.BigEndian.PutUint64(expectedData, uint64(i))
		}
		if !bytes.Equal(blockData, expectedData) {
			t.Fatalf("Unexpected data at block %d: %v", i, blockData)
		}
	}

	// Read from loookup map
	for _, blockItem := range blocks.Blocks() {
		block, found := blocks.BlockAtTime(blockItem.Timestamp)
		if !found {
			t.Fatalf("Missing block for timestamp %d in lookup map", blockItem.Timestamp)
		}

		if block.Len > 0 && block.EncoderType != enc && block.EncoderType != encoders.EncoderTypeNull {
			t.Fatalf("Unexpected encoder at block %d: %v (want %v)", blockItem.Timestamp, block.EncoderType, enc)
		}

		blockData, err := gpf.ReadBlock(blockItem.Timestamp)
		if err != nil {
			t.Fatalf("Failed to read block %d: %s", blockItem.Timestamp, err)
		}

		expectedData := []byte{}
		if blockItem.Timestamp != 1000 {
			expectedData = make([]byte, 8)
			binary.BigEndian.PutUint64(expectedData, uint64(blockItem.Timestamp))
		}
		if !bytes.Equal(blockData, expectedData) {
			t.Fatalf("Unexpected data at block %d: %v, want %v", blockItem.Timestamp, blockData, expectedData)
		}
	}

	if err := gpf.open(ModeRead); err == nil {
		t.Fatalf("Expected error trying to re-open already open file, got none")
	}

	if err := gpf.Close(); err != nil {
		t.Fatalf("Failed to close test file: %s", err)
	}
}

func TestInvalidMetadata(t *testing.T) {

	os.RemoveAll("/tmp/test_db")
	if err := os.MkdirAll("/tmp/test_db/0", 0755); err != nil {
		t.Fatalf("Error creating test dir for reading: %s", err)
	}
	if err := os.WriteFile("/tmp/test_db/0/.blockmeta", []byte{0x1}, 0644); err != nil {
		t.Fatalf("Error creating test metdadata for reading: %s", err)
	}

	testDir := NewDir("/tmp/test_db", 1000, ModeRead)
	if err := testDir.Open(); err == nil || err.Error() != "error decoding metadata file `/tmp/test_db/0/.blockmeta`: input data too small to be a GPDir metadata header (len: 1)" {
		t.Fatalf("Expected error `input data too small to be a GPDir metadata header (len: 1)` opening corrupt GPDir, got `%s`", err)
	}
}

func TestEmptyMetadata(t *testing.T) {

	os.RemoveAll("/tmp/test_db")

	testDir := NewDir("/tmp/test_db", 1000, ModeWrite)
	if err := testDir.Open(); err != nil {
		t.Fatalf("Error opening test dir for writing: %s", err)
	}

	if err := testDir.Close(); err != nil {
		t.Fatalf("Error writing test dir: %s", err)
	}

	testDir = NewDir("/tmp/test_db", 1000, ModeRead)
	if err := testDir.Open(); err != nil {
		t.Fatalf("Error opening test dir for writing: %s", err)
	}

	for i := 0; i < int(types.ColIdxCount); i++ {
		if len(testDir.Metadata.BlockMetadata[i].BlockList) != 0 {
			t.Fatalf("BlockList for column %d should be empty", i)
		}
	}

	if len(testDir.Metadata.BlockTraffic) != 0 {
		t.Fatalf("BlockTraffic should be empty")
	}
}

func TestMetadataRoundTrip(t *testing.T) {

	os.RemoveAll("/tmp/test_db")

	testDir := NewDir("/tmp/test_db", 1000, ModeWrite)
	if err := testDir.Open(); err != nil {
		t.Fatalf("Error opening test dir for writing: %s", err)
	}

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
	if err := jsoniter.NewEncoder(buf).Encode(testDir.Metadata); err != nil {
		t.Fatalf("Error encoding reference data for later comparison")
	}
	var refMetadata Metadata
	if err := jsoniter.NewDecoder(buf).Decode(&refMetadata); err != nil {
		t.Fatalf("Error decoding reference data for later comparison")
	}

	if err := testDir.Close(); err != nil {
		t.Fatalf("Error writing test dir: %s", err)
	}

	testDir = NewDir("/tmp/test_db", 1000, ModeRead)
	if err := testDir.Open(); err != nil {
		t.Fatalf("Error opening test dir for writing: %s", err)
	}

	if !reflect.DeepEqual(testDir.Metadata.BlockTraffic, refMetadata.BlockTraffic) {
		t.Fatalf("Mismatched global block metadata: %#v vs %#v", testDir.Metadata.BlockTraffic, refMetadata.BlockTraffic)
	}
	for i := 0; i < int(types.ColIdxCount); i++ {
		if !reflect.DeepEqual(testDir.Metadata.BlockMetadata[i], refMetadata.BlockMetadata[i]) {
			t.Fatalf("Mismatched metadata: %#v vs %#v", testDir.Metadata.BlockMetadata[i], refMetadata.BlockMetadata[i])
		}
	}

	var (
		sumNumV4Entries, sumNumV6Entries, sumDrops int
	)
	for i := 0; i < testDir.NBlocks(); i++ {
		sumNumV4Entries += int(testDir.BlockTraffic[i].NumV4Entries)
		sumNumV6Entries += int(testDir.BlockTraffic[i].NumV6Entries)
		sumDrops += int(testDir.BlockTraffic[i].NumDrops)
	}
	if sumNumV4Entries != int(testDir.Metadata.Traffic.NumV4Entries) {
		t.Fatalf("Mismatched number of total IPv4 entries vs. computed (%d vs %d)", testDir.Metadata.Traffic.NumV4Entries, sumNumV4Entries)
	}
	if sumNumV6Entries != int(testDir.Metadata.Traffic.NumV6Entries) {
		t.Fatalf("Mismatched number of total IPv6 entries vs. computed (%d vs %d)", testDir.Metadata.Traffic.NumV6Entries, sumNumV6Entries)
	}
	if sumDrops != int(testDir.Metadata.Traffic.NumDrops) {
		t.Fatalf("Mismatched number of total packet drops vs. computed (%d vs %d)", testDir.Metadata.Traffic.NumDrops, sumDrops)
	}
}

func (g *GPFile) validateBlocks(nExpected int) error {
	blocks, err := g.Blocks()
	if err != nil {
		return fmt.Errorf("Failed to get blocks: %w", err)
	}
	if len(blocks.Blocks()) != nExpected {
		return fmt.Errorf("Unexpected number of blocks, want %d, have %d", nExpected, len(blocks.Blocks()))
	}

	return nil
}
