package api

import (
	"path/filepath"
	"strings"
)

const (
	unixPrefix  = "unix:"
	httpPrefix  = "http://"
	httpsPrefix = "https://"
)

// ExtractUnixSocket determines whether the provided address contains a unix:
// prefix. If so, it will treat the remainder as the path to the socket
func ExtractUnixSocket(addr string) (socketFile string) {
	if strings.HasPrefix(addr, unixPrefix) {
		socketFile = filepath.Clean(strings.TrimPrefix(addr, unixPrefix))
	}
	return
}

// ExtractSchemeAddr extracts the scheme from the address if it is present and returns
// the trimmed address with it. If not scheme match is found, scheme is empty and the
// input to the function will be returned in address
func ExtractSchemeAddr(addr string) (scheme string, address string) {
	switch {
	case strings.HasPrefix(addr, unixPrefix):
		return "", filepath.Clean(strings.TrimPrefix(addr, unixPrefix))
	case strings.HasPrefix(addr, httpPrefix):
		return httpPrefix, filepath.Clean(strings.TrimPrefix(addr, httpPrefix))
	case strings.HasPrefix(addr, httpsPrefix):
		return httpsPrefix, filepath.Clean(strings.TrimPrefix(addr, httpsPrefix))
	}
	return "", addr
}
