//go:build !linux
// +build !linux

package info

func hostID() (string, error) {
	return UnknownID, errors.New("not implemented")
}
