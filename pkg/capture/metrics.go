package capture

import (
	"testing"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	captureSubsystem        = "capture"
	captureManagerSubsystem = "capture_manager"
)

var promPacketsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "packets_processed_total",
	Help:      "Number of packets processed",
},
	[]string{"iface"},
)
var promBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "bytes_total",
	Help:      "Number of bytes tracked in flow map",
},
	[]string{"iface", "direction"},
)
var promPackets = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "packets_total",
	Help:      "Number of packets seen in flow map",
},
	[]string{"iface", "direction"},
)
var promNumFlows = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "flows_total",
	Help:      "Number of flows tracked in the flow map",
},
	[]string{"iface"},
)
var promPacketsDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "packets_dropped_total",
	Help:      "Number of packets dropped",
},
	[]string{"iface"},
)
var promCaptureErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "errors_total",
	Help:      "Number of errors encountered during packet capture",
},
	[]string{"iface"},
)

var promInterfacesCapturing = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: config.ServiceName,
	Subsystem: captureManagerSubsystem,
	Name:      "interfaces_capturing_total",
	Help:      "Number of interfaces that are actively capturing traffic",
})

// not exposing the interface due to the high-cardinality nature of the histogram
var promRotationDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: config.ServiceName,
	Subsystem: captureManagerSubsystem,
	Name:      "rotation_duration_seconds",
	Help:      "Total flow map rotation time, aggregated across all interfaces",
	// rotation is significantly faster than the writeout. Hence the small buckets
	Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 0.75, 1},
})

func init() {
	prometheus.MustRegister(
		promPacketsProcessed,
		promPacketsDropped,
		promBytes,
		promPackets,
		promNumFlows,
		promCaptureErrors,
		promInterfacesCapturing,
		promRotationDuration,
	)
}

// ResetCounters allows to externally reset all Prometheus counters (e.g. for testing purposes
// or in order to manually reset all of them)
// This method must not (and cannot) be called outside of testing
func ResetCounters() {

	// Check if we are actually calling this function from test code
	if !testing.Testing() {
		panic("cannot reset counter from non-testing code")
	}

	promPacketsProcessed.Reset()
	promBytes.Reset()
	promPackets.Reset()
	promNumFlows.Reset()
	promPacketsDropped.Reset()
	promCaptureErrors.Reset()
}
