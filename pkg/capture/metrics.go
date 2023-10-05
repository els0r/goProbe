package capture

import (
	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	captureSubsystem        = "capture"
	captureManagerSubsystem = "capture_manager"
)

var packetsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "packets_processed_total",
	Help:      "Number of packets processed",
},
	[]string{"iface"},
)
var bytesReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "bytes_received_total",
	Help:      "Number of bytes received",
},
	[]string{"iface"},
)
var bytesSent = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "bytes_received_total",
	Help:      "Number of bytes send",
},
	[]string{"iface"},
)
var numFlows = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "flows_total",
	Help:      "Number of flows present in the flow map",
},
	[]string{"iface"},
)
var packetsDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "packets_dropped_total",
	Help:      "Number of packets dropped",
},
	[]string{"iface"},
)
var captureErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: config.ServiceName,
	Subsystem: captureSubsystem,
	Name:      "errors_total",
	Help:      "Number of errors encountered during packet capture",
},
	[]string{"iface"},
)

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
	Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 0.75, 1},
})

func init() {
	prometheus.MustRegister(
		packetsProcessed,
		packetsDropped,
		bytesReceived,
		bytesSent,
		captureErrors,
		interfacesCapturing,
		rotationDuration,
	)
}
