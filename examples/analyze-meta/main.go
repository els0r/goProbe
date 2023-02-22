package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/sirupsen/logrus"
)

var (
	pathMetaFile string
)

func main() {
	logger := logrus.StandardLogger()

	flag.StringVar(&pathMetaFile, "path", "", "Path to meta file")
	flag.Parse()

	pathMetaFile = filepath.Clean(pathMetaFile)
	dirPath := filepath.Dir(pathMetaFile)
	timestamp, err := strconv.ParseInt(filepath.Base(dirPath), 10, 64)
	if err != nil {
		logger.Fatalf("failed to extract timestamp: %s", err)
	}
	baseDirPath := filepath.Dir(dirPath)

	gpDir := gpfile.NewDir(baseDirPath, timestamp, gpfile.ModeRead)
	if err := gpDir.Open(); err != nil {
		logger.WithField("path", dirPath).Fatalf("failed to open GPF dir: %v", err)
	}
	defer gpDir.Close()

	for i := types.ColumnIndex(0); i < types.ColIdxCount; i++ {
		err = PrintMetaTable(gpDir, i, os.Stdout)
		if err != nil {
			logger.Fatalf("print meta table: %v", err)
		}
	}
}

func PrintMetaTable(gpDir *gpfile.GPDir, column types.ColumnIndex, w io.Writer) error {

	blockMetadata := gpDir.BlockMetadata[column]
	blocks := blockMetadata.Blocks()

	fmt.Fprintf(w, `
              Column: %s
    Number of Blocks: %d
                Size: %d bytes

`, types.ColumnFileNames[column], len(blocks) /*gpf.TypeWidth(),*/, blockMetadata.CurrentOffset)

	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', tabwriter.AlignRight)

	tFormat := "2006-01-02 15:04:05"

	sep := "\t"

	header := []string{"#", "offset (bl)", "ts", "time UTC", "length", "raw length", "encoder", "ratio %", "first 4 bytes", "", "integrity check"}
	fmtStr := sep + strings.Join([]string{"%d", "%d", "%d", "%s", "%d", "%d", "%s", "%.2f", "%x", "%s", "%s"}, sep) + sep + "\n"

	fmt.Fprintln(tw, sep+strings.Join(header, sep)+sep)
	fmt.Fprintln(tw, sep+strings.Repeat(sep, len(header))+sep)

	var curOffset int64
	var b = make([]byte, 4)
	attnMagicMismatch := " !"

	colFile, err := gpDir.Column(column)
	if err != nil {
		return fmt.Errorf("failed to access underlying GPFile for column %s: %w", types.ColumnFileNames[column], err)
	}
	for i, block := range blocks {

		// First, just attempt to read the block
		if _, err := colFile.ReadBlock(block.Timestamp); err != nil {
			return fmt.Errorf("column %d reading block %d failed: %w", column, i, err)
		}
	}

	for i, block := range blocks {

		// Access the raw data of the underlying file / buffer and validate its integrity
		_, err := colFile.RawFile().Seek(curOffset, 0)
		if err != nil {
			return fmt.Errorf("column %d seeking at block %d failed: %w", column, i, err)
		}
		n, err := colFile.RawFile().Read(b)
		if err != nil {
			return fmt.Errorf("column %d read at block %d failed: %w", column, i, err)
		}
		if n != 4 {
			return fmt.Errorf("wrong number of bytes read: %d", n)
		}

		first4Bytes := binary.BigEndian.Uint32(b)

		// specifically check block integrity when lz4 encoding is on
		var attn string
		if block.EncoderType == encoders.EncoderTypeLZ4 {
			// LZ4 magic number
			if first4Bytes != 0x04224d18 {
				attn = attnMagicMismatch
			}
		}

		var ratio float64
		if !block.IsEmpty() {
			ratio = 100 * float64(block.Len) / float64(block.RawLen)
		}
		fmt.Fprintf(tw, fmtStr, i+1,
			block.Offset,
			block.Timestamp, time.Unix(block.Timestamp, 0).UTC().Format(tFormat),
			block.Len, block.RawLen,
			block.EncoderType, ratio,
			b, attn,
			// TODO: diagnostics for lz4
			"",
		)
		curOffset += int64(block.Len)
	}
	fmt.Fprintln(tw)

	return tw.Flush()
}
