package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for MSAKI
type Metrics struct {
	// HTTP metrics
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestSize     *prometheus.SummaryVec
	ResponseSize    *prometheus.SummaryVec

	// Model metrics
	ModelsTotal       prometheus.Gauge
	ModelsRunning     prometheus.Gauge
	ModelStartTotal   *prometheus.CounterVec
	ModelStopTotal    *prometheus.CounterVec
	ModelHealthStatus *prometheus.GaugeVec

	// Proxy metrics
	ProxyRequestsTotal   *prometheus.CounterVec
	ProxyRequestDuration *prometheus.HistogramVec
	ProxyErrors          *prometheus.CounterVec

	// Chat metrics
	ChatMessagesTotal *prometheus.CounterVec
	ChatTokensTotal   *prometheus.CounterVec
}

// New creates a new Metrics instance and registers all metrics
func New() *Metrics {
	m := &Metrics{
		// HTTP metrics
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "msaki_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "msaki_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		RequestSize: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "msaki_http_request_size_bytes",
				Help: "HTTP request size in bytes",
			},
			[]string{"method", "path"},
		),
		ResponseSize: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "msaki_http_response_size_bytes",
				Help: "HTTP response size in bytes",
			},
			[]string{"method", "path"},
		),

		// Model metrics
		ModelsTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "msaki_models_total",
				Help: "Total number of configured models",
			},
		),
		ModelsRunning: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "msaki_models_running",
				Help: "Number of currently running models",
			},
		),
		ModelStartTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "msaki_model_start_total",
				Help: "Total number of model start operations",
			},
			[]string{"model", "status"},
		),
		ModelStopTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "msaki_model_stop_total",
				Help: "Total number of model stop operations",
			},
			[]string{"model", "status"},
		),
		ModelHealthStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "msaki_model_health_status",
				Help: "Health status of models (1=healthy, 0=unhealthy)",
			},
			[]string{"model"},
		),

		// Proxy metrics
		ProxyRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "msaki_proxy_requests_total",
				Help: "Total number of proxied requests",
			},
			[]string{"model", "endpoint", "status"},
		),
		ProxyRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "msaki_proxy_request_duration_seconds",
				Help:    "Proxied request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
			},
			[]string{"model", "endpoint"},
		),
		ProxyErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "msaki_proxy_errors_total",
				Help: "Total number of proxy errors",
			},
			[]string{"model", "error_type"},
		),

		// Chat metrics
		ChatMessagesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "msaki_chat_messages_total",
				Help: "Total number of chat messages",
			},
			[]string{"model", "role"},
		),
		ChatTokensTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "msaki_chat_tokens_total",
				Help: "Total number of tokens processed",
			},
			[]string{"model", "type"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.RequestSize,
		m.ResponseSize,
		m.ModelsTotal,
		m.ModelsRunning,
		m.ModelStartTotal,
		m.ModelStopTotal,
		m.ModelHealthStatus,
		m.ProxyRequestsTotal,
		m.ProxyRequestDuration,
		m.ProxyErrors,
		m.ChatMessagesTotal,
		m.ChatTokensTotal,
	)

	return m
}

// Handler returns the Prometheus HTTP handler
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// Middleware creates a Gin middleware for recording HTTP metrics
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		// Record request size
		reqSize := computeApproximateRequestSize(c.Request)
		m.RequestSize.WithLabelValues(method, path).Observe(float64(reqSize))

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		respSize := float64(c.Writer.Size())

		m.RequestsTotal.WithLabelValues(method, path, status).Inc()
		m.RequestDuration.WithLabelValues(method, path).Observe(duration)
		m.ResponseSize.WithLabelValues(method, path).Observe(respSize)
	}
}

// RecordModelStart records a model start operation
func (m *Metrics) RecordModelStart(model string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.ModelStartTotal.WithLabelValues(model, status).Inc()
}

// RecordModelStop records a model stop operation
func (m *Metrics) RecordModelStop(model string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.ModelStopTotal.WithLabelValues(model, status).Inc()
}

// SetModelHealth sets the health status of a model
func (m *Metrics) SetModelHealth(model string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	m.ModelHealthStatus.WithLabelValues(model).Set(value)
}

// RecordProxyRequest records a proxied request
func (m *Metrics) RecordProxyRequest(model, endpoint string, status int, duration time.Duration) {
	m.ProxyRequestsTotal.WithLabelValues(model, endpoint, strconv.Itoa(status)).Inc()
	m.ProxyRequestDuration.WithLabelValues(model, endpoint).Observe(duration.Seconds())
}

// RecordProxyError records a proxy error
func (m *Metrics) RecordProxyError(model, errorType string) {
	m.ProxyErrors.WithLabelValues(model, errorType).Inc()
}

// RecordChatMessage records a chat message
func (m *Metrics) RecordChatMessage(model, role string) {
	m.ChatMessagesTotal.WithLabelValues(model, role).Inc()
}

// RecordTokens records token usage
func (m *Metrics) RecordTokens(model string, promptTokens, completionTokens int) {
	m.ChatTokensTotal.WithLabelValues(model, "prompt").Add(float64(promptTokens))
	m.ChatTokensTotal.WithLabelValues(model, "completion").Add(float64(completionTokens))
}

// SetModelsTotal sets the total number of configured models
func (m *Metrics) SetModelsTotal(count int) {
	m.ModelsTotal.Set(float64(count))
}

// SetModelsRunning sets the number of running models
func (m *Metrics) SetModelsRunning(count int) {
	m.ModelsRunning.Set(float64(count))
}

// computeApproximateRequestSize computes the approximate request size
func computeApproximateRequestSize(r *http.Request) int {
	size := 0
	if r.URL != nil {
		size += len(r.URL.String())
	}
	size += len(r.Method)
	size += len(r.Proto)
	for name, values := range r.Header {
		size += len(name)
		for _, value := range values {
			size += len(value)
		}
	}
	size += len(r.Host)
	if r.ContentLength != -1 {
		size += int(r.ContentLength)
	}
	return size
}
