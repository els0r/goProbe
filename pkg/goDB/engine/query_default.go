//go:build !linux
// +build !linux

package engine

import (
	"context"
	"sync"
	"time"

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
	query  *goDB.Query
	dbPath string

	keepAlive      time.Duration
	sem            concurrency.Semaphore
	stats          *workload.Stats
	statsCallbacks workload.StatsFuncs
}

func (qr *QueryRunner) runLiveQuery(ctx context.Context, _ chan hashmap.AggFlowMapWithMetadata, stmt *query.Statement) (wg *sync.WaitGroup) {
	wg = new(sync.WaitGroup)

	if !stmt.Live {
		return
	}

	logging.FromContext(ctx).Error("unsupported OS / architecture, cannot run live query")
	return

}
