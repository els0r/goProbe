package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/sirupsen/logrus"
)

func main() {

	var (
		dbPath     string
		dryRun     bool
		recompress bool
		nWorkers   int
		wg         sync.WaitGroup
	)
	flag.StringVar(&dbPath, "path", "", "Path to legacy goDB")
	flag.BoolVar(&dryRun, "dry-run", true, "Perform a dry-run")
	flag.BoolVar(&recompress, "recompress", false, "Convert lz4custom into lz4")
	flag.IntVar(&nWorkers, "n", 4, "Number of parallel conversion workers")
	flag.Parse()

	if dbPath == "" {
		logrus.StandardLogger().Fatal("Path to legacy goDB requried")
	}

	workChan := make(chan string, 64)
	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go func() {
			for file := range workChan {
				if err := convert(file, dryRun, recompress); err != nil {
					logrus.StandardLogger().Fatalf("Error converting legacy file %s: %s", file, err)
				}
				logrus.StandardLogger().Infof("Converted legacy file %s", file)
			}
			wg.Done()
		}()
	}

	// Get all interfaces
	dirents, err := ioutil.ReadDir(dbPath)
	if err != nil {
		logrus.StandardLogger().Fatal(err)
	}
	for _, dirent := range dirents {
		if !dirent.IsDir() {
			continue
		}

		// Get all date directories (usually days)
		dates, err := ioutil.ReadDir(filepath.Join(dbPath, dirent.Name()))
		if err != nil {
			logrus.StandardLogger().Fatal(err)
		}
		for _, date := range dates {
			if !date.IsDir() {
				continue
			}

			// Get all files in date directory
			files, err := ioutil.ReadDir(filepath.Join(dbPath, dirent.Name(), date.Name()))
			if err != nil {
				logrus.StandardLogger().Fatal(err)
			}
			for _, file := range files {
				fullPath := filepath.Join(dbPath, dirent.Name(), date.Name(), file.Name())
				if filepath.Ext(strings.TrimSpace(fullPath)) != ".gpf" {
					continue
				}

				// Check if the expected header file already exists (and skip, if so)
				if _, err := os.Stat(fullPath + gpfile.HeaderFileSuffix); err == nil {
					if !recompress {
						logrus.StandardLogger().Infof("File %s already converted, skipping...", fullPath)
						continue
					}
				}

				workChan <- fullPath
			}
		}
	}
	close(workChan)
	wg.Wait()
}

func convert(path string, dryRun, recompress bool) error {
	if !recompress {
		return convertLegacy(path, dryRun)
	}
	return recompressFile(path, dryRun)
}

func recompressFile(path string, dryRun bool) error {
	oldFile, err := gpfile.New(path, gpfile.ModeRead)
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	os.Remove(tmpPath)
	os.Remove(tmpPath + gpfile.HeaderFileSuffix)

	newFile, err := gpfile.New(tmpPath, gpfile.ModeWrite)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + gpfile.HeaderFileSuffix)

	blocks, err := oldFile.Blocks()
	if err != nil {
		return fmt.Errorf("failed to access metadata: %w", err)
	}

	overallSizeWritten := 0
	for _, block := range blocks.OrderedList() {
		logger := logrus.StandardLogger().WithFields(
			logrus.Fields{
				"timestamp": block.Timestamp,
				"encoder":   block.EncoderType,
			},
		)
		origData, err := oldFile.ReadBlock(block.Timestamp)
		if err != nil {
			logger.Errorf("failed to read block: %v", err)
			continue
		}
		if block.EncoderType == encoders.EncoderTypeLZ4Custom {
			logger.Debug("found lz4 custom block. Converting")
		}
		err = newFile.WriteBlock(block.Timestamp, origData)
		if err != nil {
			return fmt.Errorf("failed to recompress block at time %d: %w", block.Timestamp, err)
		}
		overallSizeWritten += len(origData)
	}
	if err := newFile.Close(); err != nil {
		return err
	}

	if !dryRun {
		if err := os.Rename(tmpPath+gpfile.HeaderFileSuffix, path+gpfile.HeaderFileSuffix); err != nil {
			return err
		}
		if overallSizeWritten > 0 {
			if err := os.Rename(tmpPath, path); err != nil {
				return err
			}
		} else {
			logrus.StandardLogger().Infof("No flow data detected in file %s, no need to write data file (only header was written), removing legacy data file\n", path)
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func convertLegacy(path string, dryRun bool) error {

	// Open the legacy file
	legacyFile, err := NewLegacyGPFile(path)
	if err != nil {
		return err
	}
	defer legacyFile.Close()

	tmpPath := path + ".tmp"
	os.Remove(tmpPath)
	os.Remove(tmpPath + gpfile.HeaderFileSuffix)

	newFile, err := gpfile.New(tmpPath, gpfile.ModeWrite)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + gpfile.HeaderFileSuffix)

	timestamps := legacyFile.GetTimestamps()
	overallSizeWritten := 0
	for _, ts := range timestamps {
		if ts == 0 {
			continue
		}
		block, err := legacyFile.ReadTimedBlock(ts)
		if err != nil {
			if err.Error() == "Incorrect number of bytes read for decompression" || strings.HasPrefix(err.Error(), "Invalid LZ4 data detected during decompression") {
				logrus.StandardLogger().Warnf("%s for legacy file %s, skipping block for timestamp %v", err, path, time.Unix(ts, 0))
				continue
			}
			return err
		}

		// Cut off the now unneccessary block prefix / suffix
		block = block[8 : len(block)-8]

		if err := newFile.WriteBlock(ts, block); err != nil {
			return err
		}
		overallSizeWritten += len(block)
	}

	if err := newFile.Close(); err != nil {
		return err
	}

	if !dryRun {
		if err := os.Rename(tmpPath+gpfile.HeaderFileSuffix, path+gpfile.HeaderFileSuffix); err != nil {
			return err
		}
		if overallSizeWritten > 0 {
			if err := os.Rename(tmpPath, path); err != nil {
				return err
			}
		} else {
			logrus.StandardLogger().Infof("No flow data detected in file %s, no need to write data file (only header was written), removing legacy data file\n", path)
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}

	return nil
}
