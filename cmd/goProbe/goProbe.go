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
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/flags"
	"github.com/els0r/goProbe/pkg/api/goprobe/server"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/version"

	capconfig "github.com/els0r/goProbe/cmd/goProbe/config"
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
		fmt.Printf("goProbe\n%s", version.Version())
		os.Exit(0)
	}

	// Read / parse config file
	config, err := capconfig.ParseFile(flags.CmdLine.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config file: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	err = logging.Init(logging.LevelFromString(config.Logging.Level), logging.Encoding(config.Logging.Encoding),
		logging.WithVersion(appVersion),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger := logging.Logger()
	logger.Info("loaded configuration")

	// Setup profiling (if enabled)
	if flags.CmdLine.ProfilingOutputDir != "" {
		if err := startProfiling(flags.CmdLine.ProfilingOutputDir); err != nil {
			logger.Fatal(err)
		}
		defer func() {
			if err := stopProfiling(flags.CmdLine.ProfilingOutputDir); err != nil {
				logger.Fatal(err)
			}
		}()
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
	var (
		apiServer  *server.Server
		apiOptions = []server.Option{
			server.WithDBPath(config.DB.Path),
			// Set the release mode of GIN depending on the log level
			server.WithDebugMode(
				logging.LevelFromString(config.Logging.Level) == logging.LevelDebug,
			),
		}
	)

	// if config.API.Metrics {
	// 	apiOptions = append(apiOptions, api.WithMetricsExport())
	// }
	// if len(config.API.Keys) > 0 {
	// 	apiOptions = append(apiOptions, api.WithKeys(config.API.Keys))
	// }
	// if config.API.Host != "" {
	// 	apiOptions = append(apiOptions, api.WithHost(config.API.Host))
	// }
	// if config.API.Timeout > 0 {
	// 	apiOptions = append(apiOptions, api.WithTimeout(config.API.Timeout))
	// }

	// create server and start listening for requests
	if config.API != nil {
		apiServer = server.New(config.API.Addr, captureManager, apiOptions...)

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

func startProfiling(dirPath string) error {

	err := os.MkdirAll(dirPath, 0750)
	if err != nil {
		return fmt.Errorf("failed to create pprof directory: %w", err)
	}

	f, err := os.Create(filepath.Join(filepath.Clean(dirPath), "goprobe_cpu_profile.pprof"))
	if err != nil {
		return fmt.Errorf("failed to create CPU profile file: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		return fmt.Errorf("failed to start CPU profiling: %w", err)
	}

	return nil
}

func stopProfiling(dirPath string) error {

	pprof.StopCPUProfile()

	f, err := os.Create(filepath.Join(filepath.Clean(dirPath), "goprobe_mem_profile.pprof"))
	if err != nil {
		return fmt.Errorf("failed to create memory profile file: %w", err)
	}
	if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to start memory profiling: %w", err)
	}

	return nil
}
