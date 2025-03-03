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

var (
	DefaultMetricsHistogramBins = []float64{0.01, 0.05, 0.1, 0.25, 1, 5, 10, 30, 60, 300}
)

type promMetrics struct {
	promPacketsProcessed *prometheus.CounterVec
	promBytes            *prometheus.CounterVec
	promPackets          *prometheus.CounterVec
	promPacketsDropped   *prometheus.CounterVec
	promCaptureIssues    *prometheus.CounterVec

	promGlobalBufferUsage *prometheus.GaugeVec
	promNumFlows          *prometheus.GaugeVec

	promInterfacesCapturing prometheus.Gauge
	promRotationDuration    prometheus.Histogram
}

// Metrics denotes all capture-specific metrics, tracked in Prometheus
type Metrics struct {
	promMetrics

	trackIfaces bool
}

// MetricsOption denotes a functional option for Metrics
type MetricsOption func(*Metrics)

// DisableIfaceTracking removes the "iface" label from all Prometheus
// metrics (reducing cardinality, in particular if many interfaces from many sensors
// are being tracked)
func DisableIfaceTracking() MetricsOption {
	return func(m *Metrics) {
		m.trackIfaces = false
	}
}

func NewMetrics(opts ...MetricsOption) *Metrics {
	metrics := &Metrics{
		trackIfaces: true,
	}

	for _, opt := range opts {
		opt(metrics)
	}

	labels := []string{}
	if metrics.trackIfaces {
		labels = append(labels, "iface")
	}

	metrics.promMetrics = promMetrics{
		promPacketsProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.ServiceName,
			Subsystem: captureSubsystem,
			Name:      "packets_processed_total",
			Help:      "Number of packets processed",
		},
			labels,
		),
		promBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.ServiceName,
			Subsystem: captureSubsystem,
			Name:      "bytes_total",
			Help:      "Number of bytes tracked in flow map",
		},
			append(labels, "direction"),
		),
		promPackets: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.ServiceName,
			Subsystem: captureSubsystem,
			Name:      "packets_total",
			Help:      "Number of packets seen in flow map",
		},
			append(labels, "direction"),
		),
		promGlobalBufferUsage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.ServiceName,
			Subsystem: captureSubsystem,
			Name:      "global_packet_buffer_usage",
			Help:      "Percentage of global buffer capacity used during flow map rotation",
		},
			labels,
		),
		promNumFlows: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: config.ServiceName,
			Subsystem: captureSubsystem,
			Name:      "flows_total",
			Help:      "Number of flows tracked in the flow map",
		},
			labels,
		),
		promPacketsDropped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.ServiceName,
			Subsystem: captureSubsystem,
			Name:      "packets_dropped_total",
			Help:      "Number of packets dropped",
		},
			labels,
		),
		promCaptureIssues: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: config.ServiceName,
			Subsystem: captureSubsystem,
			Name:      "capture_issues_total",
			Help:      "Number of unexpected issues encountered during packet capture",
		},
			append(labels, "issue_type"),
		),
		promInterfacesCapturing: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: config.ServiceName,
			Subsystem: captureManagerSubsystem,
			Name:      "interfaces_capturing_total",
			Help:      "Number of interfaces that are actively capturing traffic",
		}),
		// not exposing the interface due to the high-cardinality nature of the histogram
		promRotationDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: config.ServiceName,
			Subsystem: captureManagerSubsystem,
			Name:      "rotation_duration_seconds",
			Help:      "Total flow map rotation time, aggregated across all interfaces",
			// rotation is significantly faster than the writeout. Hence the small buckets
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 0.75, 1},
		}),
	}

	prometheus.MustRegister(
		metrics.promPacketsProcessed,
		metrics.promPacketsDropped,
		metrics.promBytes,
		metrics.promPackets,
		metrics.promGlobalBufferUsage,
		metrics.promNumFlows,
		metrics.promCaptureIssues,
		metrics.promInterfacesCapturing,
		metrics.promRotationDuration,
	)

	return metrics
}

func (m *Metrics) ObservePacketsProcessed(iface string) prometheus.Counter {
	if m.trackIfaces {
		return m.promPacketsProcessed.WithLabelValues(iface)
	}
	return m.promPacketsProcessed.WithLabelValues()
}

func (m *Metrics) ObservePacketsDropped(iface string) prometheus.Counter {
	if m.trackIfaces {
		return m.promPacketsDropped.WithLabelValues(iface)
	}
	return m.promPacketsDropped.WithLabelValues()
}

func (m *Metrics) ObserveBytesTotal(iface, direction string) prometheus.Counter {
	if m.trackIfaces {
		return m.promBytes.WithLabelValues(iface, direction)
	}
	return m.promBytes.WithLabelValues(direction)
}

func (m *Metrics) ObservePacketsTotal(iface, direction string) prometheus.Counter {
	if m.trackIfaces {
		return m.promPackets.WithLabelValues(iface, direction)
	}
	return m.promPackets.WithLabelValues(direction)
}

func (m *Metrics) ObserveGlobalBufferUsage(iface string) prometheus.Gauge {
	if m.trackIfaces {
		return m.promGlobalBufferUsage.WithLabelValues(iface)
	}
	return m.promGlobalBufferUsage.WithLabelValues()
}

func (m *Metrics) ObserveNumFlows(iface string) prometheus.Gauge {
	if m.trackIfaces {
		return m.promNumFlows.WithLabelValues(iface)
	}
	return m.promNumFlows.WithLabelValues()
}

func (m *Metrics) ObserveCaptureIssues(iface, issue_type string) prometheus.Counter {
	if m.trackIfaces {
		return m.promCaptureIssues.WithLabelValues(iface, issue_type)
	}
	return m.promCaptureIssues.WithLabelValues(issue_type)
}

func (m *Metrics) ObserveNumIfacesCapturing() prometheus.Gauge {
	return m.promInterfacesCapturing
}

func (m *Metrics) ObserveRotationDuration() prometheus.Histogram {
	return m.promRotationDuration
}

// ResetCountersTestingOnly allows to externally reset all Prometheus counters (e.g. for
// testing purposes or in order to manually reset all of them)
// This method must not (and cannot) be called outside of testing
func (m *Metrics) ResetCountersTestingOnly() {

	// Check if we are actually calling this function from test code
	if !testing.Testing() {
		panic("cannot reset counter from non-testing code")
	}

	m.promPacketsProcessed.Reset()
	m.promBytes.Reset()
	m.promPackets.Reset()
	m.promNumFlows.Reset()
	m.promPacketsDropped.Reset()
	m.promCaptureIssues.Reset()
}
