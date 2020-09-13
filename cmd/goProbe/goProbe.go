/////////////////////////////////////////////////////////////////////////////////
//
// goProbe.go
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Binary for the lightweight packet aggregation tool goProbe
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/flags"
	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/discovery"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/els0r/log"

	capconfig "github.com/els0r/goProbe/cmd/goProbe/config"
)

var (
	// cfg may be potentially accessed from multiple goroutines,
	// so we need to synchronize access.
	config *capconfig.Config

	// captureManager may also be accessed
	// from multiple goroutines, so we need to synchronize access.
	captureManager *capture.Manager
)

func main() {
	var err error

	// A general note on error handling: Any errors encountered during startup that make it
	// impossible to run are logged to stderr before the program terminates with a
	// non-zero exit code.
	// Issues encountered during capture will be logged to syslog by default

	// logger for the initial setup phase (logs to stdout)
	var initLogger = log.NewTextLogger()

	// get flags
	err = flags.Read()
	if err != nil {
		os.Exit(1)
	}
	if flags.CmdLine.Version {
		fmt.Printf("goProbe\n%s", version.Version())
		os.Exit(0)
	}

	// Config file
	config, err = capconfig.ParseFile(flags.CmdLine.Config)
	if err != nil {
		initLogger.Errorf("Failed to load config file: %s", err)
		os.Exit(1)
	}

	// Initialize logger
	var logger log.Logger

	// other loggers can be injected here
	logger, err = log.NewFromString(
		config.Logging.Destination,
		log.WithLevel(log.GetLevel(config.Logging.Level)),
	)
	if err != nil {
		initLogger.Errorf("Failed to initialize Logger: %s. Exiting!", err)
		os.Exit(1)
	}
	initLogger.Close()
	defer logger.Close()

	logger.Debug("Loaded config file")

	// It doesn't make sense to monitor zero interfaces
	if len(config.Interfaces) == 0 {
		logger.Error("No interfaces have been specified in the configuration file")
		os.Exit(1)
	}

	// Limit the number of interfaces
	if len(config.Interfaces) > capture.MaxIfaces {
		logger.Errorf("Cannot monitor more than %d interfaces", capture.MaxIfaces)
		os.Exit(1)
	}

	// We quit on encountering SIGTERM or SIGINT (see further down)
	sigExitChan := make(chan os.Signal, 1)
	signal.Notify(sigExitChan, syscall.SIGTERM, os.Interrupt)

	// Create DB directory if it doesn't exist already.
	if err := os.MkdirAll(capconfig.RuntimeDBPath(), 0755); err != nil {
		logger.Errorf("Failed to create database directory: '%s'", err)
		os.Exit(1)
	}

	// Initialize packet logger
	ifaces := make([]string, len(config.Interfaces))
	i := 0
	for k := range config.Interfaces {
		ifaces[i] = k
		i++
	}
	capture.InitPacketLog(config.DBPath, ifaces)
	defer capture.PacketLog.Close()

	// None of the initialization steps failed.
	logger.Info("Started goProbe")

	captureManager = capture.NewManager(logger)

	// No captures are being deleted here, so we can safely discard the channel we pass
	logger.Debug("Updating capture manager configuration")
	captureManager.Update(config.Interfaces, make(chan capture.TaggedAggFlowMap))

	// configure api server
	var (
		server     *api.Server
		apiOptions []api.Option
	)

	if config.API.Metrics {
		apiOptions = append(apiOptions, api.WithMetricsExport())
	}
	if config.API.Logging {
		apiOptions = append(apiOptions, api.WithLogger(logger))
	}
	if len(config.API.Keys) > 0 {
		apiOptions = append(apiOptions, api.WithKeys(config.API.Keys))
	}
	if config.API.Host != "" {
		apiOptions = append(apiOptions, api.WithHost(config.API.Host))
	}
	if config.API.Timeout > 0 {
		apiOptions = append(apiOptions, api.WithTimeout(config.API.Timeout))
	}

	// run go-routine to register with discovery service
	var (
		discoveryConfigUpdate chan *discovery.Config
		discoveryConfig       *discovery.Config
	)
	if config.API.Discovery != nil {

		var clientOpts []discovery.Option
		if config.API.Discovery.SkipVerify {
			clientOpts = append(clientOpts, discovery.WithAllowSelfSignedCerts())
		}

		discoveryConfigUpdate = discovery.RunConfigRegistration(
			discovery.NewClient(config.API.Discovery.Registry, clientOpts...),
			logger,
		)

		// allow API to update config
		apiOptions = append(apiOptions, api.WithDiscoveryConfigUpdate(discoveryConfigUpdate))
	}

	// create server and start listening for requests
	server, err = api.New(config.API.Port, captureManager, apiOptions...)
	if err != nil {
		logger.Errorf("failed to spawn API server: %s", err)
	} else {
		// handle -docgen flag
		if flags.CmdLine.DocGen {
			var mdRoutes, jsonRoutes *os.File

			logger.Info("generating API docs and quitting")

			// open files
			fname := "pkg/api/"
			mdRoutes, err = os.OpenFile(fname+"README.md", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				logger.Errorf("failed to open '%s.md' for writing: %s", fname, err)
				os.Exit(1)
			}
			jsonRoutes, err = os.OpenFile(fname+"routes.json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				logger.Errorf("failed to open '%s.json' for writing: %s", fname, err)
				os.Exit(1)
			}

			// generate the documentation
			err = server.GenerateAPIDocs(jsonRoutes, mdRoutes)
			if err != nil {
				logger.Errorf("docgen failed: %s", err)
				os.Exit(1)
			}

			// exit program
			os.Exit(0)
		}

		// start server
		server.Run()
	}

	// report supported API versions if discovery is used
	if config.API.Discovery != nil {
		discoveryConfig = discovery.MakeConfig(config)
		discoveryConfig.Versions = server.SupportedAPIs()

		discoveryConfigUpdate <- discoveryConfig
	}

	// Start goroutine for writeouts
	go handleWriteouts(captureManager.WriteoutHandler, config.SyslogFlows, logger)

	// Start regular rotations
	go handleRotations(captureManager, logger)

	// Wait for signal to exit
	<-sigExitChan

	logger.Debug("Shutting down")

	// We intentionally don't unlock the mutex hereafter,
	// because the program exits anyways. This ensures that there
	// can be no new Rotations/Updates/etc... while we're shutting down.
	var (
		writeoutsChan          chan<- capture.Writeout = captureManager.WriteoutHandler.WriteoutChan
		completedWriteoutsChan                         = captureManager.WriteoutHandler.CompletedChan
	)
	captureManager.DisableAll()

	// One last writeout
	woChan := make(chan capture.TaggedAggFlowMap, capture.MaxIfaces)
	writeoutsChan <- capture.Writeout{woChan, time.Now()}
	captureManager.RotateAll(woChan)
	close(woChan)
	close(writeoutsChan)
	if discoveryConfigUpdate != nil {
		close(discoveryConfigUpdate)
	}

	captureManager.CloseAll()

	<-completedWriteoutsChan

	return
}

func handleRotations(manager *capture.Manager, logger log.Logger) {
	var writeoutsChan chan<- capture.Writeout = manager.WriteoutHandler.WriteoutChan

	// One rotation every DBWriteInterval seconds...
	ticker := time.NewTicker(time.Second * time.Duration(goDB.DBWriteInterval))
	for {
		select {
		case <-ticker.C:
			logger.Debug("Initiating flow data flush")

			manager.LastRotation = time.Now()
			woChan := make(chan capture.TaggedAggFlowMap, capture.MaxIfaces)
			writeoutsChan <- capture.Writeout{woChan, captureManager.LastRotation}
			manager.RotateAll(woChan)
			close(woChan)

			if len(writeoutsChan) > 2 {
				if len(writeoutsChan) > capture.WriteoutsChanDepth {
					logger.Error(fmt.Sprintf("Writeouts are lagging behind too much: Queue length is %d", len(writeoutsChan)))
					os.Exit(1)
				}
				logger.Warn(fmt.Sprintf("Writeouts are lagging behind: Queue length is %d", len(writeoutsChan)))
			}

			logger.Debug("Restarting any interfaces that have encountered errors.")
			manager.EnableAll()
		}
	}
}

func handleWriteouts(handler *capture.WriteoutHandler, logToSyslog bool, logger log.Logger) {
	var (
		writeoutsChan  <-chan capture.Writeout = handler.WriteoutChan
		doneChan       chan<- struct{}         = handler.CompletedChan
		writeoutsCount                         = 0
		dbWriters                              = make(map[string]*goDB.DBWriter)
		lastWrite                              = make(map[string]int)
	)

	var syslogWriter *goDB.SyslogDBWriter
	if logToSyslog {
		var err error
		if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
			// we are not failing here due to the fact that a DB write out should still be attempted.
			// TODO: consider making a hard fail configurable
			logger.Error(fmt.Sprintf("Failed to create syslog based flow writer: %s", err.Error()))
		}
	}

	for writeout := range writeoutsChan {
		t0 := time.Now()
		var summaryUpdates []goDB.InterfaceSummaryUpdate
		count := 0
		for taggedMap := range writeout.Chan {
			// Ensure that there is a DBWriter for the given interface
			_, exists := dbWriters[taggedMap.Iface]
			if !exists {
				et, _ := encoders.GetTypeByString(config.EncoderType)
				w := goDB.NewDBWriter(capconfig.RuntimeDBPath(),
					taggedMap.Iface,
					et,
				)
				dbWriters[taggedMap.Iface] = w
			}

			// Prep metadata for current block
			meta := goDB.BlockMetadata{}
			meta.PcapPacketsReceived = -1
			meta.PcapPacketsDropped = -1
			meta.PcapPacketsIfDropped = -1
			if taggedMap.Stats.Pcap != nil {
				meta.PcapPacketsReceived = taggedMap.Stats.Pcap.PacketsReceived
				meta.PcapPacketsDropped = taggedMap.Stats.Pcap.PacketsDropped
				meta.PcapPacketsIfDropped = taggedMap.Stats.Pcap.PacketsIfDropped
			}
			meta.PacketsLogged = taggedMap.Stats.PacketsLogged
			meta.Timestamp = writeout.Timestamp.Unix()

			// Write to database, update summary
			update, err := dbWriters[taggedMap.Iface].Write(taggedMap.Map, meta, writeout.Timestamp.Unix())
			lastWrite[taggedMap.Iface] = writeoutsCount
			if err != nil {
				logger.Error(fmt.Sprintf("Error during writeout: %s", err.Error()))
			} else {
				summaryUpdates = append(summaryUpdates, update)
			}

			// write out flows to syslog if necessary
			if logToSyslog {
				if syslogWriter != nil {
					syslogWriter.Write(taggedMap.Map, taggedMap.Iface, writeout.Timestamp.Unix())
				} else {
					logger.Error("Cannot write flows to <nil> syslog writer. Attempting reinitialization.")

					// try to reinitialize the writer
					if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
						logger.Error(fmt.Sprintf("Failed to reinitialize syslog writer: %s", err.Error()))
					}
				}
			}

			count++
		}

		// We are done with the writeout, let's try to write the updated summary
		err := goDB.ModifyDBSummary(capconfig.RuntimeDBPath(), 10*time.Second, func(summ *goDB.DBSummary) (*goDB.DBSummary, error) {
			if summ == nil {
				summ = goDB.NewDBSummary()
			}
			for _, update := range summaryUpdates {
				summ.Update(update)
			}
			return summ, nil
		})
		if err != nil {
			logger.Error(fmt.Sprintf("Error updating summary: %s", err.Error()))
		}

		// Clean up dead writers. We say that a writer is dead
		// if it hasn't been used in the last few writeouts.
		var remove []string
		for iface, last := range lastWrite {
			if writeoutsCount-last >= 3 {
				remove = append(remove, iface)
			}
		}
		for _, iface := range remove {
			delete(dbWriters, iface)
			delete(lastWrite, iface)
		}

		writeoutsCount++
		logger.Debug(fmt.Sprintf("Completed writeout (count: %d) in %s", count, time.Now().Sub(t0)))
	}

	logger.Debug("Completed all writeouts")
	doneChan <- struct{}{}
}
