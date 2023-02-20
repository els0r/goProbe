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

	gpDir, err := gpfile.NewDir(baseDirPath, timestamp, gpfile.ModeRead)
	if err != nil {
		logger.WithField("path", dirPath).Fatalf("failed to open GPF dir: %v", err)
	}

	for i := types.ColumnIndex(0); i < types.ColIdxCount; i++ {
		gpfFile, err := gpDir.Column(i)
		if err != nil {
			logger.WithField("path", dirPath).Fatalf("failed to access GPF column file: %v", err)
		}

		err = PrintMetaTable(gpfFile, os.Stdout)
		if err != nil {
			logger.Fatalf("print meta table: %v", err)
		}
	}

}

func PrintMetaTable(gpf *gpfile.GPFile, w io.Writer) error {
	file, err := os.OpenFile(gpf.Filename(), gpfile.ModeRead, 0644)
	if err != nil {
		return fmt.Errorf("failed to open GPF file for reading: %v", err)
	}
	defer file.Close()

	blocks, err := gpf.Blocks()
	if err != nil {
		return fmt.Errorf("failed to get blocks: %w", err)
	}

	fmt.Fprintf(w, `
                File: %s
    Number of Blocks: %d
                Size: %d bytes
    Default Encoding: %s

`, gpf.Filename(), len(blocks.Blocks) /*gpf.TypeWidth(),*/, blocks.CurrentOffset, gpf.DefaultEncoder().Type())

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
	for i, block := range blocks.OrderedList() {
		n, err := file.ReadAt(b, curOffset)
		if err != nil {
			return fmt.Errorf("file read at %d failed: %w", curOffset, err)
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
