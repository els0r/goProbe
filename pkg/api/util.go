package api

import (
	"path/filepath"
	"strings"
)

const (
	unixPrefix = "unix:"
)

// ExtractUnixSocket determines whether the provided address contains a unix:
// prefix. If so, it will treat the remainder as the path to the socket
func ExtractUnixSocket(addr string) (socketFile string) {
	if strings.HasPrefix(addr, unixPrefix) {
		socketFile = filepath.Clean(strings.TrimPrefix(addr, unixPrefix))
	}
	return
}
