package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"go.uber.org/zap"
)

type work struct {
	iface string
	path  string
}

type converter struct {
	dbDir string
	pipe  chan work
}

var logger *zap.SugaredLogger

func main() {

	zapLogger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("failed to instantiate logger: %s\n", err)
		os.Exit(1)
	}
	defer zapLogger.Sync()
	logger = zapLogger.Sugar()

	var (
		inPath, outPath string
		dryRun          bool
		nWorkers        int
		wg              sync.WaitGroup
	)
	flag.StringVar(&inPath, "path", "", "Path to legacy goDB")
	flag.StringVar(&outPath, "output", "", "Path to output goDB")
	flag.BoolVar(&dryRun, "dry-run", true, "Perform a dry-run")
	flag.IntVar(&nWorkers, "n", 4, "Number of parallel conversion workers")
	flag.Parse()

	if inPath == "" || outPath == "" {
		logger.Fatal("Paths to legacy / output goDB requried")
	}

	c := converter{
		dbDir: outPath,
		pipe:  make(chan work, 64),
	}

	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go func() {
			for w := range c.pipe {
				if err := c.convertDir(w, dryRun); err != nil {
					logger.Fatalf("Error converting legacy dir %s: %s", w.path, err)
				}
				logger.Infof("Converted legacy dir %s", w.path)
			}
			wg.Done()
		}()
	}

	// Get all interfaces
	ifaces, err := ioutil.ReadDir(inPath)
	if err != nil {
		logger.Fatal(err)
	}
	for _, iface := range ifaces {
		if !iface.IsDir() {
			continue
		}

		// Get all date directories (usually days)
		dates, err := ioutil.ReadDir(filepath.Join(inPath, iface.Name()))
		if err != nil {
			logger.Fatal(err)
		}
		for _, date := range dates {
			if !date.IsDir() {
				continue
			}

			c.pipe <- work{
				iface: iface.Name(),
				path:  filepath.Join(inPath, iface.Name(), date.Name()),
			}
		}
	}

	close(c.pipe)
	wg.Wait()
}

type blockFlows struct {
	ts    int64
	iface string
	data  *hashmap.Map
}

type fileSet interface {
	GetTimestamps() ([]int64, error)
	GetBlock(ts int64) (*hashmap.Map, error)
	Close() error
}

// headerFileSuffix denotes the suffix used for the legcay header data
const headerFileSuffix = ".meta"

func isLegacyDir(path string) (bool, error) {
	dirents, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}

	var countGPFs, countMeta int
	for _, dirent := range dirents {
		dname := strings.TrimSpace(strings.ToLower(dirent.Name()))
		if strings.HasSuffix(dname, gpfile.FileSuffix) {
			countGPFs++
		} else if strings.HasSuffix(dname, gpfile.FileSuffix+headerFileSuffix) {
			countMeta++
		}
	}

	return countMeta == 0 && countGPFs > 0, nil
}

func (c converter) convertDir(w work, dryRun bool) error {

	var (
		fs  fileSet
		err error
	)
	if isLegacy, err := isLegacyDir(w.path); err != nil {
		return err
	} else if isLegacy {
		fs, err = NewLegacyFileSet(w.path)
		if err != nil {
			return fmt.Errorf("failed to read legacy data set in %s: %w", w.path, err)
		}
	} else {
		fs, err = NewModernFileSet(w.path)
		if err != nil {
			return fmt.Errorf("failed to read modern data set in %s: %w", w.path, err)
		}
	}

	dirTimestamp, err := strconv.ParseInt(filepath.Base(w.path), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to get directory timestamp: %w", err)
	}

	defer func() {
		if err := fs.Close(); err != nil {
			panic(err)
		}
	}()

	var allBlocks []blockFlows
	timestamps, err := fs.GetTimestamps()
	if err != nil {
		return err
	}
	for _, ts := range timestamps {
		if ts == 0 {
			continue
		}

		flows, err := fs.GetBlock(ts)
		if err != nil {
			logger.Errorf("failed to get block from file set: %s", err)
			continue
		}

		allBlocks = append(allBlocks, blockFlows{
			ts:    ts,
			iface: w.iface,
			data:  flows,
		})
	}

	// If no blocks were read / remain (e.g. due to corruption), we can skip this directory
	if len(allBlocks) == 0 {
		return nil
	}

	// Sort by timestamp to cover potential out-of-order scenarios
	sort.Slice(allBlocks, func(i, j int) bool {
		return allBlocks[i].ts < allBlocks[j].ts
	})

	metadata, err := ReadMetadata(filepath.Join(w.path, MetadataFileName))
	if err != nil {
		return fmt.Errorf("failed to read metadata from %s: %w", filepath.Join(w.path, MetadataFileName), err)
	}
	writer := goDB.NewDBWriter(c.dbDir, w.iface, encoders.EncoderTypeLZ4)

	var bulkWorkload []goDB.BulkWorkload
	for _, block := range allBlocks {
		blockMetadata, err := metadata.GetBlock(block.ts)
		if err != nil {
			return fmt.Errorf("failed to get block metdadata from file set: %w", err)
		}

		bulkWorkload = append(bulkWorkload, goDB.BulkWorkload{
			FlowMap: block.data,
			CaptureMeta: goDB.CaptureMetadata{
				PacketsDropped: blockMetadata.PcapPacketsDropped + blockMetadata.PcapPacketsIfDropped,
			},
			Timestamp: block.ts,
		})
	}

	if !dryRun {
		if err = writer.WriteBulk(bulkWorkload, dirTimestamp); err != nil {
			return fmt.Errorf("failed to write flows: %w", err)
		}
	}

	return nil
}
