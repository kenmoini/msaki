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

// OllamaChatRequest represents an Ollama chat request
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   *bool           `json:"stream,omitempty"`
}

// OllamaMessage represents a message in the Ollama format
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaGenerateRequest represents an Ollama generate request
type OllamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream *bool  `json:"stream,omitempty"`
}

// OllamaChatHandler handles POST /api/ollama/chat
func (p *Proxy) OllamaChatHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var req OllamaChatRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		if req.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
			return
		}

		model := p.manager.GetModel(req.Model)
		if model == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}

		if model.Status() != models.StatusRunning {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "model is not running"})
			return
		}

		model.UpdateActivity()

		cfg := model.Config()
		targetURL, err := p.buildTargetURL(cfg, model.Port())
		if err != nil {
			p.recordError(req.Model, "invalid_target")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build target URL"})
			return
		}

		// Rewrite path for Ollama backend
		c.Request.URL.Path = "/api/chat"

		proxy := p.createReverseProxy(targetURL, cfg)

		// Ollama defaults to streaming
		shouldStream := req.Stream == nil || *req.Stream
		if shouldStream {
			p.handleStreaming(c, proxy, req.Model, "ollama/chat", start)
			return
		}

		proxy.ServeHTTP(c.Writer, c.Request)

		if p.metrics != nil {
			duration := time.Since(start)
			p.metrics.RecordProxyRequest(req.Model, "ollama/chat", c.Writer.Status(), duration)
		}
	}
}

// OllamaGenerateHandler handles POST /api/ollama/generate
func (p *Proxy) OllamaGenerateHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var req OllamaGenerateRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		if req.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
			return
		}

		model := p.manager.GetModel(req.Model)
		if model == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}

		if model.Status() != models.StatusRunning {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "model is not running"})
			return
		}

		model.UpdateActivity()

		cfg := model.Config()
		targetURL, err := p.buildTargetURL(cfg, model.Port())
		if err != nil {
			p.recordError(req.Model, "invalid_target")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build target URL"})
			return
		}

		// Rewrite path for Ollama backend
		c.Request.URL.Path = "/api/generate"

		proxy := p.createReverseProxy(targetURL, cfg)

		// Ollama defaults to streaming
		shouldStream := req.Stream == nil || *req.Stream
		if shouldStream {
			p.handleStreaming(c, proxy, req.Model, "ollama/generate", start)
			return
		}

		proxy.ServeHTTP(c.Writer, c.Request)

		if p.metrics != nil {
			duration := time.Since(start)
			p.metrics.RecordProxyRequest(req.Model, "ollama/generate", c.Writer.Status(), duration)
		}
	}
}

// OllamaTagsHandler handles GET /api/ollama/tags (lists models)
func (p *Proxy) OllamaTagsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		allModels := p.manager.ListModels()

		// Build Ollama-compatible tags response
		modelList := make([]map[string]interface{}, 0)
		for _, m := range allModels {
			modelList = append(modelList, map[string]interface{}{
				"name":        m.Name,
				"modified_at": time.Now().Format(time.RFC3339),
				"size":        0,
				"digest":      "",
				"details": map[string]interface{}{
					"format": "gguf",
					"family": "msaki",
				},
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"models": modelList,
		})
	}
}
