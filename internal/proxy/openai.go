package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kenmoini/msaki/internal/models"
)

// OpenAIChatRequest represents an OpenAI chat completion request
type OpenAIChatRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
}

// OpenAIMessage represents a message in the OpenAI format
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionsHandler handles POST /v1/chat/completions
func (p *Proxy) ChatCompletionsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Read and parse request body to get model name
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": map[string]interface{}{
					"message": "failed to read request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		// Restore body for proxy
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var req OpenAIChatRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": map[string]interface{}{
					"message": "invalid JSON in request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		if req.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": map[string]interface{}{
					"message": "model is required",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		model := p.manager.GetModel(req.Model)
		if model == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": map[string]interface{}{
					"message": "model not found: " + req.Model,
					"type":    "invalid_request_error",
				},
			})
			return
		}

		if model.Status() != models.StatusRunning {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": map[string]interface{}{
					"message": "model is not running",
					"type":    "server_error",
				},
			})
			return
		}

		// Update model activity
		model.UpdateActivity()

		cfg := model.Config()
		targetURL, err := p.buildTargetURL(cfg, model.Port())
		if err != nil {
			p.recordError(req.Model, "invalid_target")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": map[string]interface{}{
					"message": "failed to build target URL",
					"type":    "server_error",
				},
			})
			return
		}

		proxy := p.createReverseProxy(targetURL, cfg)

		// Handle streaming
		if req.Stream {
			p.handleStreaming(c, proxy, req.Model, "chat/completions", start)
			return
		}

		// Standard proxy
		proxy.ServeHTTP(c.Writer, c.Request)

		if p.metrics != nil {
			duration := time.Since(start)
			p.metrics.RecordProxyRequest(req.Model, "chat/completions", c.Writer.Status(), duration)
		}
	}
}

// CompletionsHandler handles POST /v1/completions
func (p *Proxy) CompletionsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Read and parse request body to get model name
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": map[string]interface{}{
					"message": "failed to read request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		// Restore body for proxy
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var req struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream,omitempty"`
		}
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": map[string]interface{}{
					"message": "invalid JSON in request body",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		if req.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": map[string]interface{}{
					"message": "model is required",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		model := p.manager.GetModel(req.Model)
		if model == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": map[string]interface{}{
					"message": "model not found: " + req.Model,
					"type":    "invalid_request_error",
				},
			})
			return
		}

		if model.Status() != models.StatusRunning {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": map[string]interface{}{
					"message": "model is not running",
					"type":    "server_error",
				},
			})
			return
		}

		model.UpdateActivity()

		cfg := model.Config()
		targetURL, err := p.buildTargetURL(cfg, model.Port())
		if err != nil {
			p.recordError(req.Model, "invalid_target")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": map[string]interface{}{
					"message": "failed to build target URL",
					"type":    "server_error",
				},
			})
			return
		}

		proxy := p.createReverseProxy(targetURL, cfg)

		if req.Stream {
			p.handleStreaming(c, proxy, req.Model, "completions", start)
			return
		}

		proxy.ServeHTTP(c.Writer, c.Request)

		if p.metrics != nil {
			duration := time.Since(start)
			p.metrics.RecordProxyRequest(req.Model, "completions", c.Writer.Status(), duration)
		}
	}
}

// ModelsListHandler handles GET /v1/models
func (p *Proxy) ModelsListHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		allModels := p.manager.ListModels()

		// Build OpenAI-compatible models response
		modelList := make([]map[string]interface{}, 0)
		for _, m := range allModels {
			modelList = append(modelList, map[string]interface{}{
				"id":       m.Name,
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "msaki",
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   modelList,
		})
	}
}
