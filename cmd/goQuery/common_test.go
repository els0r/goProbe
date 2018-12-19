/////////////////////////////////////////////////////////////////////////////////
//
// common_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, October 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

const (
	// "Magic" environment variables used for tests. See comment above
	// TestMagicCallMain for detailed explanation.
	MAGIC_ENV_VAR = "GOTEST_argumentsJson"

	// Path to a small test database that is needed for many tests and part of
	// the repository. We don't need to make this configurable since the small
	// test database is checked into the git repository.
	SMALL_GODB = "./testdb"
)

// NOT a normal test!
//
// The following code is inspired by
// http://talks.golang.org/2015/tricks.slide#37 Here is how it works:
//
// 1. When  this method is called by the testrunner (the 1st time), the
// environment variable MAGIC_ENV_VAR is not set. So nothing happens. (So how
// does the environment variable get set? We set the environment variable and
// execute the test executable as a subprocess of the main testing process in
// callMain(). The subprocess is instructed to only call the TestMagicCallMain
// method.)
//
// 2. When TestMagicCallMain is called for the 2nd time, the environment
// variable MAGIC_ENV_VAR has been set and hence the if-branch is taken. The if
// branch modifies the value of os.Args and hands control to main() (from
// GPQuery). The result is that the testing subprocess acts like goQuery called
// with the arguments in os.Args.
//
// 3. The main testing process checks whether the subprocess acting like goquery
// behaved as intended.
func TestMagicCallMain(t *testing.T) {
	if argumentsJson := os.Getenv(MAGIC_ENV_VAR); argumentsJson != "" {
		var arguments []string
		err := json.Unmarshal([]byte(argumentsJson), &arguments)
		if err != nil {
			panic("Couldn't unmarshal JSON argument string")
		}

		os.Args = []string{os.Args[0]}
		os.Args = append(os.Args, arguments...)

		main()
		return
	}
}

// Returns a Cmd struct to execute goQuery with the given arguments.
// See TestMagicCallMain for further details of how we do this.
// Note: Actually, we don't really execute goQuery, but rather the
// main() method from goQuery inside a test executable. In particular, our fake goQuery
// prints PASS whenever main() from goQuery runs successfully and spams us with failure
// information when main() from goQuery fails.
func callMain(arg ...string) *exec.Cmd {
	argumentsJson, err := json.Marshal(arg)
	if err != nil {
		panic(fmt.Sprintf("Couldn't encode arguments as JSON. Error: %s", err))
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMagicCallMain")
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", MAGIC_ENV_VAR, argumentsJson))
	return cmd
}

func checkDbExists(tb testing.TB, path string) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) || !fi.IsDir() {
			tb.Fatalf("Couldn't find database at %s", path)
		}
	}
}
