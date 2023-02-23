package v1

import (
	"fmt"
	"net/http"
	"time"

	capconfig "github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/cmd/goProbe/flags"
	"github.com/els0r/goProbe/pkg/capture"
	"github.com/els0r/goProbe/pkg/discovery"
	"github.com/els0r/goProbe/pkg/logging"
	"github.com/els0r/status"
	"github.com/go-chi/chi/v5"
)

func (a *API) postRequestRoutes(r chi.Router) {
	// list actions here
	r.Post("/_reload", a.handleReload)
	r.Post("/_query", a.handleQuery)
}

func (a *API) handleReload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.WithContext(ctx)

	pp := printPretty(r)

	if pp {
		status.SetOutput(w)
		status.Line("reloading configuration")
	}

	cfg, err := reloadConfig()
	if err != nil {
		logger.Error(err)
		if pp {
			status.Fail(err.Error())
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	a.writeoutHandler.UpdateAndRotate(ctx, cfg.Interfaces, time.Now())

	// return OK
	if pp {
		status.Ok("")
	} else {
		http.Error(w, http.StatusText(http.StatusOK), http.StatusOK)
	}

	// send discovery update
	if a.discoveryConfigUpdate != nil {
		a.discoveryConfigUpdate <- discovery.MakeConfig(cfg)
	}
}

// reloadConfig attempts to reload the configuration file and updates
// the global config if successful.
func reloadConfig() (*capconfig.Config, error) {
	c, err := capconfig.ParseFile(flags.CmdLine.Config)
	c.Lock()
	defer c.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to reload config file: %s", err)
	}
	if c == nil {
		return nil, fmt.Errorf("retrieved <nil> configuration")
	}

	if len(c.Interfaces) > capture.MaxIfaces {
		return nil, fmt.Errorf("cannot monitor more than %d interfaces", capture.MaxIfaces)
	}

	if capconfig.RuntimeDBPath() != c.DB.Path {
		return nil, fmt.Errorf("failed to reload config file: Cannot change database path while running")
	}
	return c, nil
}
