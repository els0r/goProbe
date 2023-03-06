//go:build linux
// +build linux

package info

import (
	"io"
	"os"
)

const machineIDFile = "/etc/machine-id"

// GetHostID attempts to read the machine-id from /etc/machine-id and returns
// it as a string
func GetHostID() (id string) {
	f, err := os.OpenFile(machineIDFile, os.O_RDONLY, 0600)
	if err != nil {
		return id
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return id
	}
	return string(b)
}
