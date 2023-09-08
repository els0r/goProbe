// Package flags is for parsing goProbe's command line parameters.
package flags

import (
	"errors"
	"flag"
)

// Flags stores goProbe's command line parameters
type Flags struct {
	Config  string
	Version bool
}

// CmdLine globally exposes the parsed flags
var CmdLine = &Flags{}

// Read reads in the command line parameters
func Read() error {
	flag.StringVar(&CmdLine.Config, "config", "", "path to goProbe's configuration file (required)")
	flag.BoolVar(&CmdLine.Version, "version", false, "print goProbe's version and exit")

	flag.Parse()

	if CmdLine.Config == "" && !CmdLine.Version {
		flag.PrintDefaults()
		return errors.New("no configuration file provided")
	}
	return nil
}
