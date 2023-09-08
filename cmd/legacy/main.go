package main

import (
	"flag"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/els0r/telemetry/logging"
)

type work struct {
	iface string
	path  string
}

type converter struct {
	dbDir            string
	dbPermissions    fs.FileMode
	compressionLevel int
	pipe             chan work
}

var logger *logging.L

func main() {

	var (
		inPath, outPath  string
		profilePath      string
		dryRun, debug    bool
		nWorkers         int
		compressionLevel int
		dbPermissionsStr string
		wg               sync.WaitGroup
	)
	flag.StringVar(&inPath, "i", "", "Path to (legacy) input goDB")
	flag.StringVar(&outPath, "o", "", "Path to output goDB")
	flag.StringVar(&profilePath, "profile", "", "Path to (optional) output CPU profile")
	flag.BoolVar(&dryRun, "dry-run", true, "Perform a dry-run")
	flag.StringVar(&dbPermissionsStr, "p", fmt.Sprintf("%o", goDB.DefaultPermissions), "Permissions to use when writing files to DB (UNIX octal file mode)")
	flag.IntVar(&nWorkers, "n", runtime.NumCPU()/2, "Number of parallel conversion workers")
	flag.IntVar(&compressionLevel, "l", 0, "Custom LZ4 compression level (uses internal default if <= 0)")
	flag.BoolVar(&debug, "debug", false, "Enable debug / verbose mode")
	flag.Parse()

	logLevel := logging.LevelInfo
	if debug {
		logLevel = logging.LevelDebug
	}
	err := logging.Init(logLevel,
		logging.EncodingLogfmt,
		logging.WithVersion(version.Short()),
	)
	if err != nil {
		fmt.Printf("failed to instantiate logger: %s\n", err)
		os.Exit(1)
	}
	logger = logging.Logger()

	if inPath == "" || outPath == "" {
		logger.Fatal("Paths to input & output goDB requried")
	}

	if profilePath != "" {
		f, err := os.Create(filepath.Clean(profilePath))
		if err != nil {
			logger.Fatalf("failed to create CPU profile file: %s", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			logger.Fatalf("failed to start CPU profiling: %s", err)
		}
		defer pprof.StopCPUProfile()
	}
	dbPermissions, err := strconv.ParseUint(dbPermissionsStr, 8, 32)
	if err != nil {
		logger.Fatalf("failed to parse file permissions: %s", err)
	}

	c := converter{
		dbDir:            outPath,
		dbPermissions:    fs.FileMode(dbPermissions),
		compressionLevel: compressionLevel,
		pipe:             make(chan work, nWorkers*4),
	}

	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go func() {
			for w := range c.pipe {
				if err := c.convertDir(w, dryRun); err != nil {
					logger.Fatalf("error converting legacy dir %s: %s", w.path, err)
				}
				logger.Debugf("successfully converted legacy dir %s", w.path)
			}
			wg.Done()
		}()
	}

	// Get all interfaces
	ifaces, err := os.ReadDir(inPath)
	if err != nil {
		logger.Fatal(err.Error())
	}
	for _, iface := range ifaces {
		if !iface.IsDir() {
			continue
		}

		// Get all date directories (usually days)
		dates, err := os.ReadDir(filepath.Join(inPath, iface.Name()))
		if err != nil {
			logger.Fatal(err.Error())
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
	data  *hashmap.AggFlowMap
}

type fileSet interface {
	GetTimestamps() ([]int64, error)
	GetBlock(ts int64) (*hashmap.AggFlowMap, error)
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
	writer := goDB.NewDBWriter(c.dbDir, w.iface, encoders.EncoderTypeLZ4).Permissions(c.dbPermissions).EncoderLevel(c.compressionLevel)

	var bulkWorkload []goDB.BulkWorkload
	for _, block := range allBlocks {
		blockMetadata, err := metadata.GetBlock(block.ts)
		if err != nil {
			return fmt.Errorf("failed to get block metdadata from file set: %w", err)
		}

		bulkWorkload = append(bulkWorkload, goDB.BulkWorkload{
			FlowMap: block.data,
			CaptureStats: capturetypes.CaptureStats{
				Dropped: ensureUnsigned(blockMetadata.PcapPacketsDropped) + ensureUnsigned(blockMetadata.PcapPacketsIfDropped),
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

func newKeyFromNetIPAddr(sip, dip netip.Addr, dport []byte, proto byte, isIPv4 bool) types.Key {
	if isIPv4 {
		return types.NewV4KeyStatic(sip.As4(), dip.As4(), dport, proto)
	}
	return types.NewV6KeyStatic(sip.As16(), dip.As16(), dport, proto)
}

func ensureUnsigned(in int) uint64 {
	if in <= 0 {
		return 0
	}
	return uint64(in)
}
