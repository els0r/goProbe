package writeout

import (
	"context"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/logging"
)

const WriteoutsChanDepth = 100

type Handler struct {
	// LastRotation is the time the handler last rotated all flow maps
	LastRotation time.Time

	writeoutsChan  chan *writeout
	captureManager *capture.Manager

	encoderType encoders.Type
	logToSyslog bool
}

func (h *Handler) WithSyslogWriting(b bool) *Handler {
	h.logToSyslog = b
	return h
}

func NewHandler(captureManager *capture.Manager, encoderType encoders.Type) *Handler {
	return &Handler{
		LastRotation: time.Now(),

		writeoutsChan:  make(chan *writeout, WriteoutsChanDepth),
		captureManager: captureManager,

		encoderType: encoderType,
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

func (h *Handler) FullWriteout(ctx context.Context, at time.Time) {
	logger := logging.WithContext(ctx)

	// don't rotate if a bogus timestamp is supplied
	if at.Sub(h.LastRotation) < 0 {
		return
	}
	h.LastRotation = at

	writeout := newWriteout(at)
	defer writeout.close()

	select {
	case <-ctx.Done():
		logger.Debug("writeout was cancelled")
		return
	case h.writeoutsChan <- writeout:
		logger.Debug("initiating flow data flush")

		h.captureManager.RotateAll(writeout.dataChan)
	}
}

func (h *Handler) UpdateAndRotate(ctx context.Context, ifaces config.Ifaces, at time.Time) {
	logger := logging.WithContext(ctx)

	// don't rotate if a bogus timestamp is supplied
	if at.Sub(h.LastRotation) < 0 {
		return
	}
	h.LastRotation = at

	writeout := newWriteout(at)
	defer writeout.close()

	select {
	case <-ctx.Done():
		logger.Debug("writeout was cancelled")
		return
	case h.writeoutsChan <- writeout:
		logger.Debug("initiating flow data flush")

		h.captureManager.Update(ifaces, writeout.dataChan)
	}
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
				h.FullWriteout(ctx, t)

				if len(h.writeoutsChan) > 2 {
					log := logger.With("queue_length", len(h.writeoutsChan))
					if len(h.writeoutsChan) > capture.WriteoutsChanDepth {
						log.Fatalf("writeouts are lagging behind too much")
					}
					log.Warn("writeouts are lagging behind")
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
				logger.Error("failed to create syslog based flow writer: %v", err)
			}
		}

		for writeout := range h.writeoutsChan {
			t0 := time.Now()
			var summaryUpdates []goDB.InterfaceSummaryUpdate
			count := 0
			for taggedMap := range writeout.dataChan {
				// Ensure that there is a DBWriter for the given interface
				_, exists := dbWriters[taggedMap.Iface]
				if !exists {
					w := goDB.NewDBWriter(config.RuntimeDBPath(),
						taggedMap.Iface,
						h.encoderType,
					)
					dbWriters[taggedMap.Iface] = w
				}

				// Prep metadata for current block
				meta := goDB.BlockMetadata{
					PcapPacketsReceived: -1,
					PcapPacketsDropped:  -1,
				}
				if taggedMap.Stats.CaptureStats != nil {
					meta.PcapPacketsReceived = taggedMap.Stats.CaptureStats.PacketsReceived
					meta.PcapPacketsDropped = taggedMap.Stats.CaptureStats.PacketsDropped
				}
				meta.PacketsLogged = taggedMap.Stats.PacketsLogged
				meta.Timestamp = writeout.atTimestamp.Unix()

				// Write to database, update summary
				update, err := dbWriters[taggedMap.Iface].Write(taggedMap.Map, meta, meta.Timestamp)
				lastWrite[taggedMap.Iface] = writeoutsCount
				if err != nil {
					logger.Errorf("writeout failed: %v", err)
				} else {
					summaryUpdates = append(summaryUpdates, update)
				}

				// write out flows to syslog if necessary
				if h.logToSyslog {
					if syslogWriter != nil {
						syslogWriter.Write(taggedMap.Map, taggedMap.Iface, meta.Timestamp)
					} else {
						logger.Error("cannot write flows to <nil> syslog writer. Attempting reinitialization")

						// try to reinitialize the writer
						if syslogWriter, err = goDB.NewSyslogDBWriter(); err != nil {
							logger.Error("failed to reinitialize syslog writer: %v", err)
						}
					}
				}
				count++
			}

			// We are done with the writeout, let's try to write the updated summary
			err := goDB.ModifyDBSummary(config.RuntimeDBPath(), 10*time.Second, func(summ *goDB.DBSummary) (*goDB.DBSummary, error) {
				if summ == nil {
					summ = goDB.NewDBSummary()
				}
				for _, update := range summaryUpdates {
					summ.Update(update)
				}
				return summ, nil
			})
			if err != nil {
				logger.Error("error updating summary: %v", err)
			}

			// Clean up dead writers. We say that a writer is dead
			// if it hasn't been used in the last few writeouts.
			var remove []string
			for iface, last := range lastWrite {
				if writeoutsCount-last >= 3 {
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
