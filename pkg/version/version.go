// Package version is used by the release process to add an
// informative version string to some commands.
package version

import (
	"fmt"
	"runtime"
	"time"
)

//go:generate go run make_version.go

// These strings will be overwritten by an init function in
// created by make_version.go during the release process.
var (
	BuildTime = time.Time{}
	GitSHA    = ""
)

// Version returns a newline-terminated string describing the current
// version of the build.
func Version() string {
	if GitSHA == "" {
		return "devel\n"
	}

	str := fmt.Sprintf(`    Build time:     %s
    Git hash:       %s
    Go versions:    %s
`, BuildTime.In(time.UTC).Format(time.Stamp+" 2006 UTC"),
		GitSHA,
		runtime.Version(),
	)
	return str
}
