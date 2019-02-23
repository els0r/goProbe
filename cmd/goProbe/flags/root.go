package flags

import (
	"errors"
	"flag"
)

type Flags struct {
	Config  string
	Version bool
}

var CmdLine *Flags

func init() {
	CmdLine = &Flags{}
}

//var versionCmd = &cobra.Command{
//	Use:   "version",
//	Short: `Print the version number of goProbe and exit`,
//	Long:  `Print the version number of goProbe and exit`,
//	Run: func(cmd *cobra.Command, args []string) {
//		fmt.Printf("goProbe %s\n", version.VersionText())
//		os.Exit(0)
//	},
//}
//
//var rootCmd *cobra.Command

//func init() {
//	CmdLine = Flags{}
//	rootCmd = &cobra.Command{
//		Use:   "goProbe",
//		Short: "goProbe is a lightweight network traffic meta-data capture process",
//		Long:  "goProbe is a lightweight network traffic meta-data capture process",
//	}
//
//	rootCmd.Flags().StringVar(&CmdLine.Config, "config", "/opt/ntm/goProbe/etc/goprobe.conf", "path to goProbe's configuration file (required)")
//
//	rootCmd.MarkFlagRequired("config")
//	rootCmd.AddCommand(versionCmd)
//}

func Read() error {
	flag.StringVar(&CmdLine.Config, "config", "", "path to goProbe's configuration file (required)")
	flag.BoolVar(&CmdLine.Version, "version", false, "print goProbe's version and exit")

	flag.Parse()

	if CmdLine.Config == "" && !CmdLine.Version {
		flag.PrintDefaults()
		return errors.New("No configuration file provided")
	}
	return nil
}
