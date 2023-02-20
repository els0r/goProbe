package gpfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
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

	// Read in random order
	for ts, block := range blocks.Blocks {
		if block.Len > 0 && block.EncoderType != enc && block.EncoderType != encoders.EncoderTypeNull {
			t.Fatalf("Unexpected encoder at block %d: %v (want %v)", ts, block.EncoderType, enc)
		}

		blockData, err := gpf.ReadBlock(ts)
		if err != nil {
			t.Fatalf("Failed to read block %d: %s", ts, err)
		}

		expectedData := []byte{}
		if ts != 1000 {
			expectedData = make([]byte, 8)
			binary.BigEndian.PutUint64(expectedData, uint64(ts))
		}
		if !bytes.Equal(blockData, expectedData) {
			t.Fatalf("Unexpected data at block %d: %v, want %v", ts, blockData, expectedData)
		}
	}

	// Read ordered
	for i, block := range blocks.OrderedList() {
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
	if err := gpf.open(ModeRead); err == nil {
		t.Fatalf("Expected error trying to re-open already open file, got none")
	}

	if err := gpf.Close(); err != nil {
		t.Fatalf("Failed to close test file: %s", err)
	}
}

func (g *GPFile) validateBlocks(nExpected int) error {
	blocks, err := g.Blocks()
	if err != nil {
		return fmt.Errorf("Failed to get blocks: %w", err)
	}
	if len(blocks.Blocks) != nExpected {
		return fmt.Errorf("Unexpected number of blocks, want %d, have %d", nExpected, len(blocks.Blocks))
	}
	if len(blocks.OrderedList()) != nExpected {
		return fmt.Errorf("Unexpected number of ordered block list, want %d, have %d", nExpected, len(blocks.OrderedList()))
	}
	if blocks.Version != headerVersion {
		return fmt.Errorf("Unexpected header version, want %d, have %d", headerVersion, blocks.Version)
	}

	return nil
}
