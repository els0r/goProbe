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
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/flags"
	gpserver "github.com/els0r/goProbe/pkg/api/goprobe/server"
	"github.com/els0r/goProbe/pkg/api/server"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/els0r/telemetry/logging"

	gpconf "github.com/els0r/goProbe/cmd/goProbe/config"
)

const shutdownGracePeriod = 30 * time.Second

func main() {

	// A general note on error handling: Any errors encountered during startup that make it
	// impossible to run are logged to stderr before the program terminates with a
	// non-zero exit code.
	// Issues encountered during capture will be logged to syslog by default

	// Read / parse command-line flags
	if err := flags.Read(); err != nil {
		os.Exit(1)
	}
	appVersion := version.Short()
	if flags.CmdLine.Version {
		fmt.Printf("%s", version.Version())
		os.Exit(0)
	}

	// Read / parse config file
	configMonitor, err := gpconf.NewMonitor(flags.CmdLine.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize config file monitor: %v\n", err)
		os.Exit(1)
	}
	config := configMonitor.GetConfig()

	// Initialize logger
	loggerOpts := []logging.Option{
		logging.WithVersion(appVersion),
	}
	if config.Logging.Destination != "" {
		loggerOpts = append(loggerOpts, logging.WithFileOutput(config.Logging.Destination))
	}

	err = logging.Init(logging.LevelFromString(config.Logging.Level), logging.Encoding(config.Logging.Encoding),
		loggerOpts...,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger := logging.Logger()
	logger.Info("loaded configuration")

	// write spec and exit
	openAPIfile := flags.CmdLine.OpenAPISpecOutfile
	if openAPIfile != "" {
		// skeleton server just for route registration
		err := server.GenerateSpec(context.Background(), openAPIfile, gpserver.New("127.0.0.1:8145", config.DB.Path, nil, nil))
		if err != nil {
			logger.Fatal(err)
		}
		os.Exit(0)
	}

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

	// Initialize packet logger
	ifaces := make([]string, len(config.Interfaces))
	i := 0
	for k := range config.Interfaces {
		ifaces[i] = k
		i++
	}

	// Create DB directory if it doesn't exist already.
	// #nosec G301
	if err := os.MkdirAll(filepath.Clean(config.DB.Path), 0755); err != nil {
		logger.Fatalf("failed to create database directory: %v", err)
	}

	var cmOpts []capture.ManagerOption
	if config.API.Metrics {
		var metricsOpts []capture.MetricsOption
		if config.API.DisableIfaceMetrics {
			metricsOpts = append(metricsOpts, capture.DisableIfaceTracking())
		}
		cmOpts = append(cmOpts, capture.WithMetrics(capture.NewMetrics(metricsOpts...)))
	}

	// None of the initialization steps failed.
	captureManager, err := capture.InitManager(ctx, config, cmOpts...)
	if err != nil {
		logger.Fatal(err)
	}

	// Initialize constant monitoring / reloading of the config file
	configMonitor.Start(ctx, captureManager.Update)

	// configure api server
	var apiServer *gpserver.Server

	// create server and start listening for requests
	if config.API != nil {
		var apiOptions = []server.Option{

			// Set the release mode of GIN depending on the log level
			server.WithDebugMode(
				logging.LevelFromString(config.Logging.Level) == logging.LevelDebug,
			),
			server.WithProfiling(config.API.Profiling),

			// this line will enable not only HTTP request metrics, but also the default prometheus golang client
			// metrics for memory, cpu, gc performance, etc.
			server.WithMetrics(config.API.Metrics, capture.DefaultMetricsHistogramBins...),

			// enable global query rate limit if provided
			server.WithQueryRateLimit(config.API.QueryRateLimit.MaxReqPerSecond, config.API.QueryRateLimit.MaxBurst, config.API.QueryRateLimit.MaxConcurrent),
		}
		// if len(config.API.Keys) > 0 {
		// 	apiOptions = append(apiOptions, api.WithKeys(config.API.Keys))
		// }

		apiServer = gpserver.New(config.API.Addr, config.DB.Path, captureManager, configMonitor, apiOptions...)

		// serve API
		go func() {
			logger.With("addr", config.API.Addr).Info("starting API server")
			err := apiServer.Serve()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatalf("failed to spawn goProbe API server: %s", err)
			}
		}()
	}

	logger.Info("started goProbe")

	// listen for the interrupt signal
	<-ctx.Done()

	// restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	logger.Info("shutting down gracefully")

	// the context is used to inform the server it has ShutdownGracePeriod to wrap up the requests it is
	// currently handling
	fallbackCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	// shut down running server resources, forcibly if need be
	if config.API != nil {
		// TODO: Technically this is a race condition (as detected by the race detector) because it accesses the
		// underlying server being accessed in the (blocking) call to apiServer.Serve() - This is kind of by design
		// but maybe there are ways to clean it up (not urgent, just a bit "unclean")
		err := apiServer.Shutdown(fallbackCtx)
		if err != nil {
			logger.Errorf("forced shut down of goProbe API server: %v", err)
		}
	}

	captureManager.Close(fallbackCtx)
	logger.Info("graceful shut down completed")
}
