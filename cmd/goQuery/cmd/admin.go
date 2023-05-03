package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/els0r/goProbe/cmd/goQuery/pkg/conf"
	"github.com/els0r/goProbe/pkg/goDB/info"
	"github.com/els0r/goProbe/pkg/goDB/storage/gpfile"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/types"
	"github.com/els0r/status"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var adminCmd = &cobra.Command{
	Use:   "admin [command]",
	Short: `Advanced maintenance options (should not be used in interactive mode).`,
	Long:  adminHelp,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("admin requires a sub-command as argument")
		}
		return nil
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean [date]",
	Short: "Clean the database by removing all files before [date]",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("clean requires exactly one date as argument")
		}
		// convert date into timestamp
		tClean, err := query.ParseTimeArgument(args[0])
		if err != nil {
			return fmt.Errorf("failed to set clean date: %s", err)
		}

		dbpath := viper.GetString(conf.QueryDBPath)

		// check if DB exists at path
		err = info.CheckDBExists(dbpath)
		if err != nil {
			return err
		}

		// cleanup DB
		t := time.Unix(tClean, 0)
		fmt.Printf("Cleaning DBs older than '%s' at %s\n", t.Format(time.ANSIC), dbpath)
		err = cleanOldDBDirs(dbpath, tClean)
		if err != nil {
			return fmt.Errorf("database clean up failed: %s", err)
		}
		return nil
	},
}

var wipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Wipes the entire database. CAUTION: all your data will be lost if you run this!",
	RunE: func(cmd *cobra.Command, args []string) error {

		dbpath := viper.GetString(conf.QueryDBPath)

		status.Linef("Completely wiping DB")

		// check if DB exists at path
		err := info.CheckDBExists(dbpath)
		defer handleStatus(err)

		if err != nil {
			return err
		}

		err = wipeDB(dbpath)
		if err != nil {
			return fmt.Errorf("database wipe failed: %s", err)
		}
		return nil
	},
}

func init() {
	// subcommands
	adminCmd.AddCommand(cleanCmd, wipeCmd)
	adminCmd.SetHelpFunc(printAdminHelp)
}

func printAdminHelp(cmd *cobra.Command, args []string) {
	fmt.Println(adminHelp)
}

type cleanIfaceResult struct {
	DeltaCounts  types.Counters         // number of flows deleted
	DeltaTraffic gpfile.TrafficMetadata // traffic bytes deleted
	NewBegin     int64                  // timestamp of new begin
	Gone         bool                   // The interface has no entries left
}

// TODO: this method probably doesn't reflect the new folder structure. Double check!
func cleanIfaceDir(dbPath string, timestamp int64, iface string) (result cleanIfaceResult, err error) {

	dayTimestamp := gpfile.DirTimestamp(timestamp)

	status.Linef("cleaning DBs for %s", iface)

	entries, err := ioutil.ReadDir(filepath.Join(dbPath, iface))
	defer handleStatus(err)

	if err != nil {
		return result, err
	}

	result.NewBegin = math.MaxInt64

	clean := true
	for _, entry := range entries {
		if !entry.IsDir() {
			clean = false
			continue
		}

		dirTimestamp, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil || fmt.Sprintf("%d", dirTimestamp) != entry.Name() {
			// a directory whose name isn't an int64 wasn't created by
			// goProbe; leave it untouched
			clean = false
			continue
		}

		entryPath := filepath.Join(dbPath, iface, entry.Name())
		gpDir := gpfile.NewDir(filepath.Join(dbPath, iface), dirTimestamp, gpfile.ModeRead)
		if dirTimestamp < dayTimestamp {
			// delete directory
			if err := gpDir.Open(); err == nil {
				result.DeltaCounts = result.DeltaCounts.Add(gpDir.Counts)
				result.DeltaTraffic = result.DeltaTraffic.Add(gpDir.Traffic)
			}

			if err := os.RemoveAll(entryPath); err != nil {
				return result, err
			}
		} else {
			clean = false
			if dirTimestamp < result.NewBegin {
				// update NewBegin
				if err := gpDir.Open(); err == nil {
					if timeFirst, _ := gpDir.TimeRange(); gpDir.NBlocks() > 0 && timeFirst < result.NewBegin {
						result.NewBegin = timeFirst
					}
					result.DeltaCounts = result.DeltaCounts.Add(gpDir.Counts)
					result.DeltaTraffic = result.DeltaTraffic.Add(gpDir.Traffic)
				}
			}

		}
	}

	result.Gone = result.NewBegin == math.MaxInt64

	if clean {
		if err := os.RemoveAll(filepath.Join(dbPath, iface)); err != nil {
			return result, err
		}
	}

	return
}

// Cleans up all directories that cannot contain any flow records
// recorded at timestamp or later.
func cleanOldDBDirs(dbPath string, timestamp int64) error {
	if timestamp >= time.Now().Unix() {
		return fmt.Errorf("only database entries from the past can be cleaned")
	}

	ifaces, err := ioutil.ReadDir(dbPath)
	if err != nil {
		return err
	}

	// Contains changes required to each interface's summary
	ifaceResults := make(map[string]cleanIfaceResult)

	for _, iface := range ifaces {
		if !iface.IsDir() {
			continue
		}

		result, err := cleanIfaceDir(dbPath, timestamp, iface.Name())
		if err != nil {
			return err
		}
		ifaceResults[iface.Name()] = result
	}
	return nil
}

func wipeDB(dbPath string) error {
	// Get list of files in directory
	var dirList []os.FileInfo
	var err error

	if dirList, err = ioutil.ReadDir(dbPath); err != nil {
		return err
	}

	for _, file := range dirList {
		if file.IsDir() {
			if rmerr := os.RemoveAll(dbPath + "/" + file.Name()); rmerr != nil {
				return rmerr
			}
		}
	}

	return err
}

func handleStatus(err error) {
	if err != nil {
		status.Failf("%s", err)
		return
	}
	status.Ok("")
}
