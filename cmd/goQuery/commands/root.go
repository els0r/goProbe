package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/goProbe/pkg/version"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var supportedCmds = "{QUERY TYPE|COLUMNS|admin|examples|list|version}"

var rootCmd = &cobra.Command{
	Use:           "goQuery [flags] [" + supportedCmds + "]",
	Short:         helpBase,
	Long:          helpBaseLong,
	RunE:          entrypoint,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// any commands other than query type will be hooked up to this command
var subRootCmd = &cobra.Command{}

// Execute is the main entrypoint and runs the CLI tool
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		cliLogger.Fatalf("Error running query: %s", err)
	}
}

// globally accessible variable for other packages
var (
	cmdLineParams = &query.Args{}
	argsLocation  string // for stored queries
)

func init() {
	initCLILogger()
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogger)

	// flags to be also passed to children commands
	subRootCmd.PersistentFlags().StringP(conf.DBPath, "d", defaults.DBPath, helpMap["DBPath"])
	_ = viper.BindPFlags(subRootCmd.PersistentFlags())

	// help commands
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultHelpFlag()

	subRootCmd.InitDefaultHelpCmd()
	subRootCmd.InitDefaultHelpFlag()

	rootCmd.Flags().BoolVarP(&cmdLineParams.In, "in", "", query.DefaultIn, helpMap["In"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.List, "list", "", false, helpMap["List"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Out, "out", "", query.DefaultOut, helpMap["Out"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.SortAscending, "ascending", "a", false, helpMap["SortAscending"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Sum, "sum", "", false, helpMap["Sum"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Version, "version", "v", false, "Print version information and exit\n")

	rootCmd.Flags().StringVarP(&cmdLineParams.Ifaces, "ifaces", "i", "", helpMap["Ifaces"])
	rootCmd.Flags().StringVarP(&cmdLineParams.First, "first", "f", time.Now().AddDate(0, -1, 0).Format(time.ANSIC), helpMap["First"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Last, "last", "l", time.Now().Format(time.ANSIC), "Show flows no later than --last. See help for --first for more info\n")
	rootCmd.Flags().StringVarP(&cmdLineParams.Condition, "condition", "c", "", helpMap["Condition"])
	rootCmd.Flags().IntVarP(&cmdLineParams.NumResults, "limit", "n", query.DefaultNumResults, helpMap["NumResults"])
	rootCmd.Flags().StringVarP(&cmdLineParams.SortBy, "sort-by", "s", query.DefaultSortBy, helpMap["SortBy"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Format, "format", "e", query.DefaultFormat, helpMap["Format"])

	rootCmd.Flags().BoolVarP(&cmdLineParams.DNSResolution.Enabled, "resolve", "", false, helpMap["Resolve"])
	rootCmd.Flags().IntVarP(&cmdLineParams.DNSResolution.MaxRows, "resolve-rows", "", query.DefaultResolveRows, helpMap["ResolveRows"])
	rootCmd.Flags().DurationVarP(&cmdLineParams.DNSResolution.Timeout, "resolve-timeout", "", query.DefaultResolveTimeout, helpMap["ResolveTimeout"])

	rootCmd.Flags().IntVarP(&cmdLineParams.MaxMemPct, "max-mem", "", query.DefaultMaxMemPct, helpMap["MaxMemPct"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.LowMem, "low-mem", "", false, helpMap["LowMem"])

	rootCmd.Flags().DurationVarP(&cmdLineParams.QueryTimeout, "timeout", "", query.DefaultQueryTimeout, helpMap["QueryTimeout"])

	rootCmd.PersistentFlags().String(conf.QueryServerAddr, "", helpMap["QueryServer"])
	rootCmd.Flags().StringVarP(&cmdLineParams.HostQuery, "hosts-query", "q", "", "hosts resolution query")

	rootCmd.PersistentFlags().String(conf.QueryDBPath, "", helpMap["DBPath"])

	rootCmd.PersistentFlags().String(conf.StoredQuery, "", "Load JSON serialized query arguments from disk and run them")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file location")

	_ = viper.BindPFlags(rootCmd.PersistentFlags())
}

var cliLogger *logging.L

func initCLILogger() {
	var err error
	cliLogger, err = logging.New(logging.LevelError, logging.EncodingPlain,
		logging.WithOutput(os.Stderr),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to instantiate CLI logger: %v\n", err)
		os.Exit(1)
	}
}

func initLogger() {
	// since this is a command line tool, only warnings and errors should be printed and they
	// shouldn't go to a dedicated file
	err := logging.Init(logging.LevelWarn, logging.EncodingLogfmt,
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
func entrypoint(cmd *cobra.Command, args []string) error {
	// assign query args
	var queryArgs = *cmdLineParams

	// the DB path that can be set in the configuration file has precedence over the one
	// in the arguments
	dbPathCfg := viper.GetString(conf.DBPath)
	if dbPathCfg != "" {
		dbPathCfg = viper.GetString(conf.QueryDBPath)
	}

	// run commands that don't require any argument
	// handle list flag
	if cmdLineParams.List {
		err := listInterfaces(queryArgs.DBPath)
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
	if viper.GetString(conf.StoredQuery) != "" {
		argumentsJSON, err := os.ReadFile(filepath.Clean(argsLocation))
		if err != nil {
			return fmt.Errorf("failed to read query args from %s: %w", argsLocation, err)
		}
		// unmarshal arguments into the command line parameters
		if err = json.Unmarshal(argumentsJSON, &queryArgs); err != nil {
			return fmt.Errorf("failed to unmarshal JSON query args %s: %w", string(argumentsJSON), err)
		}
	} else {
		// check that query type or other subcommands were provided
		if len(args) == 0 {
			return errors.New("no query type or command provided")
		}
		if args[0] == "help" {
			cmd.Help()
			return nil
		}

		// attach subcommands
		subRootCmd.AddCommand(
			adminCmd,
			exampleCmd,
			listCmd,
			versionCmd,
		)

		// execute subcommands if possible
		for _, c := range subRootCmd.Commands() {
			if c.Name() == args[0] {
				c.SetArgs(args[1:])
				return c.Execute()
			}
		}

		// if we didn't find a supported command, we assume this is the query type
		queryArgs.Query = args[0]
	}

	queryCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer stop()

	var ctx context.Context
	if cmdLineParams.QueryTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(queryCtx, cmdLineParams.QueryTimeout)
		defer cancel()
	} else {
		ctx = queryCtx
	}

	queryArgs.Caller = os.Args[0] // take the full path of called binary

	// run the query
	var result *results.Result

	// run query against query server if it is specified, otherwise, take the local DB
	var querier query.Runner
	if viper.GetString(conf.QueryServerAddr) != "" {
		if queryArgs.HostQuery == "" {
			err := fmt.Errorf("list of target hosts is empty")
			fmt.Fprintf(os.Stderr, "Distributed query preparation failed: %v\n", err)
			return err
		}

		// store the query type and make sure that aliases are resolved. This
		// is important so that the hostname/hostid can be appended
		queryArgs.Query = strings.Join(types.ToAttributeNames(queryArgs.Query), ",")

		// make sure that the hostname is present in the query type (and therefore output)
		// The assumption being that a human will have better knowledge
		// of hostnames than of their ID counterparts
		if queryArgs.Format == "txt" {
			if !strings.Contains(queryArgs.Query, types.HostnameName) {
				queryArgs.Query += "," + types.HostnameName
			}
		}

		querier = client.New(viper.GetString(conf.QueryServerAddr))
	} else {
		querier = engine.NewQueryRunner(dbPathCfg)
	}

	// convert the command line parameters
	stmt, err := queryArgs.Prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
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
		fmt.Fprintf(stmt.Output, "Status %q: %s\n", result.Status.Code, result.Status.Message)
		return nil
	}

	if err = stmt.Print(ctx, result); err != nil {
		return fmt.Errorf("failed to print query result: %w", err)
	}
	return nil
}
