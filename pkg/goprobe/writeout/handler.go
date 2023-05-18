package writeout

import (
	"context"
	"time"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
)

// WriteoutsChanDepth defines a default depth for sending writeouts over the writeout channel
const WriteoutsChanDepth = 100

// Handler defines a generic interface for handling writeouts
type Handler interface {

	// HandleWriteout provides access to writeouts via a channel
	HandleWriteout(ctx context.Context, timestamp time.Time, writeoutChan <-chan capturetypes.TaggedAggFlowMap) <-chan struct{}
}
