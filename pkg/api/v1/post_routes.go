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

func (a *API) postRequestRoutes(r chi.Router) {
	// list actions here
	r.Post("/_reload", a.handleReload)
}

func (a *API) handleReload(w http.ResponseWriter, r *http.Request) {
	pp := r.FormValue("pretty") == "1"

	if pp {
		status.SetOutput(w)
		status.Line("Reloading configuration")
	}

	var writeoutsChan chan<- capture.Writeout = a.c.WriteoutHandler.WriteoutChan

	capconfig.Mutex.Lock()
	defer capconfig.Mutex.Unlock()
	config, err := reloadConfig()
	if err != nil {
		if a.logger != nil {
			a.logger.Error(err.Error())
		}
		if pp {
			status.Fail(err.Error())
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	woChan := make(chan capture.TaggedAggFlowMap, capture.MaxIfaces)
	writeoutsChan <- capture.Writeout{woChan, time.Now()}
	a.c.Update(config.Interfaces, woChan)
	close(woChan)

	// return OK
	if pp {
		status.Ok("")
	} else {
		http.Error(w, http.StatusText(http.StatusOK), http.StatusOK)
	}
}

// reloadConfig attempts to reload the configuration file and updates
// the global config if successful.
func reloadConfig() (*capconfig.Config, error) {
	c, err := capconfig.ParseFile(flags.CmdLine.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to reload config file: %s", err)
	}
	if c == nil {
		return nil, fmt.Errorf("retrieved <nil> configuration")
	}

	if len(c.Interfaces) > capture.MaxIfaces {
		return nil, fmt.Errorf("cannot monitor more than %d interfaces", capture.MaxIfaces)
	}

	if capconfig.RuntimeDBPath() != c.DBPath {
		return nil, fmt.Errorf("failed to reload config file: Cannot change database path while running")
	}
	return c, nil
}
