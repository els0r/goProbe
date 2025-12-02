// Package cmd contains the goProbe command line interface implementation
package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/capture"
	"github.com/els0r/goProbe/v4/pkg/conf"
	"github.com/els0r/goProbe/v4/pkg/version"
	"github.com/els0r/telemetry/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/time/rate"

	gpconf "github.com/els0r/goProbe/v4/cmd/goProbe/config"
	gpserver "github.com/els0r/goProbe/v4/pkg/api/goprobe/server"
)

const shutdownGracePeriod = 30 * time.Second

var (
	defaultRequestDurationHistogramBins = []float64{0.01, 0.05, 0.1, 0.25, 1, 5, 10, 30, 60, 300}
)

func Execute() error {
	rootCmd, err := newRootCmd(run)
	if err != nil {
		return err
	}

	rootCmd.AddCommand(newVersionCmd())

	return rootCmd.Execute()
}

// runFunc is the type of the function that is called when the root command is executed. It's defined
// mainly for testing purposes
type runFunc func(ctx context.Context, cfg *gpconf.Config) error

func newRootCmd(run runFunc) (*cobra.Command, error) {
	cfg := &gpconf.Config{}

	rootCmd := &cobra.Command{
		Use:   "goProbe",
		Short: "goProbe is a network traffic metadata capture tool",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := initConfig(cfg)
			if err != nil {
				return fmt.Errorf("failed to initialize configuration: %w", err)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			return initLogging()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), cfg)
		},
	}

	err := registerFlags(rootCmd, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to register flags: %w", err)
	}

	return rootCmd, nil
}

const (
	flagOpenAPISpecOutfile = "openapi.spec-outfile"

	dbKey             = "db"
	flagDBPath        = dbKey + ".path"
	flagDBEncoderType = dbKey + ".encoder_type"
	flagDBPermissions = dbKey + ".permissions"

	autodetectionKey         = "autodetection"
	flagAutodetectionEnabled = autodetectionKey + ".enabled"
	flagAutodetectionExclude = autodetectionKey + ".exclude"

	flagSyslogFlows = "syslog_flows"

	apiKey                             = "api"
	flagAPIAddr                        = apiKey + ".addr"
	flagAPIMetrics                     = apiKey + ".metrics"
	flagAPIDisableIfaceMetrics         = apiKey + ".disable_iface_metrics"
	flagAPIProfiling                   = apiKey + ".profiling"
	flagAPITimeout                     = apiKey + ".request_timeout"
	flagAPIKeys                        = apiKey + ".keys"
	flagAPIQueryRateLimitMaxReq        = apiKey + ".query_rate_limit.max_req_per_sec"
	flagAPIQueryRateLimitMaxBurst      = apiKey + ".query_rate_limit.max_burst"
	flagAPIQueryRateLimitMaxConcurrent = apiKey + ".query_rate_limit.max_concurrent"

	localBuffersKey            = "local_buffers"
	flagLocalBuffersSizeLimit  = localBuffersKey + ".size_limit"
	flagLocalBuffersNumBuffers = localBuffersKey + ".num_buffers"
)

func registerFlags(cmd *cobra.Command, cfg *gpconf.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration must not be nil")
	}

	pflags := cmd.PersistentFlags()

	conf.RegisterFlags(cmd)

	// NOTE: this is a breaking change compared to the previous flag parsing implementation
	// needs a deprecation notice for 4.2
	pflags.String(flagOpenAPISpecOutfile, "", "write OpenAPI 3.0.3 spec to output file and exit")

	// db config bindings
	pflags.StringVar(&cfg.DB.Path, flagDBPath, "", "path to database directory")
	pflags.StringVar(&cfg.DB.EncoderType, flagDBEncoderType, "", "database encoder type")
	pflags.Uint32(flagDBPermissions, 0, "database file permissions")

	// autodetect config bindings
	pflags.BoolVar(&cfg.AutoDetection.Enabled, flagAutodetectionEnabled, false, "enable auto-detection of interfaces")
	pflags.StringSliceVar(&cfg.AutoDetection.Exclude, flagAutodetectionExclude, nil, "list of interface names to exclude from auto-detection")

	// top-level config bindings
	pflags.BoolVar(&cfg.SyslogFlows, flagSyslogFlows, false, "enable syslog flow logging")

	// api config bindings (optional section)
	pflags.StringVar(&cfg.API.Addr, flagAPIAddr, "", "API server address")
	pflags.BoolVar(&cfg.API.Metrics, flagAPIMetrics, false, "enable API metrics")
	pflags.BoolVar(&cfg.API.DisableIfaceMetrics, flagAPIDisableIfaceMetrics, false, "disable per-interface metrics")
	pflags.BoolVar(&cfg.API.Profiling, flagAPIProfiling, false, "enable API profiling")
	pflags.IntVar(&cfg.API.Timeout, flagAPITimeout, 0, "API request timeout in seconds")
	pflags.StringSliceVar(&cfg.API.Keys, flagAPIKeys, nil, "API authentication keys")
	pflags.Float64(flagAPIQueryRateLimitMaxReq, 0, "maximum query requests per second")
	pflags.IntVar(&cfg.API.QueryRateLimit.MaxBurst, flagAPIQueryRateLimitMaxBurst, 0, "maximum query burst size")
	pflags.IntVar(&cfg.API.QueryRateLimit.MaxConcurrent, flagAPIQueryRateLimitMaxConcurrent, 0, "maximum concurrent queries")

	// local_buffers config bindings (optional section)
	pflags.IntVar(&cfg.LocalBuffers.SizeLimit, flagLocalBuffersSizeLimit, 0, "local buffer size limit")
	pflags.IntVar(&cfg.LocalBuffers.NumBuffers, flagLocalBuffersNumBuffers, 0, "number of local buffers")

	return viper.BindPFlags(pflags)
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cfg *gpconf.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration must not be nil")
	}

	// Initialize the Interfaces map if it's nil (required for viper.Unmarshal to work correctly)
	if cfg.Interfaces == nil {
		cfg.Interfaces = make(gpconf.Ifaces)
	}

	path := viper.GetString(conf.ConfigFile)
	if path != "" {
		viper.SetConfigFile(path)

		err := viper.ReadInConfig()
		if err != nil {
			return fmt.Errorf("failed to read configuration file: %w", err)
		}
	}

	// Configure key replacer for both config files and environment variables
	// This allows config file keys with underscores (e.g., encoder_type, syslog_flows)
	// to be accessed via flag keys with underscores (e.g., db.encoder_type, syslog_flows)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "__"))
	viper.AutomaticEnv()

	// Unmarshal the entire config from viper (includes config file, flags, and env vars)
	err := viper.Unmarshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to parse configuration: %v", err)
	}

	// Type conversions for types that viper can't unmarshal directly

	// DB permissions: uint32 to fs.FileMode
	cfg.DB.Permissions = os.FileMode(viper.GetUint32(flagDBPermissions))

	// API QueryRateLimit: float64 to rate.Limit
	if viper.IsSet(flagAPIQueryRateLimitMaxReq) {
		cfg.API.QueryRateLimit.MaxReqPerSecond = rate.Limit(viper.GetFloat64(flagAPIQueryRateLimitMaxReq))
	}

	return nil
}

func initLogging() error {
	// Initialize logger
	appVersion := version.Version()
	loggerOpts := []logging.Option{
		logging.WithVersion(appVersion),
	}

	dst := viper.GetString(conf.LogDestination)
	if dst != "" {
		loggerOpts = append(loggerOpts, logging.WithFileOutput(dst))
	}

	err := logging.Init(
		logging.LevelFromString(viper.GetString(conf.LogLevel)),
		logging.Encoding(viper.GetString(conf.LogEncoding)),
		loggerOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	return nil
}

func run(ctx context.Context, cfg *gpconf.Config) error {
	// A general note on error handling: Any errors encountered during startup that make it
	// impossible to run are logged to stderr before the program terminates with a
	// non-zero exit code.
	// Issues encountered during capture will be logged to syslog by default

	// Read / parse config file
	configMonitor, err := gpconf.NewMonitor(viper.GetString(conf.ConfigFile), gpconf.WithInitialConfig(cfg))
	if err != nil {
		return fmt.Errorf("failed to initialize config file monitor: %w", err)
	}
	_, _, _, err = configMonitor.Reload(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize config monitor: %w", err)
	}
	config := configMonitor.GetConfig()

	logger := logging.FromContext(ctx)
	logger.Info("loaded configuration")

	// write spec and exit
	openAPIfile := viper.GetString(flagOpenAPISpecOutfile)
	if openAPIfile != "" {
		// skeleton server just for route registration
		err := server.GenerateSpec(context.Background(), openAPIfile, gpserver.New("127.0.0.1:8145", config.DB.Path, nil, nil))
		if err != nil {
			return fmt.Errorf("failed to generate OpenAPI spec: %w", err)
		}
		os.Exit(0)
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
		return fmt.Errorf("failed to create database directory: %v", err)
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
		return fmt.Errorf("failed to initialize capture manager: %v", err)
	}

	// Initialize constant monitoring / reloading of the config file
	configMonitor.Start(ctx, captureManager.Update)

	config = configMonitor.GetConfig()

	// It doesn't make sense to monitor zero interfaces
	if len(config.Interfaces) == 0 {
		return fmt.Errorf("no interfaces have been detected for monitoring")
	}

	// Limit the number of interfaces
	if len(config.Interfaces) > capture.MaxIfaces {
		return fmt.Errorf("cannot monitor more than %d interfaces", capture.MaxIfaces)
	}

	// configure api server
	var apiServer *gpserver.Server

	// create server and start listening for requests
	if config.API.Addr != "" {
		var apiOptions = []server.Option{

			// Set the release mode of GIN depending on the log level
			server.WithDebugMode(
				logging.LevelFromString(viper.GetString(conf.LogLevel)) == logging.LevelDebug,
			),
			server.WithProfiling(config.API.Profiling),

			// this line will enable not only HTTP request metrics, but also the default prometheus golang client
			// metrics for memory, cpu, gc performance, etc.
			server.WithMetrics(config.API.Metrics, defaultRequestDurationHistogramBins...),

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
	if config.API.Addr != "" && apiServer != nil {
		// TODO: Technically this is a race condition (as detected by the race detector) because it accesses the
		// underlying server being accessed in the (blocking) call to apiServer.Serve() - This is kind of by design
		// but maybe there are ways to clean it up (not urgent, just a bit "unclean")
		err := apiServer.Shutdown(fallbackCtx)
		if err != nil {
			return fmt.Errorf("forced shut down of goProbe API server: %v", err)
		}
	}

	captureManager.Close(fallbackCtx)
	logger.Info("graceful shut down completed")

	return nil
}
