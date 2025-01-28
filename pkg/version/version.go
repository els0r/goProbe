// Package version is used by the release process to add an
// informative version string to some commands.
package version

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

//go:generate go run make_version.go

// These strings will be overwritten by an init function in
// created by make_version.go during the release process.
var (
	BuildTime = time.Time{}
	GitSHA    = ""
	SemVer    = ""
)

const (
	devel = "devel"
)

// Version returns a newline-terminated string describing the current
// version of the build.
func Version() string {
	progName := filepath.Base(os.Args[0])

	if GitSHA == "" {
		return progName + " " + devel + "\n"
	}

	semver := SemVer
	if semver == "" {
		semver = devel
	}

	str := fmt.Sprintf(`%s - %s:
    Build time:     %s
    Git hash:       %s
    Go versions:    %s
`,
		progName, semver,
		BuildTime.In(time.UTC).Format(time.Stamp+" 2006 UTC"),
		GitSHA,
		runtime.Version(),
	)
	return str
}

// Short returns a shortened GitSHA string that is equivalent to
// git rev-parse --short. If SemVer has been provided, it will be
// prepended
func Short() string {
	if len(GitSHA) < 8 {
		return devel
	}
	short := GitSHA[0:8]
	if SemVer != "" {
		short = SemVer + "-" + short
	}
	return short
}
