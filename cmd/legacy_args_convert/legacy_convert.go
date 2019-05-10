// small binary to convert the JSON cmd line arguments (from output_consistency tests of previous goQuery versions) to JSON serialized query.Args

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/els0r/goProbe/pkg/query"
	lg "github.com/els0r/log"
)

var (
	outFolder           = "./converted_args"
	legacyFolder        = "./legacy_args"
	argsSuffix          = ".args.json"
	correctOutputSuffix = ".correctOutput.json"
)

func main() {

	// create logger
	log, lerr := lg.NewFromString("console", lg.WithLevel(lg.DEBUG))
	if lerr != nil {
		fmt.Fprintf(os.Stderr, "failed to instantiate logger: %s", lerr)
		os.Exit(1)
	}

	testCases, err := testCases()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	err = os.MkdirAll(outFolder, 0755)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	for i, testCase := range testCases {
		argumentFile := path.Join(legacyFolder, testCase+argsSuffix)

		// Read arguments
		argumentsJSON, err := ioutil.ReadFile(argumentFile)
		if err != nil {
			log.Errorf("Could not read argument file %s. Error: %s", argumentFile, err)
			os.Exit(1)
		}
		var arguments [][]string
		err = json.Unmarshal(argumentsJSON, &arguments)
		if err != nil {
			log.Errorf("Could not decode argument file %s. Error: %s", argumentFile, err)
			os.Exit(1)
		}

		// convert to query Args
		queryArgs := convert(arguments)
		fmt.Println(i, queryArgs)

		// marshal the objects
		var outFile *os.File
		outPath := path.Join(outFolder, testCase+argsSuffix)
		outFile, err = os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			log.Errorf("couldn't open output file for writing: %s", err)
			os.Exit(1)
		}

		enc := json.NewEncoder(outFile)
		enc.SetIndent("", "  ")
		err = enc.Encode(queryArgs)
		if err != nil {
			log.Errorf("couldn't write json args: %s", err)
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func convert(args [][]string) []query.Args {
	var queryArgs []query.Args
	for _, argrow := range args {
		qa := query.NewArgs(argrow[len(argrow)-1], "")
		for i := 0; i < len(argrow)-1; {
			nextArg := argrow[i+1]
			switch argrow[i] {
			case "-c":
				qa.Condition = nextArg
				i += 2
			case "-i":
				qa.Ifaces = nextArg
				i += 2
			case "-d":
				if nextArg == "$TESTDB" {
					qa.DBPath = "./testdb"
				} else {
					qa.DBPath = nextArg
				}
				i += 2
			case "-f":
				qa.First = nextArg
				i += 2
			case "-l":
				qa.Last = nextArg
				i += 2
			case "-n":
				qa.NumResults, _ = strconv.Atoi(nextArg)
				i += 2
			case "-e":
				qa.Format = nextArg
				i += 2
			case "-sum":
				qa.Sum = true
				i++
			case "-out":
				qa.Out = true
				i++
			case "-in":
				qa.In = true
				i++
			}
		}
		queryArgs = append(queryArgs, *qa)
	}
	return queryArgs
}

func testCases() (testCases []string, err error) {
	// Open file descriptor for legacyFolder
	fd, err := os.Open(legacyFolder)
	if err != nil {
		err = fmt.Errorf("Could not open directory %v: %v", legacyFolder, err)
		return
	}
	// Get a list of ALL files in the directory, that's what the -1 is for.
	fileInfos, err := fd.Readdir(-1)
	if err != nil {
		err = fmt.Errorf("Could not enumerate directory %v: %v", legacyFolder, err)
		return
	}

	// Map to keep track of whether (for each testcase) we have seen a correctOutput file AND an args file.
	seen := make(map[string]struct {
		correctOutput bool
		args          bool
	})

	// Loop over all entries of legacyFolder and populate seen map.
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		name := fileInfo.Name()
		if strings.HasSuffix(name, argsSuffix) {
			key := name[:len(name)-len(argsSuffix)]
			seenElem := seen[key]
			seenElem.args = true
			seen[key] = seenElem
		} else if strings.HasSuffix(name, correctOutputSuffix) {
			key := name[:len(name)-len(correctOutputSuffix)]
			seenElem := seen[key]
			seenElem.correctOutput = true
			seen[key] = seenElem
		}
	}

	// Check validity condition, i.e. whether for each testcase we have both files.
	for testName, seenFiles := range seen {
		if !seenFiles.args {
			return nil, fmt.Errorf("File %v%v is missing", testName, argsSuffix)
		}
		if !seenFiles.correctOutput {
			return nil, fmt.Errorf("File %v%v is missing", testName, correctOutputSuffix)
		}
	}

	// If we get here, the validity condition is satisfied; we return the list of test cases.
	for testName := range seen {
		testCases = append(testCases, testName)
	}
	return
}
