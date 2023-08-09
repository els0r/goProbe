package writeout

import (
	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	writeoutSubsystem = "godb_handler"
)

var writeoutDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: config.ServiceName,
	Subsystem: writeoutSubsystem,
	Name:      "writeout_duration_seconds",
	Help:      "Total flow data writeout time, aggregated across all interfaces written to DB",
	// these buckets should capture disks of various speed and setups with many interfaces
	Buckets: []float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 5, 10, 30, 60},
})

func init() {
	prometheus.MustRegister(
		writeoutDuration,
	)
}
