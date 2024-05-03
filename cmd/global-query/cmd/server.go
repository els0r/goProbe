package cmd

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/els0r/goProbe/cmd/global-query/pkg/conf"
	gqserver "github.com/els0r/goProbe/pkg/api/globalquery/server"
	"github.com/els0r/goProbe/pkg/api/server"
	"github.com/els0r/goProbe/plugins"
	"github.com/els0r/telemetry/logging"
	"github.com/els0r/telemetry/tracing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/els0r/goProbe/plugins/contrib" // Include third-party plugins (if enabled, see README)
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run global-query in server mode",
	Long:  "Run global-query in server mode",
	RunE:  serverEntrypoint,
}

func init() {
	rootCmd.AddCommand(serverCmd)

	pflags := serverCmd.PersistentFlags()

	pflags.String(conf.ServerAddr, conf.DefaultServerAddr, "address to which the server binds")
	pflags.Duration(conf.ServerShutdownGracePeriod, conf.DefaultServerShutdownGracePeriod, "duration the server will wait during shutdown before forcing shutdown")

	pflags.String(conf.OpenAPI, "", "write OpenAPI 3.0.3 spec to output file and exit")

	// telemetry
	pflags.Bool(conf.ProfilingEnabled, false, "enable profiling endpoints")

	_ = viper.BindPFlags(pflags)
}

func serverEntrypoint(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer stop()

	logger := logging.FromContext(ctx)

	// print OpenAPI spec and exit
	openAPIfile := viper.GetString(conf.OpenAPI)
	if openAPIfile != "" {
		// create skeleton server
		apiServer := gqserver.New("0.0.0.0:8146", nil, nil)

		logger.With("path", openAPIfile).Info("writing OpenAPI spec only")
		f, err := os.OpenFile(openAPIfile, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		return apiServer.OpenAPI(f)
	}

	shutdownTracing, err := tracing.InitFromFlags(ctx)
	if err != nil {
		logger.With("error", err).Error("failed to set up tracing")
	}

	hostListResolver, err := initHostListResolver()
	if err != nil {
		logger.Errorf("failed to prepare query: %v", err)
		return err
	}

	qlogger := logger.With("plugins", plugins.GetInitializer())
	qlogger.Debug("getting available plugins")

	// get the querier
	querier, err := initQuerier(ctx)
	if err != nil {
		qlogger.Errorf("failed to set up queriers: %v", err)
		return err
	}

	// set up the API server
	addr := viper.GetString(conf.ServerAddr)
	apiServer := gqserver.New(addr, hostListResolver, querier,
		// Set the release mode of GIN depending on the log level
		server.WithDebugMode(
			logging.LevelFromString(viper.GetString(conf.LogLevel)) == logging.LevelDebug,
		),
		server.WithProfiling(viper.GetBool(conf.ProfilingEnabled)),
		server.WithTracing(viper.GetBool(tracing.TracingEnabledArg)),
	)

	// initializing the server in a goroutine so that it won't block the graceful
	// shutdown handling below
	logger.With("addr", addr).Info("starting API server")
	go func() {
		err = apiServer.Serve()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("listen: %v", err)
		}
	}()

	// listen for the interrupt signal
	<-ctx.Done()

	// restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	logger.Info("shutting down server gracefully")

	// the context is used to inform the server it has ShutdownGracePeriod to wrap up the requests it is
	// currently handling
	ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration(conf.ServerShutdownGracePeriod))
	defer cancel()

	// shut down running resources, forcibly if need be
	err = apiServer.Shutdown(ctx)
	if err != nil {
		logger.With("error", err).Error("forced shut down of API server")
	}
	err = shutdownTracing(ctx)
	if err != nil {
		logger.With("error", err).Error("forced shut down of tracing")
	}

	logger.Info("shut down complete")
	return nil
}
