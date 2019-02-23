// Package v1 specifies goProbe API version 1
//
// Actions (POST):
// Path: /_
//  - reload:
//      Triggers a reload of the configuration.
//      Parameters:
//          None
//
// Statistics (GET):
// Path: /stats
// Paramters:
//    * pretty: if set to 1, it will use status line to print out the statistics. Default format is JSON
// - /packets
//    Returns the number of packets received in the last writeout period
//    Parameters:
//        * debug
// - /errors
//    Returns the pcap errors ocurring on each interface

package v1

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/status"
	"github.com/go-chi/chi"

	capconfig "github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/cmd/goProbe/flags"
	log "github.com/els0r/log"
)

type Option func(*API)

func WithLogger(logger log.Logger) Option {
	return func(a *API) {
		a.logger = logger
	}
}

type API struct {
	c      *capture.Manager
	logger log.Logger
}

func New(manager *capture.Manager, opts ...Option) *API {
	a := &API{c: manager}

	// apply options
	for _, opt := range opts {
		opt(a)
	}

	if a.logger != nil {
		a.logger.Debugf("Enabling API %s", a.Version())
	}

	return a
}

func (a *API) Version() string {
	return "v1"
}

func (a *API) Routes() *chi.Mux {
	r := chi.NewRouter()
	// action routes
	r.Mount("/", a.postRoutes())

	// getter routes
	r.Mount("/stats", a.getRoutes())

	return r
}

func (a *API) postRoutes() *chi.Mux {
	r := chi.NewRouter()

	// list actions here
	r.Post("/_reload", a.handleReload)

	return r
}

func (a *API) getRoutes() *chi.Mux {
	r := chi.NewRouter()

	// list actions here
	r.Get("/packets", a.getPacketStats)
	r.Get("/errors", a.getErrors)

	return r
}

func (a *API) handleReload(w http.ResponseWriter, r *http.Request) {
	status.SetOutput(w)
	var writeoutsChan chan<- capture.Writeout = a.c.WriteoutHandler.WriteoutChan

	capconfig.Mutex.Lock()
	status.Line("Reloading configuration")
	config, err := reloadConfig()
	if err == nil {
		woChan := make(chan capture.TaggedAggFlowMap, capture.MAX_IFACES)
		writeoutsChan <- capture.Writeout{woChan, time.Now()}
		a.c.Update(config.Interfaces, woChan)
		close(woChan)

		status.Ok("")
	} else {
		if a.logger != nil {
			a.logger.Error(err.Error())
		}
		status.Fail(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	capconfig.Mutex.Unlock()
}

func (a *API) getPacketStats(w http.ResponseWriter, r *http.Request) {
	status.SetOutput(w)

	stats := a.c.StatusAll()

	writeLn := func(msg string) {
		_, writeErr := fmt.Fprint(w, msg+"\n")
		if writeErr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}

	writeLn("Interface and pcap statistics")

	// print info for each interface
	var loggedRcvd, pcapRcvd, pcapDrop, pcapIfDrop uint64
	var numActive int
	for _, stat := range stats {
		if stat.Stats.Pcap != nil {
			loggedRcvd += uint64(stat.Stats.PacketsLogged)
			pcapRcvd += uint64(stat.Stats.Pcap.PacketsReceived)
			pcapDrop += uint64(stat.Stats.Pcap.PacketsDropped)
			pcapIfDrop += uint64(stat.Stats.Pcap.PacketsIfDropped)
		}
		if stat.State == capture.CAPTURE_STATE_ACTIVE {
			numActive++
		}
	}
	writeLn(fmt.Sprintf(
		`
   last writeout: %.0fs ago
  packets logged: %d

   pcap received: %d
         dropped: %d
   iface dropped: %d`,
		time.Now().Sub(a.c.LastRotation).Seconds(),
		loggedRcvd, pcapRcvd, pcapDrop, pcapIfDrop))

	// check if debug info should be printed
	dbg := r.FormValue("debug")
	if dbg == "1" {
		writeLn(fmt.Sprintf("\n%s   RCVD     DROP   IFDROP", strings.Repeat(" ", status.StatusLineIndent+8+4)))

		for iface, stat := range stats {
			var pcapInfoStr string
			if stat.Stats.Pcap != nil {
				pcapInfoStr = fmt.Sprintf("%8d %8d %8d",
					stat.Stats.Pcap.PacketsReceived,
					stat.Stats.Pcap.PacketsDropped,
					stat.Stats.Pcap.PacketsIfDropped)
			}

			status.Line(iface)
			switch stat.State {
			case capture.CAPTURE_STATE_UNINITIALIZED:
				status.Warn("unitialized")
			case capture.CAPTURE_STATE_INITIALIZED:
				status.Warn("initialized")
			case capture.CAPTURE_STATE_ACTIVE:
				status.Ok(pcapInfoStr)
			case capture.CAPTURE_STATE_ERROR:
				status.Fail("error")
			default:
				status.Custom(status.White, "NONE", "Unknown capture state")
			}
		}
	}

	writeLn("")
	status.Line("Total")

	activeStr := fmt.Sprintf("%d/%d interfaces active", numActive, len(stats))
	if numActive != len(stats) {
		if numActive == 0 {
			status.Fail(activeStr)
		} else {
			status.Warn(activeStr)
		}
	} else {
		status.Ok(activeStr)
	}
}

func (a *API) getErrors(w http.ResponseWriter, r *http.Request) {
	status.SetOutput(w)

	errors := a.c.ErrorsAll()

	for iface, errs := range errors {
		status.Line(iface)
		if len(errs) > 0 {
			for errString, count := range errs {
				status.Warnf(" [%8d] %s", count, errString)
			}
		} else {
			status.Ok("no errors")
		}
	}
}

// reloadConfig attempts to reload the configuration file and updates
// the global config if successful.
func reloadConfig() (*capconfig.Config, error) {
	c, err := capconfig.ParseFile(flags.CmdLine.Config)
	if err != nil {
		return nil, fmt.Errorf("Failed to reload config file: %s", err)
	}
	if c == nil {
		return nil, fmt.Errorf("Retrieved <nil> configuration")
	}

	if len(c.Interfaces) > capture.MAX_IFACES {
		return nil, fmt.Errorf("Cannot monitor more than %d interfaces.", capture.MAX_IFACES)
	}

	if capconfig.RuntimeDBPath() != c.DBPath {
		return nil, fmt.Errorf("Failed to reload config file: Cannot change database path while running.")
	}
	return c, nil
}
