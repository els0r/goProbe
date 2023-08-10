package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var defaultMetricPath = "/metrics"

// Prometheus contains the metrics gathered by the instance and its path. It's been stripped and adapted from
// https://github.com/zsais/go-gin-prometheus/blob/master/middleware.go, the main reason being the lack of support
// for histogram buckets.
//
// For histograms, it uses the experimental NativeHistograms feature
type Prometheus struct {
	reqCnt       *prometheus.CounterVec
	reqSz, resSz prometheus.Summary

	// request duration can be configured
	reqDur         *prometheus.HistogramVec
	reqDurHistOpts *prometheus.HistogramOpts
	reqDurLabels   []string

	additionalMetrics []prometheus.Collector

	router *gin.Engine

	metricsPath string

	// gin.Context string to use as a prometheus URL label
	pathLabelFromContext string
}

// WithMetricsPath sets the metrics path to `path`. The default is `/metrics`.
func (p *Prometheus) WithMetricsPath(path string) *Prometheus {
	p.metricsPath = path
	return p
}

// WithNativeHistograms enables the use of the native prometheus histogram. This is still an experimental feature
func (p *Prometheus) WithNativeHistograms(enabled bool) *Prometheus { //revive:disable-line
	if enabled {
		// Experimental: see documentation on NewHistogram for buckets explanation
		p.reqDurHistOpts.NativeHistogramBucketFactor = 1.1
		p.reqDur = prometheus.NewHistogramVec(*p.reqDurHistOpts, p.reqDurLabels)
	}
	return p
}

// WithRequestDurationBuckets overrides the default buckets for the request duration histogram
func (p *Prometheus) WithRequestDurationBuckets(buckets []float64) *Prometheus {
	p.reqDurHistOpts.Buckets = buckets
	p.reqDur = prometheus.NewHistogramVec(*p.reqDurHistOpts, p.reqDurLabels)
	return p
}

// NewPrometheus generates a new set of metrics with a certain subsystem name. If additionalMetrics is supplied,
// it will register those as well. The `With...` modifiers are meant to be called _before_ they are used/registered
// with gin. Best idea is to call them immediately after NewPrometheus()
func NewPrometheus(serviceName, subsystem string, additionalMetrics ...prometheus.Collector) *Prometheus {
	p := &Prometheus{
		metricsPath: defaultMetricPath,
		// request duration metric configuration
		reqDurHistOpts: &prometheus.HistogramOpts{
			Namespace: serviceName,
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "HTTP request latencies in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		reqDurLabels: []string{"code", "method", "path"},
	}
	p.newMetrics(serviceName, subsystem, additionalMetrics...)
	return p
}

// SetMetricsPath sets the metrics path in the gin.Engine. To control the value of the path, use (*Prometheus).WithMetricsPath
func (p *Prometheus) SetMetricsPath(e *gin.Engine) {
	e.GET(p.metricsPath, prometheusHandler())
}

// SetMetricsPathWithAuth set metrics paths with authentication in the gin.Engine
func (p *Prometheus) SetMetricsPathWithAuth(e *gin.Engine, accounts gin.Accounts) {
	e.GET(p.metricsPath, gin.BasicAuth(accounts), prometheusHandler())
}

func (p *Prometheus) newMetrics(serviceName, subsystem string, additionalMetrics ...prometheus.Collector) {
	// default metrics provided by the middleware library
	reqCntLabels := []string{"host", "handler"}
	reqCntLabels = append(reqCntLabels, p.reqDurLabels...)

	p.reqCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: serviceName,
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "How many HTTP requests processed, partitioned by status code and HTTP method",
		},
		reqCntLabels,
	)
	p.reqDur = prometheus.NewHistogramVec(
		*p.reqDurHistOpts,
		p.reqDurLabels,
	)
	p.reqSz = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: serviceName,
			Subsystem: subsystem,
			Name:      "request_size_bytes",
			Help:      "HTTP request sizes in bytes",
		},
	)
	p.resSz = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: serviceName,
			Subsystem: subsystem,
			Name:      "response_size_bytes",
			Help:      "HTTP reponse sizes in bytes",
		},
	)
	p.additionalMetrics = additionalMetrics
}

func (p *Prometheus) registerMetrics() {
	// register the metrics with prometheus. We use MustRegister to ensure that metrics registration
	// is handled properly (leading to a panic otherwise)
	var toRegister []prometheus.Collector
	toRegister = append(toRegister,
		p.reqCnt,
		p.reqDur,
		p.reqSz, p.resSz,
	)
	toRegister = append(toRegister, p.additionalMetrics...)

	for _, collector := range toRegister {
		collector := collector
		prometheus.MustRegister(collector)
	}
}

func (p *Prometheus) use(e *gin.Engine) {
	p.registerMetrics()
	e.Use(p.HandlerFunc())
}

// Register adds the middleware to a gin engine.
func (p *Prometheus) Register(e *gin.Engine) {
	p.use(e)
	p.SetMetricsPath(e)
}

// RegisterWithAuth adds the middleware to a gin engine with BasicAuth.
func (p *Prometheus) RegisterWithAuth(e *gin.Engine, accounts gin.Accounts) {
	p.use(e)
	p.SetMetricsPathWithAuth(e, accounts)
}

// HandlerFunc defines handler function for middleware
func (p *Prometheus) HandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == p.metricsPath {
			c.Next()
			return
		}

		start := time.Now()
		reqSz := computeApproximateRequestSize(c.Request)

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		elapsed := float64(time.Since(start)) / float64(time.Second)
		resSz := float64(c.Writer.Size())

		path := c.Request.URL.Path
		if len(p.pathLabelFromContext) > 0 {
			pp, found := c.Get(p.pathLabelFromContext)
			if !found {
				pp = "unknown"
			}
			path = pp.(string)
		}
		p.reqDur.WithLabelValues(status, c.Request.Method, path).Observe(elapsed)
		p.reqCnt.WithLabelValues(c.Request.Host, c.HandlerName(), status, c.Request.Method, path).Inc()
		p.reqSz.Observe(float64(reqSz))
		p.resSz.Observe(resSz)
	}
}

func prometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// From https://github.com/DanielHeckrath/gin-prometheus/blob/master/gin_prometheus.go
func computeApproximateRequestSize(r *http.Request) int {
	s := 0
	if r.URL != nil {
		s = len(r.URL.Path)
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.
	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	return s
}
