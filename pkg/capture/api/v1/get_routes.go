package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/status"
	"github.com/go-chi/chi"
)

func (a *API) getRequestRoutes() *chi.Mux {
	r := chi.NewRouter()

	r.Route("/stats", func(r chi.Router) {
		// list actions here
		r.Get("/packets", a.getPacketStats)
		r.Get("/errors", a.getErrors)
	})

	r.Route("/flows", func(r chi.Router) {
		r.Route("/{ifaceName}", func(r chi.Router) {
			r.With(a.IfaceCtx).Get("/", a.getActiveFlows)
		})
	})

	return r
}

func (a *API) IfaceCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ifaceName := chi.URLParam(r, "ifaceName")

		flows, err := a.c.ActiveFlows(ifaceName)
		if err != nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}
		ctx := context.WithValue(r.Context(), "activeFlows", flows)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) getActiveFlows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flows, ok := ctx.Value("activeFlows").(map[string]*capture.FlowLog)
	if !ok {
		http.Error(w, http.StatusText(422), 422)
		return
	}

	// encode answer
	err := json.NewEncoder(w).Encode(&flows)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (a *API) getPacketStats(w http.ResponseWriter, r *http.Request) {
	var (
		pretty string
		err    error
	)
	pretty = r.FormValue("pretty")
	if pretty == "" {
		pretty = "0"
	}

	stats := a.c.StatusAll()

	// get info for each interface
	var AggregatedStats = struct {
		LoggedRcvd   uint64                    `json:"logged_rcvd"`
		PcapRcvd     uint64                    `json:"pcap_rcvd"`
		PcapDrop     uint64                    `json:"pcap_drop"`
		PcapIfDrop   uint64                    `json:"pcap_ifdrop"`
		NumActive    int                       `json:"iface_active"`
		TotalIfaces  int                       `json:"iface_total"`
		LastWriteout float64                   `json:"last_writeout"`
		Ifaces       map[string]capture.Status `json:"ifaces,omitempty"`
	}{}

	AggregatedStats.TotalIfaces = len(stats)
	for _, stat := range stats {
		if stat.Stats.Pcap != nil {
			AggregatedStats.LoggedRcvd += uint64(stat.Stats.PacketsLogged)
			AggregatedStats.PcapRcvd += uint64(stat.Stats.Pcap.PacketsReceived)
			AggregatedStats.PcapDrop += uint64(stat.Stats.Pcap.PacketsDropped)
			AggregatedStats.PcapIfDrop += uint64(stat.Stats.Pcap.PacketsIfDropped)
		}
		if stat.State == capture.CAPTURE_STATE_ACTIVE {
			AggregatedStats.NumActive++
		}
	}
	AggregatedStats.LastWriteout = time.Now().Sub(a.c.LastRotation).Seconds()

	// check if debug info should be printed
	dbg := r.FormValue("debug")
	if dbg == "1" {
		AggregatedStats.Ifaces = stats
	}

	if pretty == "0" {
		err = json.NewEncoder(w).Encode(&AggregatedStats)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	if pretty == "1" {
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

   pcap received: %d
         dropped: %d
   iface dropped: %d`,
			AggregatedStats.LastWriteout,
			AggregatedStats.LoggedRcvd,
			AggregatedStats.PcapRcvd,
			AggregatedStats.PcapDrop,
			AggregatedStats.PcapIfDrop,
		))

		// check if debug info should be printed
		if dbg == "1" {
			writeLn(fmt.Sprintf("\n%s   RCVD     DROP   IFDROP", strings.Repeat(" ", status.StatusLineIndent+8+4)))

			for iface, stat := range AggregatedStats.Ifaces {
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

		activeStr := fmt.Sprintf("%d/%d interfaces active", AggregatedStats.NumActive, AggregatedStats.TotalIfaces)
		if AggregatedStats.NumActive != AggregatedStats.TotalIfaces {
			if AggregatedStats.NumActive == 0 {
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
	var (
		pretty string
		err    error
	)
	pretty = r.FormValue("pretty")
	if pretty == "" {
		pretty = "0"
	}

	errors := a.c.ErrorsAll()

	if pretty == "0" {
		err = json.NewEncoder(w).Encode(errors)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}

	if pretty == "1" {
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
