package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/els0r/goProbe/pkg/query"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cobra"
)

var supportedCmds = "{QUERY TYPE|COLUMNS|admin|examples|list|version}"

var rootCmd = &cobra.Command{
	Use:   "goQuery [flags] [" + supportedCmds + "]",
	Short: helpBase,
	Long:  helpBaseLong,
	// entry point for goQuery
	RunE:          entrypoint,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// any commands other than query type will be hooked up to this command
var subRootCmd = &cobra.Command{}

// Execute is the main entrypoint and runs the CLI tool
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// globally accessible variable for other packages
var (
	cmdLineParams    = &query.Args{}
	subcmdLineParams = &query.Args{}
	argsLocation     string // for stored queries
)

func init() {

	// flags to be also passed to children commands
	subRootCmd.PersistentFlags().StringVarP(&subcmdLineParams.DBPath, "db-path", "d", query.DefaultDBPath, helpMap["DBPath"])
	subRootCmd.PersistentFlags().BoolVarP(&subcmdLineParams.External, "external", "x", false, helpMap["External"])

	// help commands
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultHelpFlag()

	subRootCmd.InitDefaultHelpCmd()
	subRootCmd.InitDefaultHelpFlag()

	rootCmd.Flags().BoolVarP(&cmdLineParams.External, "external", "x", false, helpMap["External"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.In, "in", "", query.DefaultIn, helpMap["In"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.List, "list", "", false, helpMap["List"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Out, "out", "", query.DefaultOut, helpMap["Out"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Resolve, "resolve", "", false, helpMap["Resolve"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.SortAscending, "ascending", "a", false, helpMap["SortAscending"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Sum, "sum", "", false, helpMap["Sum"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Version, "version", "v", false, "Print version information and exit\n")

	// Strings
	rootCmd.Flags().StringVarP(&cmdLineParams.Condition, "condition", "c", "", helpMap["Condition"])
	rootCmd.Flags().StringVarP(&cmdLineParams.DBPath, "db-path", "d", query.DefaultDBPath, helpMap["DBPath"])
	rootCmd.Flags().StringVarP(&cmdLineParams.First, "first", "f", time.Now().AddDate(0, -1, 0).Format(time.ANSIC), helpMap["First"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Format, "format", "e", query.DefaultFormat, helpMap["Format"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Ifaces, "ifaces", "i", "", helpMap["Ifaces"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Last, "last", "l", time.Now().Format(time.ANSIC), "Show flows no later than --last. See help for --first for more info\n")
	rootCmd.Flags().StringVarP(&cmdLineParams.Output, "set-output", "o", "", helpMap["Output"])
	rootCmd.Flags().StringVarP(&argsLocation, "stored-query", "", "", "Load JSON serialized query arguments from disk and run them")
	rootCmd.Flags().StringVarP(&cmdLineParams.SortBy, "sort-by", "s", query.DefaultSortBy, helpMap["SortBy"])

	// Integers
	rootCmd.Flags().IntVarP(&cmdLineParams.NumResults, "limit", "n", query.DefaultNumResults, helpMap["NumResults"])
	rootCmd.Flags().IntVarP(&cmdLineParams.ResolveRows, "resolve-rows", "", query.DefaultResolveRows, helpMap["ResolveRows"])
	rootCmd.Flags().IntVarP(&cmdLineParams.ResolveTimeout, "resolve-timeout", "", query.DefaultResolveTimeout, helpMap["ResolveTimeout"])
	rootCmd.Flags().IntVarP(&cmdLineParams.MaxMemPct, "max-mem", "", query.DefaultMaxMemPct, helpMap["MaxMemPct"])
}

// main program entrypoint
func entrypoint(cmd *cobra.Command, args []string) error {

	// assign query args
	var queryArgs = *cmdLineParams

	// run commands that don't require any argument
	// handle list flag
	if cmdLineParams.List {
		err := listInterfaces(cmdLineParams.DBPath, cmdLineParams.External)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve list of available databases: %s", err)
			return err
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
	if argsLocation != "" {
		argumentsJSON, err := ioutil.ReadFile(argsLocation)
		if err != nil {
			err := fmt.Errorf("Failed to read query args from %s", argsLocation)
			fmt.Fprintf(os.Stderr, "%s", err)
			return err
		}
		// unmarshal arguments into the command line parameters
		err = json.Unmarshal(argumentsJSON, &queryArgs)
		if err != nil {
			err := fmt.Errorf("Failed to parse JSON query args %s", err)
			fmt.Fprintf(os.Stderr, "%s", err)
			return err
		}
	} else {
		// check that query type or other subcommands were provided
		if len(args) == 0 {
			err := errors.New("No query type or command provided")
			fmt.Fprintf(os.Stderr, "%s\n%s\n", err, cmd.Long)
			return err
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
				err := c.Execute()
				return err
			}
		}

		// if we didn't find a supported command, we assume this is the query type
		queryArgs.Query = args[0]
	}

	queryArgs.Caller = os.Args[0] // take the full path of called binary

	// convert the command line parameters
	query, err := queryArgs.Prepare()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query preparation failed: %v\n", err)
		return err
	}

	ctx := context.Background()

	// run the query
	result, err := query.Execute(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query execution failed: %v\n", err)
		return err
	}

	// serialize raw result if json is selected
	if query.Format == "json" {
		err = jsoniter.NewEncoder(query.Output).Encode(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Results serialization failed: %v\n", err)
			return err
		}
		return nil
	}

	return query.Print(ctx, result)
}
