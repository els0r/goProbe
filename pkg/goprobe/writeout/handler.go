package writeout

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/logging"
)

const WriteoutsChanDepth = 100

// writerCleanupCutoff governs when old DB writers are deleted from the map
// of available writers
const writerCleanupCutoff = 3

type Handler struct {
	// LastRotation is the time the handler last rotated all flow maps
	LastRotation time.Time

	writeoutsChan  chan *writeout
	captureManager *capture.Manager

	encoderType encoders.Type
	permissions fs.FileMode
	logToSyslog bool
}

func (h *Handler) WithSyslogWriting(b bool) *Handler {
	h.logToSyslog = b
	return h
}

func (h *Handler) WithPermissions(permissions fs.FileMode) *Handler {
	h.permissions = permissions
	return h
}

func NewHandler(captureManager *capture.Manager, encoderType encoders.Type) *Handler {
	return &Handler{
		LastRotation: time.Now(),

		writeoutsChan:  make(chan *writeout, WriteoutsChanDepth),
		captureManager: captureManager,

		encoderType: encoderType,
		permissions: goDB.DefaultPermissions,
	}
}

func (h *Handler) Close() {
	close(h.writeoutsChan)
}

type writeout struct {
	dataChan    chan capture.TaggedAggFlowMap
	atTimestamp time.Time
}

func newWriteout(at time.Time) *writeout {
	return &writeout{
		dataChan:    make(chan capture.TaggedAggFlowMap, capture.MaxIfaces),
		atTimestamp: at,
	}
}

func (w *writeout) close() {
	close(w.dataChan)
}

const tFormat = "2006-01-02 15:04:05 -0700 MST"

func (h *Handler) FullWriteout(ctx context.Context, at time.Time) error {
	logger := logging.WithContext(ctx)

	// don't rotate if a bogus timestamp is supplied
	if at.Sub(h.LastRotation) < 0 {
		return fmt.Errorf("attempting rotation at %s before the most recent one at %s",
			at.Format(tFormat),
			h.LastRotation.Format(tFormat),
		)
	}
	h.LastRotation = at

	writeout := newWriteout(at)
	defer writeout.close()

	select {
	case <-ctx.Done():
		logger.Debug("writeout was cancelled")
		return nil
	case h.writeoutsChan <- writeout:
		logger.Debug("initiating flow data flush")

		h.captureManager.RotateAll(writeout.dataChan)
	}
	return nil
}

func (h *Handler) UpdateAndRotate(ctx context.Context, ifaces config.Ifaces, at time.Time) error {
	logger := logging.WithContext(ctx)

	// don't rotate if a bogus timestamp is supplied
	if at.Sub(h.LastRotation) < 0 {
		return fmt.Errorf("attempting rotation at %s before the most recent one at %s",
			at.Format(tFormat),
			h.LastRotation.Format(tFormat),
		)
	}
	h.LastRotation = at

	writeout := newWriteout(at)
	defer writeout.close()

	select {
	case <-ctx.Done():
		logger.Debug("writeout was cancelled")
		return nil
	case h.writeoutsChan <- writeout:
		logger.Debug("initiating flow data flush")

		h.captureManager.Update(ifaces, writeout.dataChan)
	}
	return nil
}

func (h *Handler) HandleRotations(ctx context.Context, interval time.Duration) {
	go func() {
		logger := logging.WithContext(ctx)

		// wait until the next 5 minute interval of the hour is reached before starting the ticker
		tNow := time.Now()

		sleepUntil := tNow.Truncate(interval).Add(interval).Sub(tNow)
		logger.Infof("waiting for %s to start capture rotation", sleepUntil.Round(time.Second))

		timer := time.NewTimer(sleepUntil)
		select {
		case <-timer.C:
			break
		case <-ctx.Done():
			return
		}

		ticker := time.NewTicker(interval)

		// immediately write out after the initial sleep has completed
		t := time.Now()
		for {
			select {
			case <-ctx.Done():
				logger.Info("stopping rotation handler")
				return
			default:
				err := h.FullWriteout(ctx, t)
				if err != nil {
					logger.Errorf("failed to write data: %v", err)
				} else {
					if len(h.writeoutsChan) > 2 {
						log := logger.With("queue_length", len(h.writeoutsChan))
						if len(h.writeoutsChan) > capture.WriteoutsChanDepth {
							log.Fatalf("writeouts are lagging behind too much")
						}
						log.Warn("writeouts are lagging behind")
					}
				}

				logger.Debug("restarting any interfaces that have encountered errors")
				h.captureManager.EnableAll()

				// wait for the the next ticker to complete
				t = <-ticker.C
			}
		}
	}()
}

func (h *Handler) HandleWriteouts() <-chan struct{} {
	logger := logging.Logger()

	done := make(chan struct{})
	go func() {
		logger.Info("starting writeout handler")

		var (
			writeoutsCount = 0
			dbWriters      = make(map[string]*goDB.DBWriter)
			lastWrite      = make(map[string]int)
		)

		var syslogWriter *goDB.SyslogDBWriter
		if h.logToSyslog {
			var err error
			if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
				// we are not failing here due to the fact that a DB write out should still be attempted.
				logger.Errorf("failed to create syslog based flow writer: %v", err)
			}
		}

		for writeout := range h.writeoutsChan {
			t0 := time.Now()
			count := 0
			for taggedMap := range writeout.dataChan {
				// Ensure that there is a DBWriter for the given interface
				_, exists := dbWriters[taggedMap.Iface]
				if !exists {
					w := goDB.NewDBWriter(config.RuntimeDBPath(),
						taggedMap.Iface,
						h.encoderType,
					).Permissions(h.permissions)
					dbWriters[taggedMap.Iface] = w
				}

				packetsDropped := 0
				if taggedMap.Stats.CaptureStats != nil {
					packetsDropped = taggedMap.Stats.PacketsDropped
				}

				// Write to database, update summary
				err := dbWriters[taggedMap.Iface].Write(taggedMap.Map, goDB.CaptureMetadata{
					PacketsDropped: packetsDropped,
				}, writeout.atTimestamp.Unix())
				lastWrite[taggedMap.Iface] = writeoutsCount
				if err != nil {
					logger.Error(fmt.Sprintf("Error during writeout: %s", err.Error()))
				}

				// write out flows to syslog if necessary
				if h.logToSyslog {
					if syslogWriter != nil {
						syslogWriter.Write(taggedMap.Map, taggedMap.Iface, writeout.atTimestamp.Unix())
					} else {
						logger.Error("cannot write flows to <nil> syslog writer. Attempting reinitialization")

						// try to reinitialize the writer
						if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
							logger.Errorf("failed to reinitialize syslog writer: %v", err)
						}
					}
				}
				count++
			}

			// Clean up dead writers. We say that a writer is dead
			// if it hasn't been used in the last few writeouts.
			var remove []string
			for iface, last := range lastWrite {
				if writeoutsCount-last >= writerCleanupCutoff {
					remove = append(remove, iface)
				}
			}
			for _, iface := range remove {
				delete(dbWriters, iface)
				delete(lastWrite, iface)
			}

			writeoutsCount++

			elapsed := time.Since(t0).Round(time.Millisecond)

			logger.With("count", count, "elapsed", elapsed.String()).Debug("completed writeout")
		}

		logger.Info("completed all writeouts")
		done <- struct{}{}
	}()
	return done
}
