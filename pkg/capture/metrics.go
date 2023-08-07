package capture

import (
	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	captureSubsystem        = "capture"
	captureManagerSubsystem = "capture_manager"
)

var packetsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "packets_processed_total",
	Help:      "Number of packets processed, aggregated over all interfaces",
})
var packetsDropped = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "packets_dropped_total",
	Help:      "Number of packets dropped, aggregated over all interfaces",
})
var captureErrors = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "errors_total",
	Help:      "Number of errors encountered during packet capture, aggregated over all interfaces",
})

var interfacesCapturing = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: config.ServiceName,
	Subsystem: captureManagerSubsystem,
	Name:      "interfaces_capturing_total",
	Help:      "Number of interfaces that are actively capturing traffic",
})

var rotationDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: config.ServiceName,
	Subsystem: captureManagerSubsystem,
	Name:      "rotation_duration_seconds",
	Help:      "Total flow map rotation time, aggregated across all interfaces",
	// rotation is significantly faster than the writeout. Hence the small buckets
	Buckets: []float64{0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
})

func init() {
	prometheus.MustRegister(
		packetsProcessed,
		packetsDropped,
		captureErrors,
		interfacesCapturing,
		rotationDuration,
	)
}
