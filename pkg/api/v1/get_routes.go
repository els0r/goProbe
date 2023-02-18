package v1

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/els0r/goProbe/pkg/api/json"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/status"
	"github.com/go-chi/chi/v5"
)

func (a *API) getRequestRoutes(r chi.Router) {

	// for statistics about the capture
	r.Route("/stats", func(r chi.Router) {
		r.Get("/packets", a.getPacketStats)
		r.Get("/errors", a.getErrors)
	})

	// inspection of flows
	r.Route("/flows", func(r chi.Router) {
		r.Route("/{ifaceName}", func(r chi.Router) {
			r.With(a.IfaceCtx).Get("/", a.getActiveFlows)
		})
	})
}

// The key type is unexported to prevent collisions with context keys defined in
// other packages.
type key int

// context keys for this api
const (
	activeFlowsCtxKey key = iota
)

// IfaceCtx obtains the flow map for the queried interface and stores it in the request context
func (a *API) IfaceCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ifaceName := chi.URLParam(r, "ifaceName")

		flows, err := a.c.ActiveFlows(ifaceName)
		if err != nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		ctx := context.WithValue(r.Context(), activeFlowsCtxKey, flows)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) getActiveFlows(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	flowLog, ok := ctx.Value(activeFlowsCtxKey).(map[string]*capture.FlowLog)
	if !ok {
		http.Error(w, http.StatusText(422), 422)
		return
	}

	// encode answer
	if !printPretty(r) {
		err = json.Response(w, &flowLog)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		// range over all flow maps
		for iface, f := range flowLog {

			tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.AlignRight)

			// table header
			fmt.Fprintf(w, "%s (%d flows):\n", iface, f.Len())
			err = f.TablePrint(tw)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, "\n")
		}
	}
}

func (a *API) getPacketStats(w http.ResponseWriter, r *http.Request) {
	var err error

	stats := a.c.StatusAll()

	// get info for each interface
	var AggregatedStats = struct {
		LoggedReceived uint64 `json:"logged_recieved"`
		Handle         struct {
			Received uint64 `json:"received"`
			Dropped  uint64 `json:"dropped"`
		} `json:"handle"`
		NumIfacesActive int                       `json:"ifaces_active"`
		TotalIfaces     int                       `json:"ifaces_total"`
		LastWriteout    time.Duration             `json:"last_writeout"`
		Ifaces          map[string]capture.Status `json:"ifaces,omitempty"`
	}{}

	AggregatedStats.TotalIfaces = len(stats)
	for _, stat := range stats {
		if stat.Stats.CaptureStats != nil {
			AggregatedStats.LoggedReceived += uint64(stat.Stats.PacketsLogged)
			AggregatedStats.Handle.Received += uint64(stat.Stats.CaptureStats.PacketsReceived)
			AggregatedStats.Handle.Dropped += uint64(stat.Stats.CaptureStats.PacketsDropped)
		}
		if stat.State == capture.StateCapturing {
			AggregatedStats.NumIfacesActive++
		}
	}
	AggregatedStats.LastWriteout = time.Since(a.c.LastRotation).Round(time.Millisecond)

	// check if debug info should be printed
	dbg := r.FormValue("debug")
	if dbg == "1" {
		AggregatedStats.Ifaces = stats
	}

	if !printPretty(r) {
		err = json.Response(w, &AggregatedStats)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		status.SetOutput(w)
		writeLn := func(msg string) {
			_, writeErr := fmt.Fprint(w, msg+"\n")
			if writeErr != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		writeLn("Interface and pcap statistics")

		// print info for each interface
		writeLn(fmt.Sprintf(
			`
   last writeout: %.0fs ago
  packets logged: %d

  handle
        received: %d
         dropped: %d`,
			AggregatedStats.LastWriteout,
			AggregatedStats.LoggedReceived,
			AggregatedStats.Handle.Received,
			AggregatedStats.Handle.Dropped,
		))

		// check if debug info should be printed
		if dbg == "1" {
			writeLn(fmt.Sprintf("\n%s   RCVD     DROP", strings.Repeat(" ", status.StatusLineIndent+8+4)))

			for iface, stat := range AggregatedStats.Ifaces {
				var pcapInfoStr string
				if stat.Stats.CaptureStats != nil {
					pcapInfoStr = fmt.Sprintf("%8d %8d %8d",
						stat.Stats.CaptureStats.PacketsReceived,
						stat.Stats.CaptureStats.PacketsDropped,
					)
				}

				status.Line(iface)
				switch stat.State {
				case capture.StateInitializing:
					status.Warn("initializing")
				case capture.StateCapturing:
					status.Ok(pcapInfoStr)
				case capture.StateError:
					status.Fail("error")
				default:
					status.Custom(status.White, "NONE", "Unknown capture state")
				}
			}
		}

		writeLn("")
		status.Line("Total")

		activeStr := fmt.Sprintf("%d/%d interfaces active", AggregatedStats.NumIfacesActive, AggregatedStats.TotalIfaces)
		if AggregatedStats.NumIfacesActive != AggregatedStats.TotalIfaces {
			if AggregatedStats.NumIfacesActive == 0 {
				status.Fail(activeStr)
			} else {
				status.Warn(activeStr)
			}
		} else {
			status.Ok(activeStr)
		}
	}
}

func (a *API) getErrors(w http.ResponseWriter, r *http.Request) {
	var err error

	errors := a.c.ErrorsAll()

	if !printPretty(r) {
		err = json.Response(w, errors)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	} else {
		status.SetOutput(w)

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
}
