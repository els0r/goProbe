//go:build linux && force_pfring
// +build linux,force_pfring

package capture

import (
	"errors"
)

// TODO: think about the poll timeout
var errCaptureTimeout = errors.New("capture timeout")

func newSource() Source {
	return &PFRingSource{}
}
