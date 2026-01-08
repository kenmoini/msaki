package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kenmoini/msaki/internal/config"
	"github.com/kenmoini/msaki/internal/metrics"
	"github.com/kenmoini/msaki/internal/models"
)

// Proxy handles reverse proxying requests to model backends
type Proxy struct {
	manager *models.Manager
	metrics *metrics.Metrics
	queue   *RequestQueue
}

// New creates a new Proxy instance
func New(manager *models.Manager, m *metrics.Metrics) *Proxy {
	p := &Proxy{
		manager: manager,
		metrics: m,
		queue:   NewRequestQueue(),
	}

	// Register callback for when models become ready
	manager.OnModelReady(func(modelName string) {
		p.queue.NotifyAll(modelName)
	})

	// Register callback for when models fail to start
	manager.OnModelError(func(modelName string) {
		p.queue.FailAll(modelName, ErrModelStartFailed)
	})

	return p
}

// waitForModel queues a request and waits for the model to become ready
// Returns nil if model is ready, or an error if timeout/failure occurred
func (p *Proxy) waitForModel(modelName string) error {
	req := &QueuedRequest{
		Done:      make(chan error, 1),
		CreatedAt: time.Now(),
	}

	p.queue.Enqueue(modelName, req)

	// Wait for model to become ready or timeout
	select {
	case err := <-req.Done:
		return err
	case <-time.After(QueueTimeout):
		return ErrQueueTimeout
	}
}

// Handler returns a Gin handler for proxying requests to a specific model
func (p *Proxy) Handler(modelName string, endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		model := p.manager.GetModel(modelName)
		if model == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "model not found",
			})
			return
		}

		if model.Status() != models.StatusRunning {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "model is not running",
			})
			return
		}

		cfg := model.Config()
		targetURL, err := p.buildTargetURL(cfg, model.Port())
		if err != nil {
			p.recordError(modelName, "invalid_target")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to build target URL in Handler : " + err.Error(),
			})
			return
		}

		proxy := p.createReverseProxy(targetURL, cfg)

		// Handle streaming responses
		if isStreamingRequest(c) {
			p.handleStreaming(c, proxy, modelName, endpoint, start)
			return
		}

		// Standard proxy handling
		proxy.ServeHTTP(c.Writer, c.Request)

		// Record metrics
		if p.metrics != nil {
			duration := time.Since(start)
			p.metrics.RecordProxyRequest(modelName, endpoint, c.Writer.Status(), duration)
		}
	}
}

// DynamicHandler handles requests where the model is specified in the request
func (p *Proxy) DynamicHandler(endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Get model from query param or header
		modelName := c.Query("model")
		if modelName == "" {
			modelName = c.GetHeader("X-Model")
		}

		if modelName == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "model not specified",
			})
			return
		}

		model := p.manager.GetModel(modelName)
		if model == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "model not found",
			})
			return
		}

		if model.Status() != models.StatusRunning {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "model is not running",
			})
			return
		}

		cfg := model.Config()
		targetURL, err := p.buildTargetURL(cfg, model.Port())
		if err != nil {
			p.recordError(modelName, "invalid_target")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to build target URL in DynamicHandler : " + err.Error(),
			})
			return
		}

		proxy := p.createReverseProxy(targetURL, cfg)

		// Handle streaming responses
		if isStreamingRequest(c) {
			p.handleStreaming(c, proxy, modelName, endpoint, start)
			return
		}

		// Standard proxy handling
		proxy.ServeHTTP(c.Writer, c.Request)

		// Record metrics
		if p.metrics != nil {
			duration := time.Since(start)
			p.metrics.RecordProxyRequest(modelName, endpoint, c.Writer.Status(), duration)
		}
	}
}

func (p *Proxy) buildTargetURL(cfg *config.ModelConfig, port int) (*url.URL, error) {
	// Use BackendOverride if set, otherwise use Endpoint
	baseURL := cfg.BackendOverride
	if baseURL == "" {
		baseURL = cfg.Endpoint
	}
	// For script-based models, use localhost with allocated port
	if baseURL == "" && port > 0 {
		baseURL = fmt.Sprintf("http://localhost:%d", port)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("no base URL available")
	}
	return url.Parse(baseURL)
}

func (p *Proxy) createReverseProxy(target *url.URL, cfg *config.ModelConfig) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Configure transport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.SkipTLSVerify,
		},
		ResponseHeaderTimeout: 120 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}
	proxy.Transport = transport

	// Modify request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Set host header
		req.Host = target.Host

		// Add API key if configured
		apiKey := getAPIKey(cfg)
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		// Remove hop-by-hop headers
		removeHopHeaders(req.Header)
	}

	// Handle errors
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Proxy error: "+err.Error(), http.StatusBadGateway)
	}

	return proxy
}

func (p *Proxy) handleStreaming(c *gin.Context, proxy *httputil.ReverseProxy, modelName, endpoint string, start time.Time) {
	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Create a custom response writer that flushes
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "streaming not supported",
		})
		return
	}

	// Modify proxy to handle streaming
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Copy headers
		for k, vv := range resp.Header {
			for _, v := range vv {
				c.Header(k, v)
			}
		}
		c.Status(resp.StatusCode)

		// Stream the response
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				c.Writer.Write(buf[:n])
				flusher.Flush()
			}
			if err != nil {
				if err != io.EOF {
					// Log error but don't break - client may have disconnected
				}
				break
			}
		}

		return nil
	}

	proxy.ServeHTTP(c.Writer, c.Request)

	// Record metrics
	if p.metrics != nil {
		duration := time.Since(start)
		p.metrics.RecordProxyRequest(modelName, endpoint, c.Writer.Status(), duration)
	}
}

func (p *Proxy) recordError(model, errorType string) {
	if p.metrics != nil {
		p.metrics.RecordProxyError(model, errorType)
	}
}

// isStreamingRequest checks if the request expects streaming response
func isStreamingRequest(c *gin.Context) bool {
	// Check Accept header for SSE
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "text/event-stream") {
		return true
	}
	return false
}

// getAPIKey retrieves the API key from env var or file as configured
func getAPIKey(cfg *config.ModelConfig) string {
	// Try env var first
	if cfg.APIKeyEnv != "" {
		if key := os.Getenv(cfg.APIKeyEnv); key != "" {
			return key
		}
	}
	// Then try file
	if cfg.APIKeyPath != "" {
		data, err := os.ReadFile(cfg.APIKeyPath)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// removeHopHeaders removes hop-by-hop headers
func removeHopHeaders(header http.Header) {
	hopHeaders := []string{
		"Connection",
		"Proxy-Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	}
	for _, h := range hopHeaders {
		header.Del(h)
	}
}
