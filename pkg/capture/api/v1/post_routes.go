package v1

import (
	"fmt"
	"net/http"
	"time"

	capconfig "github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/cmd/goProbe/flags"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/status"
	"github.com/go-chi/chi"
)

func (a *API) postRequestRoutes() *chi.Mux {
	r := chi.NewRouter()

	// list actions here
	r.Post("/_reload", a.handleReload)

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
