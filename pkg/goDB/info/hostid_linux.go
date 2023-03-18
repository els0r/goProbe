//go:build linux
// +build linux

package info

import (
	"os"
)

const (
	machineIDPath     = "/etc/machine-id"
	machineIDDBusPath = "/var/lib/dbus/machine-id"
)

func hostID() (string, error) {

	// Attempt to read the machine ID from the main file
	idData, err := os.ReadFile(machineIDPath)
	if err != nil {

		// Fallback to DBus based file
		idData, err = os.ReadFile(machineIDDBusPath)
		if err != nil {
			return UnknownID, err
		}
	}

	return sanitizeHostID(idData), nil
}
