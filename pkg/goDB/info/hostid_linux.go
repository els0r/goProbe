//go:build linux
// +build linux

package info

import (
	"os"
	"path/filepath"
)

const (
	machineIDPath     = "/etc/machine-id"
	machineIDDBusPath = "/var/lib/dbus/machine-id"
)

func hostID() (string, error) {

	// Attempt to read the machine ID from the main file
	idData, err := os.ReadFile(filepath.Clean(machineIDPath))
	if err != nil {

		// Fallback to DBus based file
		idData, err = os.ReadFile(filepath.Clean(machineIDDBusPath))
		if err != nil {
			return UnknownID, err
		}
	}

	return sanitizeHostID(idData), nil
}
