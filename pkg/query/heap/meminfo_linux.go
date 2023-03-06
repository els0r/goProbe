//go:build linux
// +build linux

package heap

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const memFilePath = "/proc/meminfo"

func getPhysMem() (float64, error) {
	var memFile *os.File
	var ferr error
	if memFile, ferr = os.OpenFile(memFilePath, os.O_RDONLY, 0444); ferr != nil {
		return 0.0, fmt.Errorf("unable to open %s: %w", memFilePath, ferr)
	}

	physMem := 0.0
	memInfoScanner := bufio.NewScanner(memFile)
	for memInfoScanner.Scan() {
		if strings.Contains(memInfoScanner.Text(), "MemTotal") {
			memTokens := strings.Split(memInfoScanner.Text(), " ")
			physMem, _ = strconv.ParseFloat(memTokens[len(memTokens)-2], 64)
		}
	}

	if physMem < 0.1 {
		return 0.0, fmt.Errorf("unable to obtain amount of physical memory from %s", memFilePath)
	}

	if ferr = memFile.Close(); ferr != nil {
		return 0.0, fmt.Errorf("unable to close %s after reading: %w", memFilePath, ferr)
	}

	return physMem, nil
}
