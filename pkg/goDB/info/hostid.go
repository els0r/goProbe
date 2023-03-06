//go:build !linux
// +build !linux

package info

// GetHostID is a method that returns a system's unique identifier. This
// function is meant to be implemented with build tags for specific environments
// where the concept of a host ID makes sense
func GetHostID() string {
	return ""
}
