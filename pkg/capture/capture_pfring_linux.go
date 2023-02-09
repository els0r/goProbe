//go:build force_pfring
// +build force_pfring

package capture

import (
	"errors"
)

// TODO: think about the poll timeout
var errCaptureTimeout = errors.New("capture timeout")

func newSource() Source {
	return &PFRingSource{}
}
