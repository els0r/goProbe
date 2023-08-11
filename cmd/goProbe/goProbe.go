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
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/version"

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
	config, err := gpconf.ParseFile(flags.CmdLine.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config file: %v\n", err)
		os.Exit(1)
	}

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
	if err := os.MkdirAll(filepath.Clean(config.DB.Path), 0755); err != nil {
		logger.Fatalf("failed to create database directory: %v", err)
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
	captureManager, err := capture.InitManager(ctx, config)
	if err != nil {
		logger.Fatal(err)
	}

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
			server.WithMetrics(config.API.Metrics, []float64{0.01, 0.05, 0.1, 0.25, 1, 5, 10, 30, 60, 300}...),
		}
		// if len(config.API.Keys) > 0 {
		// 	apiOptions = append(apiOptions, api.WithKeys(config.API.Keys))
		// }

		apiServer = gpserver.New(config.API.Addr, captureManager, apiOptions...)
		apiServer.SetDBPath(config.DB.Path)

		logger.With("addr", config.API.Addr).Info("starting API server")
		go func() {
			err = apiServer.Serve()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatalf("failed to spawn goProbe API server: %s", err)
			}
		}()
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

	// shut down running server resources, forcibly if need be
	if config.API != nil {
		err = apiServer.Shutdown(fallbackCtx)
		if err != nil {
			logger.Errorf("forced shut down of goProbe API server: %v", err)
		}
	}

	captureManager.Close(fallbackCtx)
	logger.Info("graceful shut down completed")
}
