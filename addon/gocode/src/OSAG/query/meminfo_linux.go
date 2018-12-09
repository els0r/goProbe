//+build linux

package main

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

func getPhysMem() (float64, error) {
	var memFile *os.File
	var ferr error
	if memFile, ferr = os.OpenFile("/proc/meminfo", os.O_RDONLY, 0444); ferr != nil {
		return 0.0, errors.New("Unable to open /proc/meminfo: " + ferr.Error())
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
		return 0.0, errors.New("Unable to obtain amount of physical memory from /proc/meminfo")
	}

	if ferr = memFile.Close(); ferr != nil {
		return 0.0, errors.New("Unable to close /proc/meminfo after reading: " + ferr.Error())
	}

	return physMem, nil
}
