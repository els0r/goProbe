/////////////////////////////////////////////////////////////////////////////////
//
// DBConvert.go
//
// Written by Lennart Elsen lel@open.ch, July 2014
// Copyright (c) 2014 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Binary to read in database data from csv files and push it to the goDB writer
// which creates a .gpf columnar database from the data at a specified location.
package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"

	// for metrics export to metricsbeat
	_ "expvar"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/els0r/goProbe/pkg/version"

	"flag"
	"fmt"
)

// Config stores the flags provided to the converter
type Config struct {
	FilePath      string
	SavePath      string
	Iface         string
	Schema        string
	NumLines      int
	EncoderType   int
	DBPermissions uint
}

// parameter governing the number of seconds that are covered by a block
const (
	csvDefaultSchema = "time,iface,sip,dip,dport,proto,packets received,packets sent,%,data vol. received,data vol. sent,%"
)

type writeJob struct {
	iface  string
	tstamp int64
	data   *hashmap.AggFlowMap
}

type keyIndParserItem struct {
	ind    int
	parser goDB.StringKeyParser
}

// IfaceStringParser parses iface strings
type IfaceStringParser struct{}

// // ParseKey writes element to the Iface key
func (i *IfaceStringParser) ParseKey(element string, key *types.ExtendedKey) error {

	// Not very pretty: We basically just append the string and its length to the end
	ifaceBytes := []byte(element)
	newKey := make([]byte, len(*key)+len(ifaceBytes)+4)
	pos := copy(newKey, *key)
	pos += copy(newKey[pos:], ifaceBytes)
	binary.BigEndian.PutUint32(newKey[len(newKey)-4:], uint32(len(element)))

	*key = newKey

	return nil
}

func extractIface(key []byte) ([]byte, string) {

	strLen := int(binary.BigEndian.Uint32(key[len(key)-4:]))
	ifaceName := string(key[len(key)-(strLen+4) : len(key)-4])
	remainingKey := key[:len(key)-(strLen+4)]

	return remainingKey, ifaceName
}

func newStringKeyParser(field string) goDB.StringKeyParser {
	if field == "iface" {
		return &IfaceStringParser{}
	}
	return goDB.NewStringKeyParser(field)
}

// CSVConverter can read CSV files containing goProbe flow information
type CSVConverter struct {
	// map field index to how it should be parsed
	KeyParsers []keyIndParserItem
	ValParsers map[int]goDB.StringValParser
}

// NewCSVConverter initializes a CSVConverter with the Key- and Value parsers for goProbe flows
func NewCSVConverter() *CSVConverter {
	return &CSVConverter{
		KeyParsers: make([]keyIndParserItem, 0),
		ValParsers: make(map[int]goDB.StringValParser),
	}
}

func (c *CSVConverter) readSchema(schema string) error {
	logger := logging.Logger()

	fields := strings.Split(schema, ",")

	var (
		canParse  = make([]string, len(fields))
		cantParse = make([]string, len(fields))
	)

	// first try to extract all attributes which need to be parsed
	for ind, field := range fields {
		parser := newStringKeyParser(field)

		// check if a NOP parser was created. If so, try to create
		// a value parser from the field
		if _, ok := parser.(*goDB.NOPStringParser); ok {
			parser := goDB.NewStringValParser(field)

			if _, ok := parser.(*goDB.NOPStringParser); ok {
				cantParse = append(cantParse, field)
			} else {
				c.ValParsers[ind] = parser
				canParse = append(canParse, field)
			}
		} else {
			c.KeyParsers = append(c.KeyParsers, keyIndParserItem{ind, parser})
			canParse = append(canParse, field)
		}
	}

	// Ensure that IP parsers are executed first and interface parsers last (if present)
	// to ensure correct parsing
	sort.Slice(c.KeyParsers, func(i, j int) bool {
		if _, isIfaceParser := c.KeyParsers[j].parser.(*IfaceStringParser); isIfaceParser {
			return true
		}
		_, isSipParser := c.KeyParsers[i].parser.(*goDB.SipStringParser)
		_, isDipParser := c.KeyParsers[i].parser.(*goDB.DipStringParser)
		return isSipParser || isDipParser
	})

	// if only NOP parsers were created, it means that the
	// schema is fully unreadable
	if len(cantParse) == len(fields) {
		return fmt.Errorf("not a single field can be parsed in the provided schema")
	}

	// print parseable/unparseable fields:
	logger.Debugf("SCHEMA: can parse: %s. Will not parse: %s", canParse, cantParse)
	return nil
}

func (c *CSVConverter) parsesIface() bool {
	for _, p := range c.KeyParsers {
		if _, ok := p.parser.(*IfaceStringParser); ok {
			return true
		}
	}
	return false
}

func parseCommandLineArgs(cfg *Config) {
	flag.StringVar(&cfg.FilePath, "in", "", "CSV file from which the data should be read")
	flag.StringVar(&cfg.SavePath, "out", "", "Folder to which the .gpf files should be written")
	flag.StringVar(&cfg.Schema, "schema", "", "Structure of CSV file (e.g. \"sip,dip,dport,time\"")
	flag.StringVar(&cfg.Iface, "iface", "", "Interface from which CSV data was created")
	flag.IntVar(&cfg.NumLines, "n", 1000, "Number of rows to read from the CSV file")
	flag.IntVar(&cfg.EncoderType, "encoder", 0, "Encoder type to use for compression")
	flag.UintVar(&cfg.DBPermissions, "permissions", 0, "Permissions to use when writing DB (Unix file mode)")
	flag.Parse()
}

func printUsage(msg string) {
	fmt.Println(msg + ".\nUsage: ./goConvert -in <input file path> -out <output folder> [-n <number of lines to read> -schema <schema string> -iface <interface>]")
	return
}

func main() {

	// parse command line arguments
	var config Config
	parseCommandLineArgs(&config)

	// sanity check the input
	if config.FilePath == "" || config.SavePath == "" {
		printUsage("Empty path specified")
		os.Exit(1)
	}

	// get logger
	err := logging.Init("goConvert", version.Short(), "debug", "console")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to spawn logger: %s\n", err)
		os.Exit(1)
	}
	logger := logging.Logger()

	// get number of lines to read in the specified file
	cmd := exec.Command("wc", "-l", config.FilePath)
	out, cmderr := cmd.Output()
	if cmderr != nil {
		logger.Fatalf("could not obtain line count on file %s", config.FilePath)
	}

	nlString := strings.Split(string(out), " ")
	nlInFile, _ := strconv.ParseInt(nlString[0], 10, 32)
	if int(nlInFile) < config.NumLines && nlInFile > 0 {
		config.NumLines = int(nlInFile)
	}

	logger.Infof("Converting %d rows in file %s", config.NumLines, config.FilePath)

	// open file
	var file *os.File

	if file, err = os.Open(config.FilePath); err != nil {
		logger.Fatalf("file open error: %s", err)
	}

	// create a CSV converter
	var csvconv = NewCSVConverter()
	if config.Schema != "" {
		if err = csvconv.readSchema(config.Schema); err != nil {
			logger.Fatalf("failed to read schema: %s", err)
		}
	}

	// map writers. There's one for each interface
	var mapWriters = make(map[string]*goDB.DBWriter)

	// scan file line by line
	scanner := bufio.NewScanner(file)
	var (
		linesRead          = 1
		percDone, prevPerc int

		// flow map which is populated from the CSV file. This is a map of flow maps due to the fact
		// that several interfaces may be handles in a single CSV file. Thus, there is one map per
		// interface
		//
		// interface -> timestamp -> AggFlowMap
		rowKeyV4 = types.NewEmptyV4Key().ExtendEmpty()
		rowKeyV6 = types.NewEmptyV6Key().ExtendEmpty()
		flowMaps = make(map[string]map[int64]*hashmap.AggFlowMap)
	)

	// channel for passing flow maps to writer
	writeChan := make(chan writeJob, 1024)

	dbPermissions := goDB.DefaultPermissions
	if config.DBPermissions != 0 {
		dbPermissions = fs.FileMode(config.DBPermissions)
	}

	// writer routine accepting flow maps to write out
	var wg sync.WaitGroup
	wg.Add(1)
	go func(writeChan chan writeJob) {
		defer wg.Done()
		for fm := range writeChan {
			if _, ok := mapWriters[fm.iface]; !ok {
				mapWriters[fm.iface] = goDB.NewDBWriter(config.SavePath, fm.iface, encoders.Type(config.EncoderType)).Permissions(dbPermissions)
			}

			if err = mapWriters[fm.iface].Write(fm.data, goDB.CaptureMetadata{}, fm.tstamp); err != nil {
				fmt.Printf("Failed to write block at %d: %s\n", fm.tstamp, err)
				// TODO: bail here?
				os.Exit(1)
			}
		}
	}(writeChan)

	fmt.Print("Progress:   0% |")
	for scanner.Scan() {
		// create the parsers for the converter based on the title line provided in the CSV file
		if linesRead == 1 {
			if config.Schema == "" {
				if err = csvconv.readSchema(scanner.Text()); err != nil {
					logger.Fatalf("Failed to read schema: %s. Schema title line needed in CSV\n", err)
				}

				// assign interface to row key if it was specified
				if !csvconv.parsesIface() {
					if config.Iface == "" {
						logger.Fatalf("Interface has not been specified by either data or -iface parameter. Aborting")
					}

					p := &IfaceStringParser{}
					if err := p.ParseKey(config.Iface, &rowKeyV4); err != nil {
						logger.Fatalf("Failed to parse interface from config: %s\n", err)
					}
					if err := p.ParseKey(config.Iface, &rowKeyV6); err != nil {
						logger.Fatalf("Failed to parse interface from config: %s\n", err)
					}
				}

				linesRead++
				config.NumLines++ // add a line since the schema does not count as actual data
				continue
			}
		}

		if linesRead == config.NumLines {
			break
		}

		// user status output
		percDone = int(float64(linesRead) / float64(config.NumLines) * 100)
		if percDone != prevPerc {
			if percDone%50 == 0 {
				fmt.Print(" 50% ")
				runtime.GC()
				debug.FreeOSMemory()
			} else if percDone%10 == 0 {
				fmt.Printf("|")

				if linesRead > 1000 {
					// write out the current flow maps
					for iface, tflows := range flowMaps {
						recent := incompleteFlowMap(tflows)
						if len(tflows) > 1 {
							for stamp, flowMap := range tflows {
								if stamp != recent {
									// release flowMap for writing
									writeChan <- writeJob{
										iface:  iface,
										tstamp: stamp,
										data:   flowMap,
									}

									// delete the map from tracking
									delete(flowMaps[iface], stamp)
								}
							}
						}
					}
				}

				runtime.GC()
				debug.FreeOSMemory()
			} else if percDone%2 == 0 {
				fmt.Printf("-")
				runtime.GC()
				debug.FreeOSMemory()
			}
		}
		prevPerc = percDone

		// fully parse the current line and load it into key and value objects
		rowKey := &rowKeyV4
		rowVal := types.Counters{}
		fields := strings.Split(scanner.Text(), ",")
		if len(fields) < len(csvconv.KeyParsers)+len(csvconv.ValParsers) {
			fmt.Printf("Skipping incomplete data row: %s\n", scanner.Text())
			continue
		}
		for _, parser := range csvconv.KeyParsers {
			if err := parser.parser.ParseKey(fields[parser.ind], rowKey); err != nil {
				if errors.Is(err, goDB.ErrIPVersionMismatch) {
					rowKey = &rowKeyV6
					if err := parser.parser.ParseKey(fields[parser.ind], rowKey); err != nil {
						fmt.Println(err)
					}
					continue
				} else {
					fmt.Println(err)
				}
			}
			rowKey = &rowKeyV4
		}
		for ind, parser := range csvconv.ValParsers {
			if err := parser.ParseVal(fields[ind], &rowVal); err != nil {
				fmt.Println(err)
			}
		}

		// check if a new submap has to be created (e.g. if there's new data
		// from another interface
		var iface string
		*rowKey, iface = extractIface(*rowKey)

		ts, _ := rowKey.AttrTime()
		if _, exists := flowMaps[iface]; !exists {
			flowMaps[iface] = make(map[int64]*hashmap.AggFlowMap)
		}
		if _, exists := flowMaps[iface][ts]; !exists {
			flowMaps[iface][ts] = hashmap.NewAggFlowMap()
		}

		// insert the key-value pair into the correct flow map
		if rowKey.IsIPv4() {
			flowMaps[iface][ts].V4Map.Set(rowKey.Key(), rowVal)
		} else {
			flowMaps[iface][ts].V6Map.Set(rowKey.Key(), rowVal)
		}
		linesRead++
	}

	// write out the last flows in the  maps
	for iface, tflows := range flowMaps {
		for stamp, flowMap := range tflows {
			// release flowMap for writing
			writeChan <- writeJob{
				iface:  iface,
				tstamp: stamp,
				data:   flowMap,
			}
		}
	}

	close(writeChan)
	wg.Wait()

	// return if the data write failed or exited
	fmt.Print("| 100%")
	fmt.Println("\nExiting")
	os.Exit(0)
}

func incompleteFlowMap(m map[int64]*hashmap.AggFlowMap) int64 {
	var recent int64
	for k := range m {
		if k > recent {
			recent = k
		}
	}
	return recent
}
