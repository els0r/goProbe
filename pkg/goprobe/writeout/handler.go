package writeout

import (
	"context"
	"io/fs"
	"time"

	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/logging"
	"golang.org/x/exp/slog"
)

type Handler interface {
	HandleWriteout(ctx context.Context, timestamp time.Time, writeoutChan <-chan capturetypes.TaggedAggFlowMap) <-chan struct{}
}

const WriteoutsChanDepth = 100

type GoDBHandler struct {
	encoderType encoders.Type
	permissions fs.FileMode

	path        string
	dbWriters   map[string]*goDB.DBWriter
	logToSyslog bool
}

func (h *GoDBHandler) WithSyslogWriting(b bool) *GoDBHandler {
	h.logToSyslog = b
	return h
}

func (h *GoDBHandler) WithPermissions(permissions fs.FileMode) *GoDBHandler {
	h.permissions = permissions
	return h
}

func NewGoDBHandler(path string, encoderType encoders.Type) *GoDBHandler {
	return &GoDBHandler{
		path:        path,
		dbWriters:   make(map[string]*goDB.DBWriter),
		encoderType: encoderType,
		permissions: goDB.DefaultPermissions,
	}
}

const tFormat = "2006-01-02 15:04:05 -0700 MST"

func (h *GoDBHandler) handleIfaceWriteout(ctx context.Context, timestamp time.Time, taggedMap capturetypes.TaggedAggFlowMap, syslogWriter *goDB.SyslogDBWriter) {
	ctx = logging.WithFields(ctx, slog.String("iface", taggedMap.Iface))
	logger := logging.FromContext(ctx)

	// Ensure that there is a DBWriter for the given interface
	_, exists := h.dbWriters[taggedMap.Iface]
	if !exists {
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
		for iface := range h.dbWriters {
			if _, exists := seenIfaces[iface]; !exists {
				delete(h.dbWriters, iface)
			}
		}

		elapsed := time.Since(t0).Round(time.Millisecond)

		logger.With("elapsed", elapsed.String()).Debug("completed writeout")
		doneChan <- struct{}{}
	}()

	return doneChan
}
