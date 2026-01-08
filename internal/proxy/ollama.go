package proxy

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kenmoini/msaki/internal/config"
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build target URL in OllamaChatHandler " + err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build target URL in OllamaGenerateHandler " + err.Error()})
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

// OllamaChatResponse represents an Ollama chat response
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// proxyToOllama handles proxying an OpenAI-format chat request to an Ollama backend
func (p *Proxy) proxyToOllama(c *gin.Context, model *models.Model, openaiReq *OpenAIChatRequest, bodyBytes []byte, start time.Time) {
	cfg := model.Config()
	targetURL, err := p.buildTargetURL(cfg, model.Port())
	if err != nil {
		p.recordError(openaiReq.Model, "invalid_target")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"message": "failed to build target URL in proxyToOllama " + err.Error(),
				"type":    "server_error",
			},
		})
		return
	}

	// Translate OpenAI request to Ollama format
	ollamaReq := translateOpenAIToOllama(openaiReq, cfg.ModelName)
	ollamaBody, err := json.Marshal(ollamaReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"message": "failed to translate request",
				"type":    "server_error",
			},
		})
		return
	}

	// Create request to Ollama /api/chat endpoint
	ollamaURL := targetURL.String() + "/api/chat"
	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", ollamaURL, bytes.NewBuffer(ollamaBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"message": "failed to create request",
				"type":    "server_error",
			},
		})
		return
	}

	req.Header.Set("Content-Type", "application/json")

	// Add API key if configured
	apiKey := getAPIKey(cfg)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := createHTTPClient(cfg)
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": map[string]interface{}{
				"message": "failed to connect to Ollama backend: " + err.Error(),
				"type":    "server_error",
			},
		})
		return
	}
	defer resp.Body.Close()

	if openaiReq.Stream {
		p.streamOllamaToOpenAI(c, resp, openaiReq.Model, start)
	} else {
		p.translateOllamaResponse(c, resp, openaiReq.Model)
	}
}

// translateOpenAIToOllama converts an OpenAI chat request to Ollama format
func translateOpenAIToOllama(openaiReq *OpenAIChatRequest, backendModel string) *OllamaChatRequest {
	messages := make([]OllamaMessage, len(openaiReq.Messages))
	for i, msg := range openaiReq.Messages {
		messages[i] = OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Use the backend model name if specified, otherwise use the request model
	modelName := backendModel
	if modelName == "" {
		modelName = openaiReq.Model
	}

	stream := openaiReq.Stream
	return &OllamaChatRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   &stream,
	}
}

// streamOllamaToOpenAI streams Ollama responses and translates to OpenAI SSE format
func (p *Proxy) streamOllamaToOpenAI(c *gin.Context, resp *http.Response, modelName string, start time.Time) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"message": "streaming not supported",
				"type":    "server_error",
			},
		})
		return
	}

	decoder := json.NewDecoder(resp.Body)
	messageID := generateMessageID()

	for {
		var ollamaResp OllamaChatResponse
		if err := decoder.Decode(&ollamaResp); err != nil {
			if err == io.EOF {
				break
			}
			break
		}

		// Translate to OpenAI streaming format
		chunk := map[string]interface{}{
			"id":      messageID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   modelName,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": ollamaResp.Message.Content,
					},
					"finish_reason": nil,
				},
			},
		}

		if ollamaResp.Done {
			chunk["choices"].([]map[string]interface{})[0]["finish_reason"] = "stop"
			chunk["choices"].([]map[string]interface{})[0]["delta"] = map[string]interface{}{}
		}

		data, _ := json.Marshal(chunk)
		c.Writer.Write([]byte("data: " + string(data) + "\n\n"))
		flusher.Flush()

		if ollamaResp.Done {
			c.Writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			break
		}
	}

	if p.metrics != nil {
		duration := time.Since(start)
		p.metrics.RecordProxyRequest(modelName, "chat/completions", http.StatusOK, duration)
	}
}

// translateOllamaResponse translates a non-streaming Ollama response to OpenAI format
func (p *Proxy) translateOllamaResponse(c *gin.Context, resp *http.Response, modelName string) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	var ollamaResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"message": "failed to parse Ollama response",
				"type":    "server_error",
			},
		})
		return
	}

	// Translate to OpenAI format
	openaiResp := map[string]interface{}{
		"id":      generateMessageID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   modelName,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    ollamaResp.Message.Role,
					"content": ollamaResp.Message.Content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		},
	}

	c.JSON(http.StatusOK, openaiResp)
}

// createHTTPClient creates an HTTP client with the appropriate TLS config
func createHTTPClient(cfg *config.ModelConfig) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.SkipTLSVerify,
		},
		ResponseHeaderTimeout: 5 * time.Minute,
		IdleConnTimeout:       90 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute,
	}
}

// generateMessageID creates a unique message ID
func generateMessageID() string {
	return "chatcmpl-" + time.Now().Format("20060102150405.000")
}
