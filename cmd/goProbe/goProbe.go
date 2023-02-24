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
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/flags"
	"github.com/els0r/goProbe/pkg/api"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/discovery"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/version"

	capconfig "github.com/els0r/goProbe/cmd/goProbe/config"
)

const shutdownGracePeriod = 30 * time.Second

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

	// get flags
	err = flags.Read()
	if err != nil {
		os.Exit(1)
	}

	appName := filepath.Base(os.Args[0])
	appVersion := version.GitSHA[0:8]

	if flags.CmdLine.Version {
		fmt.Printf("goProbe\n%s", version.Version())
		os.Exit(0)
	}

	// CPU profiling
	if flags.CmdLine.Profiling {
		f, perr := os.Create(filepath.Join(flags.CmdLine.ProfilingOutputDir, "goprobe_cpu_profile.pprof"))
		if perr != nil {
			panic(perr)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

		defer func() {
			f2, err := os.Create(filepath.Join(flags.CmdLine.ProfilingOutputDir, "goprobe_mem_profile.pprof"))
			if err != nil {
				panic(err)
			}
			pprof.Lookup("allocs").WriteTo(f2, 0)
		}()
	}

	// Config file
	config, err = capconfig.ParseFile(flags.CmdLine.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config file: %v", err)
		os.Exit(1)
	}

	// Initialize logger
	var logOpts = []logging.Option{
		logging.WithDevelopmentMode(config.Logging.DevelopmentMode),
		logging.WithStackTraces(config.Logging.StackTraces),
	}
	err = logging.Init(appName, appVersion, config.Logging.Level, config.Logging.Encoding, logOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v. Exiting!", err)
		os.Exit(1)
	}

	logger := logging.Logger()
	logger.Info("loaded configuration")

	// It doesn't make sense to monitor zero interfaces
	if len(config.Interfaces) == 0 {
		logger.Fatalf("no interfaces have been specified in the configuration file")
	}

	// Limit the number of interfaces
	if len(config.Interfaces) > capture.MaxIfaces {
		logger.Fatalf("cannot monitor more than %d interfaces", capture.MaxIfaces)
	}

	// We quit on encountering SIGTERM or SIGINT (see further down)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer stop()

	// Create DB directory if it doesn't exist already.
	if err := os.MkdirAll(capconfig.RuntimeDBPath(), 0755); err != nil {
		logger.Fatalf("failed to create database directory: %v", err)
	}

	encoderType, err := encoders.GetTypeByString(config.DB.EncoderType)
	if err != nil {
		logger.Fatalf("failed to get encoder type from %s: %v", config.DB.EncoderType, err)
	}

	// Initialize packet logger
	ifaces := make([]string, len(config.Interfaces))
	i := 0
	for k := range config.Interfaces {
		ifaces[i] = k
		i++
	}

	// None of the initialization steps failed.
	logger.Info("started goProbe")

	captureManager = capture.NewManager(ctx)

	// no captures are being deleted here, so we can safely discard the channel we pass
	logger.Debug("updating capture manager configuration")

	captureManager.Update(config.Interfaces, nil)

	// configure api server
	var (
		server     *api.Server
		apiOptions []api.Option
	)

	if config.API.Metrics {
		apiOptions = append(apiOptions, api.WithMetricsExport())
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
		)

		// allow API to update config
		apiOptions = append(apiOptions, api.WithDiscoveryConfigUpdate(discoveryConfigUpdate))
	}

	// start goroutine for writeouts
	writeoutHandler := writeout.NewHandler(captureManager, encoderType).
		WithSyslogWriting(config.SyslogFlows)

	// start writeout handler
	doneWriting := writeoutHandler.HandleWriteouts()

	// start regular rotations
	writeoutHandler.HandleRotations(ctx, time.Duration(goDB.DBWriteInterval)*time.Second)

	// create server and start listening for requests
	server, err = api.New(config.API.Port, captureManager, writeoutHandler, apiOptions...)
	if err != nil {
		logger.Errorf("failed to spawn API server: %s", err)
	} else {
		// start server
		server.Run()
	}

	// report supported API versions if discovery is used
	if config.API.Discovery != nil {
		discoveryConfig = discovery.MakeConfig(config)
		discoveryConfig.Versions = server.SupportedAPIs()

		discoveryConfigUpdate <- discoveryConfig
	}

	// listen for the interrupt signal
	<-ctx.Done()

	// restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	logger.Info("shutting down gracefully")

	// the context is used to inform the server it has ShutdownGracePeriod to wrap up the requests it is
	// currently handling
	fallbackCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	// one last writeout
	writeoutHandler.FullWriteout(fallbackCtx, time.Now())
	writeoutHandler.Close()

	if discoveryConfigUpdate != nil {
		close(discoveryConfigUpdate)
	}

	captureManager.CloseAll()

	select {
	case <-doneWriting:
		logger.Info("graceful shut down completed")
		os.Exit(0)
	case <-fallbackCtx.Done():
		logger.Error("forced shutdown")
		os.Exit(1)
	}
}
<<<<<<< HEAD

func handleRotations(ctx context.Context, manager *capture.Manager) {
	logger := logging.WithContext(ctx)

	var writeoutsChan chan<- capture.Writeout = manager.WriteoutHandler.WriteoutChan

	// one rotation every DBWriteInterval seconds...
	rotateInterval := time.Second * time.Duration(goDB.DBWriteInterval)

	// wait until the next 5 minute interval of the hour is reached before starting the ticker
	tNow := time.Now()

	sleepUntil := tNow.Truncate(rotateInterval).Add(rotateInterval).Sub(tNow)
	logger.Infof("waiting for %s to start capture rotation", sleepUntil.Round(time.Second))

	timer := time.NewTimer(sleepUntil)
	select {
	case <-timer.C:
		break
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(rotateInterval)

	// immediately write out after the initial sleep has completed
	t := time.Now()
	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping rotation handler")
			return
		default:
			logger.Debug("initiating flow data flush")

			manager.LastRotation = t
			woChan := make(chan capture.TaggedAggFlowMap, capture.MaxIfaces)
			writeoutsChan <- capture.Writeout{Chan: woChan, Timestamp: captureManager.LastRotation}
			manager.RotateAll(woChan)
			close(woChan)

			logger = logger.With("queue_length", len(writeoutsChan))

			if len(writeoutsChan) > 2 {
				if len(writeoutsChan) > capture.WriteoutsChanDepth {
					logger.Error("writeouts are lagging behind too much")
					os.Exit(1)
				}
				logger.Warn("writeouts are lagging behind")
			}

			logger.Debug("restarting any interfaces that have encountered errors")
			manager.EnableAll()

			// wait for the the next ticker to complete
			t = <-ticker.C
		}
	}
}

func handleWriteouts(handler *capture.WriteoutHandler, logToSyslog bool) {
	logger := logging.Logger()

	var (
		writeoutsChan  <-chan capture.Writeout = handler.WriteoutChan
		writeoutsCount                         = 0
		dbWriters                              = make(map[string]*goDB.DBWriter)
		lastWrite                              = make(map[string]int)
	)

	var syslogWriter *goDB.SyslogDBWriter
	if logToSyslog {
		var err error
		if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
			// we are not failing here due to the fact that a DB write out should still be attempted.
			logger.Error("failed to create syslog based flow writer: %v", err)
		}
	}

	for writeout := range writeoutsChan {
		t0 := time.Now()
		count := 0
		for taggedMap := range writeout.Chan {
			// Ensure that there is a DBWriter for the given interface
			_, exists := dbWriters[taggedMap.Iface]
			if !exists {
				et, _ := encoders.GetTypeByString(config.DB.EncoderType)
				w := goDB.NewDBWriter(capconfig.RuntimeDBPath(),
					taggedMap.Iface,
					et,
				)
				dbWriters[taggedMap.Iface] = w
			}

			packetsDropped := 0
			if taggedMap.Stats.Pcap != nil {
				packetsDropped = taggedMap.Stats.Pcap.PacketsDropped + taggedMap.Stats.Pcap.PacketsIfDropped
			}

			// Write to database, update summary
			err := dbWriters[taggedMap.Iface].Write(taggedMap.Map, goDB.CaptureMetadata{
				PacketsDropped: packetsDropped,
			}, writeout.Timestamp.Unix())
			lastWrite[taggedMap.Iface] = writeoutsCount
			if err != nil {
				logger.Error(fmt.Sprintf("Error during writeout: %s", err.Error()))
			}

			// write out flows to syslog if necessary
			if logToSyslog {
				if syslogWriter != nil {
					syslogWriter.Write(taggedMap.Map, taggedMap.Iface, writeout.Timestamp.Unix())
				} else {
					logger.Error("cannot write flows to <nil> syslog writer. Attempting reinitialization")

					// try to reinitialize the writer
					if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
						logger.Error("failed to reinitialize syslog writer: %v", err)
					}
				}
			}
			count++
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

		elapsed := time.Since(t0).Round(time.Millisecond)

		logger.With("count", count, "elapsed", elapsed).Debug("completed writeout")
	}

	logger.Info("completed all writeouts")
	handler.DoneWriting()
}
=======
>>>>>>> c23cef4 (Refactor of writeout handling and rotation locking)
