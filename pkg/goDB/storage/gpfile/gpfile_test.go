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
	if err := gpf.WriteBlock(timestamp.Unix(), []byte{1, 2, 3, 4}); err != nil {
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

		if err := gpf.WriteBlock(int64(i), data); err != nil {
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
	testDir.BlockNumV4Entries = append(testDir.BlockNumV4Entries, 0)
	testDir.BlockNumV4Entries = append(testDir.BlockNumV4Entries, 10)
	testDir.BlockNumV4Entries = append(testDir.BlockNumV4Entries, 24)

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

	if !reflect.DeepEqual(testDir.Metadata.BlockNumV4Entries, refMetadata.BlockNumV4Entries) {
		t.Fatalf("Mismatched metadata: %#v vs %#v", testDir.Metadata.BlockNumV4Entries, refMetadata.BlockNumV4Entries)
	}
	for i := 0; i < int(types.ColIdxCount); i++ {
		if !reflect.DeepEqual(testDir.Metadata.BlockMetadata[i], refMetadata.BlockMetadata[i]) {
			t.Fatalf("Mismatched metadata: %#v vs %#v", testDir.Metadata.BlockMetadata[i], refMetadata.BlockMetadata[i])
		}
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
