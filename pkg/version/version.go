/////////////////////////////////////////////////////////////////////////////////
//
// version.go
//
// Written by Lorenz Breidenbach lob@open.ch, February 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

// Package version provides a single place to store/retrieve all version information
package version

import (
	"fmt"
	"runtime"
)

// these variables are set during build by the command
// go build -ldflags "-X pkg/version.version=3.14 ..."
var (
	version   = "unknown"
	commit    = "unknown"
	builddate = "unknown"
)

// Version returns the version number of goProbe/goQuery, e.g. "2.1"
func Version() string {
	return version
}

// Commit returns the git commit sha1 of goProbe/goQuery. If the build
// was from a dirty tree, the hash will be prepended with a "!".
func Commit() string {
	return commit
}

// BuildDate returns the date and time when goProbe/goQuery were built.
func BuildDate() string {
	return builddate
}

// Text returns ready-for-printing output for the -version target
// containing the build kind, version number, commit hash, build date and
// go version.
func Text() string {
	return fmt.Sprintf(
		"%s version %s (commit id: %s, built on: %s) using go %s",
		BuildKind,
		version,
		commit,
		builddate,
		runtime.Version(),
	)
}
