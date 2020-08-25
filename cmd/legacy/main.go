package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
)

func main() {

	var (
		dbPath string
		dryRun bool
	)
	flag.StringVar(&dbPath, "path", "", "Path to legacy goDB")
	flag.BoolVar(&dryRun, "dry-run", true, "Perform a dry-run")
	flag.Parse()

	if dbPath == "" {
		log.Fatal("Path to legacy goDB requried")
	}

	// Get all interfaces
	dirents, err := ioutil.ReadDir(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	for _, dirent := range dirents {
		if !dirent.IsDir() {
			continue
		}

		// Get all date directories (usually days)
		dates, err := ioutil.ReadDir(filepath.Join(dbPath, dirent.Name()))
		if err != nil {
			log.Fatal(err)
		}
		for _, date := range dates {
			if !date.IsDir() {
				continue
			}

			// Get all files in date directory
			files, err := ioutil.ReadDir(filepath.Join(dbPath, dirent.Name(), date.Name()))
			if err != nil {
				log.Fatal(err)
			}
			for _, file := range files {
				fullPath := filepath.Join(dbPath, dirent.Name(), date.Name(), file.Name())
				if filepath.Ext(strings.TrimSpace(fullPath)) != ".gpf" {
					continue
				}

				// Check if the expected header file already exists (and skip, if so)
				if _, err := os.Stat(fullPath + gpfile.HeaderFileSuffix); err == nil {
					log.Println("File", fullPath, "already converted, skipping...")
					continue
				}

				if err := convert(fullPath, dryRun); err != nil {
					log.Fatalf("Error converting legacy file %s: %s", fullPath, err)
				}
				log.Println("Converted", fullPath)
			}
		}
	}

}

func convert(path string, dryRun bool) error {

	// Open the legacy file
	legacyFile, err := NewLegacyGPFile(path)
	if err != nil {
		return err
	}
	defer legacyFile.Close()

	tmpPath := path + ".tmp"
	newFile, err := gpfile.New(tmpPath, gpfile.ModeWrite)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + gpfile.HeaderFileSuffix)

	timestamps := legacyFile.GetTimestamps()
	for _, ts := range timestamps {
		if ts == 0 {
			continue
		}
		block, err := legacyFile.ReadTimedBlock(ts)
		if err != nil {
			return err
		}

		// Cut off the now unneccessary block prefix / suffix
		block = block[8 : len(block)-8]

		if err := newFile.WriteBlock(ts, block); err != nil {
			return err
		}
	}

	if err := newFile.Close(); err != nil {
		return err
	}

	if !dryRun {
		if err := os.Rename(tmpPath+gpfile.HeaderFileSuffix, path+gpfile.HeaderFileSuffix); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, path); err != nil {
			return err
		}
	}

	return nil
}
