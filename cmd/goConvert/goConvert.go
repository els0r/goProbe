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
	"errors"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	// for metrics export to metricsbeat
	_ "expvar"

	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	log "github.com/els0r/log"

	"flag"
	"fmt"
)

// Config stores the flags provided to the converter
type Config struct {
	FilePath    string
	SavePath    string
	Iface       string
	Schema      string
	NumLines    int
	EncoderType int
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

// CSVConverter can read CSV files containing goProbe flow information
type CSVConverter struct {
	// map field index to how it should be parsed
	KeyParsers []keyIndParserItem
	ValParsers map[int]goDB.StringValParser

	// logging
	logger log.Logger
}

// Option allows to configure the CSVConverter
type Option func(*CSVConverter)

// WithLogger passes a logger to the CSVConverter
func WithLogger(l log.Logger) Option {
	return func(c *CSVConverter) {
		c.logger = l
	}
}

// NewCSVConverter initializes a CSVConverter with the Key- and Value parsers for goProbe flows
func NewCSVConverter(opts ...Option) *CSVConverter {

	c := &CSVConverter{
		KeyParsers: make([]keyIndParserItem, 0),
		ValParsers: make(map[int]goDB.StringValParser),
	}

	// apply options
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *CSVConverter) readSchema(schema string) error {

	fields := strings.Split(schema, ",")

	var (
		canParse  = make([]string, len(fields))
		cantParse = make([]string, len(fields))
	)

	// first try to extract all attributes which need to be parsed
	for ind, field := range fields {
		parser := goDB.NewStringKeyParser(field)

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

	// Ensure that IP parsers are executed first (if present) to ensure correct parsing
	sort.Slice(c.KeyParsers, func(i, j int) bool {
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
	if c.logger != nil {
		c.logger.Debugf("SCHEMA: can parse: %s. Will not parse: %s", canParse, cantParse)
	}
	return nil
}

func (c *CSVConverter) parsesIface() bool {
	for _, p := range c.KeyParsers {
		if _, ok := p.parser.(*goDB.IfaceStringParser); ok {
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
	logger, err := log.NewFromString("console", log.WithLevel(log.DEBUG))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to spawn logger: %s\n", err)
		os.Exit(1)
	}

	// get number of lines to read in the specified file
	cmd := exec.Command("wc", "-l", config.FilePath)
	out, cmderr := cmd.Output()
	if cmderr != nil {
		logger.Errorf("could not obtain line count on file %s", config.FilePath)
		os.Exit(1)
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
		logger.Errorf("file open error: %s", err)
		os.Exit(1)
	}

	// create a CSV converter
	var csvconv = NewCSVConverter(WithLogger(logger))
	if config.Schema != "" {
		if err = csvconv.readSchema(config.Schema); err != nil {
			logger.Errorf("failed to read schema: %s", err)
			os.Exit(1)
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
		rowKeyV4   = types.NewEmptyV4Key().ExtendEmpty()
		rowKeyV6   = types.NewEmptyV6Key().ExtendEmpty()
		flowMaps   = make(map[string]map[int64]*hashmap.AggFlowMap)
		rowSummary goDB.InterfaceSummaryUpdate
	)

	// try to read a summary file from the output folder. It may exist if data was previously written
	// to the directory already.
	summary := goDB.NewDBSummary()
	summary, err = goDB.ReadDBSummary(config.SavePath)
	if err != nil {
		if os.IsNotExist(err) {
			summary = goDB.NewDBSummary()
		} else {
			fmt.Printf("Summary file for DB exists but cannot be read: %s\n", err.Error())
			os.Exit(1)
		}
	}

	// channel for passing flow maps to writer
	writeChan := make(chan writeJob, 1024)

	// writer routine accepting flow maps to write out
	var wg sync.WaitGroup
	wg.Add(1)
	go func(writeChan chan writeJob) {
		defer wg.Done()
		for fm := range writeChan {
			if _, ok := mapWriters[fm.iface]; !ok {
				mapWriters[fm.iface] = goDB.NewDBWriter(config.SavePath, fm.iface, encoders.Type(config.EncoderType))
			}

			// create an empty metadata block for this timestamp. Of course this
			// isn't accurate, but we cannot recover the info from pcap anyhow at
			// that moment
			bm := goDB.BlockMetadata{Timestamp: fm.tstamp}
			//        fmt.Println(fm.iface+": Writing:", fm.data)
			if _, err = mapWriters[fm.iface].Write(fm.data, bm, fm.tstamp); err != nil {
				fmt.Printf("Failed to write block at %d: %s\n", fm.tstamp, err.Error())
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
					fmt.Printf("Failed to read schema: %s. Schema title line needed in CSV\n", err.Error())
					os.Exit(1)
				}

				// assign interface to row key if it was specified
				if !csvconv.parsesIface() {
					if config.Iface == "" {
						fmt.Printf("Interface has not been specified by either data or -iface parameter. Aborting")
						os.Exit(1)
					}

					p := &goDB.IfaceStringParser{}
					if err := p.ParseKey(config.Iface, &rowKeyV4); err != nil {
						fmt.Printf("Failed to parse interface from config: %s\n", err.Error())
						os.Exit(1)
					}
					if err := p.ParseKey(config.Iface, &rowKeyV6); err != nil {
						fmt.Printf("Failed to parse interface from config: %s\n", err.Error())
						os.Exit(1)
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
		rowVal := types.Val{}
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
		iface, _ := rowKey.AttrIface()
		ts, _ := rowKey.AttrTime()
		if _, exists := flowMaps[iface]; !exists {
			flowMaps[iface] = make(map[int64]*hashmap.AggFlowMap)
		}
		if _, exists := flowMaps[iface][ts]; !exists {
			flowMaps[iface][ts] = hashmap.New()
		}

		// insert the key-value pair into the correct flow map
		flowMaps[iface][ts].Set(rowKey.Key(), rowVal)

		// fill the summary update for this flow record and update the summary
		rowSummary.Interface = iface
		rowSummary.FlowCount = 1
		rowSummary.Traffic = rowVal.NBytesRcvd + rowVal.NBytesSent
		rowSummary.Timestamp = time.Unix(ts, 0)

		summary.Update(rowSummary)

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

	// summary file update: this assumes that the summary was not modified during conversion
	// of the CSV database. If a goProbe process were to write to the summary in the meantime,
	// those changes would be overwritten.
	err = goDB.ModifyDBSummary(config.SavePath, 10*time.Second,
		func(summ *goDB.DBSummary) (*goDB.DBSummary, error) {
			return summary, nil
		},
	)
	if err != nil {
		fmt.Printf("Failed to update summary: %s\n", err.Error())
		os.Exit(1)
	}

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
