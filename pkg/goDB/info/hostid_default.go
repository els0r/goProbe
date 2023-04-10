//go:build !linux
// +build !linux

package info

import "errors"

func hostID() (string, error) {
	return UnknownID, errors.New("not implemented")
}
