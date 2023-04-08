package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/els0r/goProbe/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
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

var (
	cmdLineParams = &query.Args{}
	argsLocation  string
	hostQuery     string
)

var shortText = "Query distributed goDBs and aggregate the results"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "global-query",
	Short:         shortText,
	Long:          shortText,
	RunE:          entrypoint,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogger)

	// help commands
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultHelpFlag()

	rootCmd.Flags().BoolVarP(&cmdLineParams.In, "in", "", query.DefaultIn, helpMap["In"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Out, "out", "", query.DefaultOut, helpMap["Out"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.DNSResolution.Enabled, "resolve", "", false, helpMap["Resolve"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.SortAscending, "ascending", "a", false, helpMap["SortAscending"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Sum, "sum", "", false, helpMap["Sum"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Version, "version", "v", false, "Print version information and exit\n")

	// Strings
	rootCmd.Flags().StringVarP(&cmdLineParams.Condition, "condition", "c", "", helpMap["Condition"])
	rootCmd.Flags().StringVarP(&cmdLineParams.First, "first", "f", time.Now().AddDate(0, -1, 0).Format(time.ANSIC), helpMap["First"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Format, "format", "e", query.DefaultFormat, helpMap["Format"])
	rootCmd.Flags().StringVarP(&hostQuery, "hosts", "", "", "comma-separated list of hosts to query")
	rootCmd.Flags().StringVarP(&cmdLineParams.Ifaces, "ifaces", "i", "", helpMap["Ifaces"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Last, "last", "l", time.Now().Format(time.ANSIC), "Show flows no later than --last. See help for --first for more info\n")
	rootCmd.Flags().StringVarP(&argsLocation, "stored-query", "", "", "Load JSON serialized query arguments from disk and run them")
	rootCmd.Flags().StringVarP(&cmdLineParams.SortBy, "sort-by", "s", query.DefaultSortBy, helpMap["SortBy"])

	// Integers
	rootCmd.Flags().IntVarP(&cmdLineParams.NumResults, "limit", "n", query.DefaultNumResults, helpMap["NumResults"])
	rootCmd.Flags().IntVarP(&cmdLineParams.DNSResolution.MaxRows, "resolve-rows", "", query.DefaultResolveRows, helpMap["ResolveRows"])
	rootCmd.Flags().DurationVarP(&cmdLineParams.DNSResolution.Timeout, "resolve-timeout", "", query.DefaultResolveTimeout, helpMap["ResolveTimeout"])
	rootCmd.Flags().IntVarP(&cmdLineParams.MaxMemPct, "max-mem", "", query.DefaultMaxMemPct, helpMap["MaxMemPct"])

	// Duration
	rootCmd.Flags().DurationVarP(&cmdLineParams.QueryTimeout, "timeout", "", query.DefaultQueryTimeout, helpMap["QueryTimeout"])

	rootCmd.Flags().String(conf.LogLevel, conf.DefaultLogLevel, "log level for logger")
	rootCmd.Flags().String(conf.LogEncoding, conf.DefaultLogEncoding, "message encoding format for logger")

	rootCmd.Flags().String(conf.HostsResolverType, conf.DefaultHostsResolver, "resolver used for the hosts resolution query")

	rootCmd.Flags().String(conf.HostsQuerierType, conf.DefaultHostsQuerierType, "querier used to run queries")
	rootCmd.Flags().String(conf.HostsQuerierConfig, "", "querier config file location")

	rootCmd.Flags().StringVarP(&hostQuery, "hosts-query", "q", "", "hosts resolution query")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.global-query.yaml)")

	_ = viper.BindPFlags(rootCmd.Flags())
}

func initLogger() {
	outputPaths := []string{"stderr"}

	// since this is a command line tool, only warnings and errors should be printed and they
	// shouldn't go to a dedicated file
	err := logging.Init("global-query", version.Short(), viper.GetString(conf.LogLevel), "logfmt",
		logging.WithOutputPaths(outputPaths),
		logging.WithErrorPaths(outputPaths),
		logging.WithStackTraces(false),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cmd" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cmd")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read in config: %v\n", err)
		os.Exit(1)
	}
}

func initHostListResolver() (hosts.Resolver, error) {
	resolverType := viper.GetString(conf.HostsResolverType)
	switch resolverType {
	case string(hosts.StringResolverType):
		return hosts.NewStringResolver(true), nil
	default:
		err := fmt.Errorf("hosts resolver type %q not supported", resolverType)
		return nil, err
	}
}

func initQuerier() (hosts.Querier, error) {
	querierType := viper.GetString(conf.HostsQuerierType)
	switch querierType {
	case string(hosts.APIClientQuerierType):
		return hosts.NewAPIClientQuerier(viper.GetString(conf.HostsQuerierConfig))
	default:
		err := fmt.Errorf("querier type %q not supported", querierType)
		return nil, err
	}
}

const (
	queryHostname = "hostname"
	queryHostID   = "hostid"
)

func entrypoint(cmd *cobra.Command, args []string) error {
	logger := logging.Logger()

	// assign query args
	var queryArgs = *cmdLineParams

	// check that query type or other subcommands were provided
	if len(args) == 0 {
		err := errors.New("No query type or command provided")
		fmt.Fprintf(os.Stderr, "%v\n%s\n", err, cmd.Long)
		return err
	}
	if args[0] == "help" {
		cmd.Help()
		return nil
	}

	// store the query type and make sure that aliases are resolved. This
	// is important such that the hostname/hostid can be appended
	queryArgs.Query = strings.Join(types.ToAttributeNames(args[0]), ",")

	// make sure that the hostname is present in the query type (and therefore output)
	// The assumption being that a human will have better knowledge
	// of hostnames than of their ID counterparts
	if queryArgs.Format == "txt" {
		if !strings.Contains(queryArgs.Query, queryHostname) {
			queryArgs.Query += "," + queryHostname
		}
	}

	// check if the statement can be created
	stmt, err := queryArgs.Prepare()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to prepare query statement: %v\n", err)
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer stop()

	// parse host list
	if hostQuery == "" {
		err := fmt.Errorf("list of target hosts is empty")
		fmt.Fprintf(os.Stderr, "Couldn't prepare query: %v\n", err)
		return err
	}

	hostListResolver, err := initHostListResolver()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't prepare query: %v\n", err)
		return err
	}
	hostList, err := hostListResolver.Resolve(ctx, hostQuery)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve host list: %v\n", err)
		return err
	}

	// log the query
	qlogger := logger.With("hosts", hostList)

	b, err := json.Marshal(queryArgs)
	if err == nil {
		qlogger = qlogger.With("query", string(b))
	}
	qlogger.Infof("setting up queriers")

	// get the workload provider
	querier, err := initQuerier()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up queriers: %v\n", err)
		return err
	}

	// query pipeline setup
	// sets up a fan-out, fan-in query processing pipeline
	numRunners := len(hostList)

	finalResult, statusTracker := hosts.AggregateResults(ctx, stmt,
		hosts.RunQueries(ctx, numRunners,
			hosts.PrepareQueries(ctx, querier, hostList, &queryArgs),
		),
	)

	// truncate results based on the limit
	finalResult.End()

	if queryArgs.NumResults < len(finalResult.Rows) {
		finalResult.Rows = finalResult.Rows[:queryArgs.NumResults]
	}
	finalResult.Summary.Hits.Displayed = len(finalResult.Rows)

	// serialize raw result if json is selected
	if stmt.Format == "json" {
		err = jsoniter.NewEncoder(stmt.Output).Encode(finalResult)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Results serialization failed: %v\n", err)
			return err
		}
		return nil
	}

	if finalResult.Status.Code != types.StatusOK {
		fmt.Fprintf(stmt.Output, "Status %q: %s\n", finalResult.Status.Code, finalResult.Status.Message)
	} else {
		err = stmt.Print(ctx, finalResult)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Result printing failed: %v\n", err)
			return err
		}
	}

	// print all tracker information
	statusTracker.PrintErrorHosts(stmt.Output)

	return nil
}

type nopRunner struct{}

func (n *nopRunner) Run(_ context.Context, _ *query.Statement) (res []results.Result, err error) {
	fmt.Println("running nop query")
	return res, err
}
