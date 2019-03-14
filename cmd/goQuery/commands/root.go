package commands

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/els0r/goProbe/pkg/query"
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
	cmdLineParams    *query.Args
	subcmdLineParams *query.Args
)

func init() {
	cmdLineParams = &query.Args{}
	subcmdLineParams = &query.Args{}

	// flags to be also passed to children commands
	subRootCmd.PersistentFlags().StringVarP(&subcmdLineParams.DBPath, "db-path", "d", "/opt/ntm/goProbe/db", helpMap["DBPath"])
	subRootCmd.PersistentFlags().BoolVarP(&subcmdLineParams.External, "external", "x", false, helpMap["External"])

	// Booleans
	rootCmd.LocalFlags().BoolP("help", "h", false, helpMap["Help"])

	rootCmd.Flags().BoolVarP(&cmdLineParams.External, "external", "x", false, helpMap["External"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.In, "in", "", true, helpMap["In"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.List, "list", "", false, helpMap["List"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Out, "out", "", true, helpMap["Out"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Resolve, "resolve", "", false, helpMap["Resolve"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.SortAscending, "ascending", "a", false, helpMap["SortAscending"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Sum, "sum", "", false, helpMap["Sum"])
	rootCmd.Flags().BoolVarP(&cmdLineParams.Version, "version", "v", false, "Print version information and exit")

	// Strings
	rootCmd.Flags().StringVarP(&cmdLineParams.Condition, "condition", "c", "", helpMap["Condition"])
	rootCmd.Flags().StringVarP(&cmdLineParams.DBPath, "db-path", "d", "/opt/ntm/goProbe/db", helpMap["DBPath"])
	rootCmd.Flags().StringVarP(&cmdLineParams.First, "first", "f", time.Now().AddDate(0, -1, 0).Format(time.ANSIC), helpMap["First"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Format, "format", "e", "txt", helpMap["Format"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Ifaces, "ifaces", "i", "", helpMap["Ifaces"])
	rootCmd.Flags().StringVarP(&cmdLineParams.Last, "last", "l", time.Now().Format(time.ANSIC), "Show flows no later than --last. See help for --first for more info\n")
	rootCmd.Flags().StringVarP(&cmdLineParams.Output, "set-output", "o", "", helpMap["Output"])
	rootCmd.Flags().StringVarP(&cmdLineParams.SortBy, "sort-by", "s", "bytes", helpMap["SortBy"])

	// Integers
	rootCmd.Flags().IntVarP(&cmdLineParams.NumResults, "limit", "n", 1000, helpMap["NumResults"])
	rootCmd.Flags().IntVarP(&cmdLineParams.ResolveRows, "resolve-rows", "", 25, helpMap["ResolveRows"])
	rootCmd.Flags().IntVarP(&cmdLineParams.ResolveTimeout, "resolve-timeout", "", 1, helpMap["ResolveTimeout"])
	rootCmd.Flags().IntVarP(&cmdLineParams.MaxMemPct, "max-mem", "", query.MaxMemPctDefault, helpMap["MaxMemPct"])
}

// main program entrypoint
func entrypoint(cmd *cobra.Command, args []string) error {

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
	cmdLineParams.Query = args[0]
	cmdLineParams.Caller = os.Args[0] // take the full path of called binary

	// convert the command line parameters
	query, err := cmdLineParams.Prepare()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query preparation failed: %s\n", err)
		return err
	}

	// run the query
	err = query.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query execution failed: %s\n", err)
		return err
	}
	return nil
}
