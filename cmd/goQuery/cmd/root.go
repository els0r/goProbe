package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/els0r/goProbe/cmd/goQuery/pkg/conf"
	"github.com/els0r/goProbe/pkg/api/globalquery/client"
	"github.com/els0r/goProbe/pkg/defaults"
	"github.com/els0r/goProbe/pkg/goDB/engine"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/version"
	"github.com/els0r/telemetry/logging"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// TODO: This part is currently unused - cross check if that is intentional (in which case it can be removed)
// var supportedCmds = "{QUERY TYPE|COLUMNS|examples|list|version}"

var rootCmd = &cobra.Command{
	Use:   "goQuery -i <interfaces> QUERY TYPE",
	Short: helpBase,
	Long:  helpBaseLong,
	// we want to make sure that every command can be profiled
	PersistentPreRunE:  initProfiling,
	RunE:               entrypoint,
	PersistentPostRunE: finishProfiling,
	Args:               validatePositionalArgs,
	SilenceUsage:       true,
	SilenceErrors:      true,
}

// GetRootCmd allows access to the root command of the binary
func GetRootCmd() *cobra.Command {
	return rootCmd
}

// this is a necessary re-routing, so that the tool can handle commands other than query
// without complaining that that something like "sip,dip" cannot be found as a command
func validatePositionalArgs(cmd *cobra.Command, args []string) error {
	return cobra.ArbitraryArgs(cmd, args)
}

// Execute is the main entrypoint and runs the CLI tool
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		logger, logErr := logging.New(logging.LevelError, logging.EncodingPlain,
			logging.WithOutput(os.Stderr),
		)
		if logErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to instantiate CLI logger: %v\n", logErr)

			fmt.Fprintf(os.Stderr, "Error running query: %s\n", err)
			os.Exit(1)
		}
		logger.Fatalf("Error running query: %s", err)
	}
}

// globally accessible variable for other packages
var (
	cmdLineParams = &query.Args{}
)

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogger)

	// help commands
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultHelpFlag()

	flags := rootCmd.Flags()
	pflags := rootCmd.PersistentFlags()

	flags.BoolVarP(&cmdLineParams.In, "in", "", query.DefaultIn, helpMap["In"])
	flags.BoolVarP(&cmdLineParams.Out, "out", "", query.DefaultOut, helpMap["Out"])
	flags.BoolVarP(&cmdLineParams.Sum, "sum", "", false, helpMap["Sum"])
	flags.BoolVarP(&cmdLineParams.Version, "version", "v", false, "Print version information and exit\n")

	flags.StringVarP(&cmdLineParams.Ifaces, "ifaces", "i", "", helpMap["Ifaces"])
	flags.StringVarP(&cmdLineParams.Condition, "condition", "c", "", helpMap["Condition"])

	flags.StringVarP(&cmdLineParams.SortBy, conf.SortBy, "s", query.DefaultSortBy,
		`Sort results by given column name:
  bytes         Sort by accumulated data volume (default)
  packets       Sort by accumulated packets
  time          Sort by time. Enforced for "time" queries
`,
	)
	flags.BoolVarP(&cmdLineParams.SortAscending, conf.SortAscending, "a", false,
		`Sort results in ascending instead of descending order. Forced for queries
including the "time" field.
`,
	)

	flags.Uint64VarP(&cmdLineParams.NumResults, conf.ResultsLimit, "n", query.DefaultNumResults,
		`Maximum number of final entries to show. Defaults to 95% of the overall
data volume / number of packets (depending on the '-s' parameter).
Ignored for queries including the "time" field.
`,
	)

	flags.BoolVarP(&cmdLineParams.DNSResolution.Enabled, conf.DNSResolutionEnabled, "r", false,
		`Resolve top IPs in output using reverse DNS lookups.
If the reverse DNS lookup for an IP fails, the IP is shown instead.
The lookup is performed for the first '--resolve-rows' rows
of output.
Beware: The lookup is carried out at query time; DNS data may have been
different when the packets were captured.
`,
	)
	flags.IntVar(&cmdLineParams.DNSResolution.MaxRows, conf.DNSResolutionMaxRows, query.DefaultResolveRows,
		`Maximum number of output rows to perform DNS resolution against. Before
setting this to some high value (e.g. 1000), consider that this may incur
a high load on the DNS resolver and network!
`,
	)
	flags.DurationVar(&cmdLineParams.DNSResolution.Timeout, conf.DNSResolutionTimeout, query.DefaultResolveTimeout,
		"Timeout in seconds for (reverse) DNS lookups\n",
	)

	flags.IntVar(&cmdLineParams.MaxMemPct, conf.MemoryMaxPct, query.DefaultMaxMemPct,
		`Maximum amount of memory that can be used for the query
(in % of available memory)
`,
	)
	flags.BoolVar(&cmdLineParams.LowMem, conf.MemoryLowMode, false,
		`Enable low-memory mode (reduces overall memory use at the expense of higher CPU
and I/O load)
`,
	)
	flags.StringVarP(&cmdLineParams.QueryHosts, conf.QueryHostsResolution, "q", "", "Hosts resolution query\n")

	// persistent flags to be also passed to children commands
	pflags.String(conf.ProfilingOutputDir, "", "Enable and set directory to store CPU and memory profiles")

	pflags.StringVarP(&cmdLineParams.Format, conf.ResultsFormat, "e", query.DefaultFormat,
		`Output format:
  txt           Output in plain text format (default)
  json          Output in JSON format
  csv           Output in comma-separated table format
`,
	)

	// the time parameter should be available to commands other than query
	pflags.StringVarP(&cmdLineParams.First, conf.First, "f", "", helpMap["First"])
	pflags.StringVarP(&cmdLineParams.Last, conf.Last, "l", "", "Show flows no later than --last. See help for --first for more info\n")

	pflags.String(conf.QueryServerAddr, "",
		`Address of query server to run queries against (host:port). If this value is
set, goQuery will attempt to run queries using the specified query server as opposed to its local goDB
`,
	)
	pflags.StringP(conf.QueryDBPath, "d", defaults.DBPath,
		`Path to goDB database directory. By default,
the database path from the configuration file is used.
If it does not exist, an error will be thrown.

This also implies that you have to explicitly specify
the path if you analyze data on a different host without
goProbe.
`,
	)
	pflags.String(conf.StoredQuery, "", "Load JSON serialized query arguments from disk and run them\n")
	pflags.Duration(conf.QueryTimeout, query.DefaultQueryTimeout, "Abort query processing after timeout expires\n")
	pflags.String(conf.QueryLog, "", "Log query invocations to file\n")

	pflags.String(conf.LogLevel, logging.LevelWarn.String(), "log level (debug, info, warn, error, fatal, panic)")

	pflags.StringVar(&cfgFile, "config", "", "Config file location\n")

	_ = viper.BindPFlags(pflags)
}

func initLogger() {
	// since this is a command line tool, only warnings and errors should be printed and they
	// shouldn't go to a dedicated file
	err := logging.Init(logging.LevelFromString(viper.GetString(conf.LogLevel)), logging.EncodingLogfmt,
		logging.WithVersion(version.Short()),
		logging.WithOutput(os.Stdout),
		logging.WithErrorOutput(os.Stderr),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set. goQuery doesn't need one to run
// as a CLI tool. The functionality exists to set some defaults for e.g. the query-server
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)

		viper.AutomaticEnv() // read in environment variables that match

		// If a config file is found, read it in.
		if err := viper.ReadInConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read in config: %v\n", err)
			os.Exit(1)
		}
	}
}

// main program entrypoint
func entrypoint(cmd *cobra.Command, args []string) (err error) {
	// assign query args
	var queryArgs = *cmdLineParams

	// the DB path that can be set in the configuration file has precedence over the one
	// in the arguments
	dbPathCfg := viper.GetString(conf.QueryDBPath)

	// run commands that don't require any argument
	// handle list flag
	if cmdLineParams.List {
		err := listInterfaces(dbPathCfg)
		if err != nil {
			return fmt.Errorf("failed to retrieve list of available databases: %w", err)
		}
		return nil
	}

	// print version and exit
	if cmdLineParams.Version {
		printVersion()
		return nil
	}

	// check if arguments should be loaded from disk. The cmdLineParams are taken as
	// the base for this to allow modification of single parameters
	argsLocation := viper.GetString(conf.StoredQuery)
	if argsLocation != "" {
		var argsReader io.Reader

		argsReader = os.Stdin
		if argsLocation != "-" {
			f, err := os.Open(filepath.Clean(argsLocation))
			if err != nil {
				return fmt.Errorf("failed to open query args from %s: %w", argsLocation, err)
			}
			argsReader = f
		}

		// unmarshal arguments into the command line parameters
		if err = jsoniter.NewDecoder(argsReader).Decode(&queryArgs); err != nil {
			return fmt.Errorf("failed to unmarshal JSON query args: %w", err)
		}
	} else {
		// check that query type or other subcommands were provided
		if len(args) == 0 {
			return errors.New("no query type or command provided")
		}
		if args[0] == "help" {
			return cmd.Help()
		}

		// if we didn't find a supported command, we assume this is the query type
		queryArgs.Query = args[0]
	}

	// make sure there's protection against unbounded time intervals
	queryArgs = setDefaultTimeRange(&queryArgs)

	queryCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer stop()

	var ctx context.Context
	queryTimeout := viper.GetDuration(conf.QueryTimeout)
	if queryTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(queryCtx, queryTimeout)
		defer cancel()
	} else {
		ctx = queryCtx
	}

	// get logger
	logger := logging.FromContext(ctx)

	queryArgs.Caller = os.Args[0] // take the full path of called binary

	// run the query
	var result *results.Result

	// run query against query server if it is specified, otherwise, take the local DB
	var querier query.Runner
	if viper.GetString(conf.QueryServerAddr) != "" {
		if queryArgs.QueryHosts == "" {
			err := fmt.Errorf("list of target hosts is empty")
			fmt.Fprintf(os.Stderr, "Distributed query preparation failed: %v\n", err)
			return err
		}

		// store the query type and make sure that aliases are resolved. This
		// is important so that the hostname/hostid can be appended
		queryArgs.Query = types.SanitizeQueryType(queryArgs.Query)

		// make sure that the hostname is present in the query type (and therefore output)
		// The assumption being that a human will have better knowledge
		// of hostnames than of their ID counterparts
		if queryArgs.Format == "txt" {
			if !strings.Contains(queryArgs.Query, types.HostnameName) {
				queryArgs.Query += types.AttrSep + types.HostnameName
			}
		}

		// query using query server
		querier = client.New(viper.GetString(conf.QueryServerAddr))
	} else {
		// query using local goDB
		querier = engine.NewQueryRunner(dbPathCfg)
	}

	// create query logger
	queryLogFile := viper.GetString(conf.QueryLog)
	var qlogger *logging.L
	if queryLogFile != "" {
		var err error

		logger := logger.With("file", queryLogFile)
		logger.Debugf("logging query")

		qlogger, err = logging.New(slog.LevelInfo, logging.EncodingJSON, logging.WithFileOutput(queryLogFile))
		if err != nil {
			logger.Errorf("failed to initialize query logger: %v", err)
		} else {
			qlogger.With("args", queryArgs).Infof("preparing query")
			defer func() {
				if result != nil {
					qlogger = qlogger.With("summary", result.Summary)
				}
				qlogger.Info("query finished")
			}()
		}
	}

	// convert the command line parameters
	stmt, err := queryArgs.Prepare()
	if err != nil {
		msg := "failed to prepare query"

		// if there's an args error, print it in a user-friendly way
		prettyErr, ok := err.(types.Prettier)
		if ok {
			return fmt.Errorf("%s:\n%s", msg, prettyErr.Pretty())
		}
		return fmt.Errorf("%s: %w", msg, err)
	}

	if queryLogFile != "" {
		if qlogger != nil {
			qlogger.With("stmt", stmt).Info("running query")
		}
	}

	result, err = querier.Run(ctx, &queryArgs)
	if err != nil {
		return fmt.Errorf("failed to execute query %s: %w", stmt, err)
	}

	// serialize raw results array if json is selected
	if stmt.Format == "json" {
		err = jsoniter.NewEncoder(stmt.Output).Encode(result)
		if err != nil {
			return fmt.Errorf("failed to serialize query results: %w", err)
		}
		return nil
	}

	// when running against a local goDB, there should be exactly one result
	if result.Status.Code != types.StatusOK {
		logger, err := logging.New(logging.LevelInfo, logging.EncodingPlain,
			logging.WithOutput(stmt.Output),
		)
		if err != nil {
			return err
		}
		logger.Infof("Status %q: %s", result.Status.Code, result.Status.Message)
		return nil
	}

	err = stmt.Print(ctx, result)
	if err != nil {
		return fmt.Errorf("failed to print query result: %w", err)
	}
	return nil
}

// setDefaultTimeRange handles the defaults for time arguments if they aren't set
func setDefaultTimeRange(args *query.Args) query.Args {
	logger := logging.Logger()
	if args.First == "" {
		logger.Debug("setting default value for 'first'")

		// protect against queries that are possibly too large and only go back a day if a time attribute
		// is included. This is only done if first wasn't explicitly set. If it is, it must be assumed that
		// the caller knows the possible extend of a "time" query
		if strings.Contains(args.Query, types.TimeName) || strings.Contains(args.Query, types.RawCompoundQuery) {
			logger.With("query", args.Query).Debug("time attribute detected, limiting time range to one day")
			args.First = time.Now().AddDate(0, 0, -1).Format(time.ANSIC)
		} else {
			// by default, go back one month in time
			args.First = time.Now().AddDate(0, -1, 0).Format(time.ANSIC)
		}
	}
	if args.Last == "" {
		logger.Debug("setting default value for 'last'")
		args.Last = time.Now().Format(time.ANSIC)
	}
	return *args
}
