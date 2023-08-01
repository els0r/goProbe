package writeout

import (
	"context"
	"io/fs"
	"sync"
	"time"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/logging"
	"golang.org/x/exp/slog"
)

// GoDBHandler denotes a GoDB writeout handler
type GoDBHandler struct {
	encoderType encoders.Type
	permissions fs.FileMode

	path        string
	dbWriters   map[string]*goDB.DBWriter
	logToSyslog bool

	sync.Mutex
}

// NewGoDBHandler instantiates a new GoDB handler
func NewGoDBHandler(path string, encoderType encoders.Type) *GoDBHandler {
	return &GoDBHandler{
		path:        path,
		dbWriters:   make(map[string]*goDB.DBWriter),
		encoderType: encoderType,
		permissions: goDB.DefaultPermissions,
	}
}

// WithSyslogWriting enables / disables explicit writing to Syslog facilities
func (h *GoDBHandler) WithSyslogWriting(b bool) *GoDBHandler {
	h.logToSyslog = b
	return h
}

// WithPermissions sets explicit permissions for the underlying GoDB
func (h *GoDBHandler) WithPermissions(permissions fs.FileMode) *GoDBHandler {
	h.permissions = permissions
	return h
}

// HandleWriteout provides access to writeouts to a GoDB via a channel
func (h *GoDBHandler) HandleWriteout(ctx context.Context, timestamp time.Time, writeoutChan <-chan capturetypes.TaggedAggFlowMap) <-chan struct{} {

	doneChan := make(chan struct{})
	go func() {

		logger := logging.FromContext(ctx)
		t0 := time.Now()

		var syslogWriter *goDB.SyslogDBWriter
		if h.logToSyslog {
			var err error
			if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {

				// we are not failing here due to the fact that a DB write out should still be attempted.
				logger.Errorf("failed to create syslog based flow writer: %v", err)
			}
		}

		seenIfaces := make(map[string]struct{})
		for taggedMap := range writeoutChan {
			seenIfaces[taggedMap.Iface] = struct{}{}
			h.handleIfaceWriteout(ctx, timestamp, taggedMap, syslogWriter)
		}

		// Clean up dead writers. We say that a writer is dead
		// if it hasn't been used in the last few writeouts.
		h.Lock()
		for iface := range h.dbWriters {
			if _, exists := seenIfaces[iface]; !exists {
				delete(h.dbWriters, iface)
			}
		}
		h.Unlock()

		elapsed := time.Since(t0).Round(time.Millisecond)

		logger.With("elapsed", elapsed.String()).Debug("completed writeout")
		doneChan <- struct{}{}
	}()

	return doneChan
}

func (h *GoDBHandler) handleIfaceWriteout(ctx context.Context, timestamp time.Time, taggedMap capturetypes.TaggedAggFlowMap, syslogWriter *goDB.SyslogDBWriter) {
	ctx = logging.WithFields(ctx, slog.String("iface", taggedMap.Iface))
	logger := logging.FromContext(ctx)

	// Ensure that there is a DBWriter for the given interface
	h.Lock()
	if _, exists := h.dbWriters[taggedMap.Iface]; !exists {
		w := goDB.NewDBWriter(h.path,
			taggedMap.Iface,
			h.encoderType,
		).Permissions(h.permissions)
		h.dbWriters[taggedMap.Iface] = w
	}

	// Write to database, update summary
	err := h.dbWriters[taggedMap.Iface].Write(taggedMap.Map, goDB.CaptureMetadata{
		PacketsDropped: taggedMap.Stats.Dropped,
	}, timestamp.Unix())
	if err != nil {
		logger.Errorf("failed to perform writeout: %s", err)
	}
	h.Unlock()

	// write out flows to syslog if necessary
	if h.logToSyslog {
		if syslogWriter == nil {
			logger.Error("cannot write flows to <nil> syslog writer. Attempting reinitialization")

			// try to reinitialize the writer
			if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
				logger.Errorf("failed to reinitialize syslog writer: %v", err)
				return
			}
		}

		syslogWriter.Write(taggedMap.Map, taggedMap.Iface, timestamp.Unix())
	}
}
