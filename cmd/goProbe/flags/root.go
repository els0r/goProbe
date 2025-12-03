// Package flags is for parsing goProbe's command line parameters.
package flags

import (
	"errors"
	"flag"
)

// Flags stores goProbe's command line parameters
type Flags struct {
	Config             string
	Version            bool
	OpenAPISpecOutfile string
}

// CmdLine globally exposes the parsed flags
var CmdLine = &Flags{}

// Read reads in the command line parameters
func Read() error {
	c := flag.String("config", "", "path to goProbe's configuration file (required)")
	flag.StringVar(c, "c", *c, "alias for -config")
	flag.BoolVar(&CmdLine.Version, "version", false, "print goProbe's version and exit")
	flag.StringVar(&CmdLine.OpenAPISpecOutfile, "openapi.spec-outfile", "", "write OpenAPI 3.0.3 spec to output file and exit")

	flag.Parse()

	CmdLine.Config = *c

	if CmdLine.Config == "" && !CmdLine.Version {
		flag.PrintDefaults()
		return errors.New("no configuration file provided")
	}
	return nil
}
