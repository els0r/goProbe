//go:build linux
// +build linux

package engine

import (
	"context"
	"sync"
	"time"

	"github.com/els0r/goProbe/v4/pkg/capture"
	"github.com/els0r/goProbe/v4/pkg/goDB"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/types/hashmap"
	"github.com/els0r/goProbe/v4/pkg/types/workload"
	"github.com/els0r/telemetry/logging"
	"github.com/fako1024/gotools/concurrency"
)

// QueryRunner implements the Runner interface to execute queries
// against the goDB flow database
type QueryRunner struct {
	query          *goDB.Query
	captureManager *capture.Manager
	dbPath         string

	keepAlive      time.Duration
	sem            concurrency.Semaphore
	stats          *workload.Stats
	statsCallbacks workload.StatsFuncs
}

// WithLiveData adds a capture manager which allows to query live data in addition to the data
// fetched from the DB
func WithLiveData(captureManager *capture.Manager) RunnerOption {
	return func(qr *QueryRunner) {
		qr.captureManager = captureManager
	}
}

func (qr *QueryRunner) runLiveQuery(ctx context.Context, mapChan chan hashmap.AggFlowMapWithMetadata, stmt *query.Statement) (wg *sync.WaitGroup) {
	wg = new(sync.WaitGroup)

	if !stmt.Live {
		return
	}
	// If for some reason a live query was attempted without a CaptureManager running,
	// throw an error and bail
	if qr.captureManager == nil {
		logging.FromContext(ctx).Error("no CaptureManager available, cannot run live query")
		return
	}

	wg.Add(1)
	go func() {
		qr.captureManager.GetFlowMaps(ctx, goDB.QueryFilter(qr.query), mapChan, stmt.Ifaces...)
		wg.Done()
	}()

	return
}
