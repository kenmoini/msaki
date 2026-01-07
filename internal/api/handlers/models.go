package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kenmoini/msaki/internal/models"
)

// ModelsHandler handles model-related API endpoints
type ModelsHandler struct {
	manager *models.Manager
}

// NewModelsHandler creates a new models handler
func NewModelsHandler(manager *models.Manager) *ModelsHandler {
	return &ModelsHandler{
		manager: manager,
	}
}

// List handles GET /api/models
func (h *ModelsHandler) List(c *gin.Context) {
	modelList := h.manager.ListModels()
	result := make([]*models.ModelDTO, 0, len(modelList))

	for _, m := range modelList {
		result = append(result, m.ToDTO())
	}

	c.JSON(http.StatusOK, result)
}

// Get handles GET /api/models/:name
func (h *ModelsHandler) Get(c *gin.Context) {
	name := c.Param("name")
	model := h.manager.GetModel(name)

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model not found",
		})
		return
	}

	c.JSON(http.StatusOK, model.ToDTO())
}

// Start handles POST /api/models/:name/start
func (h *ModelsHandler) Start(c *gin.Context) {
	name := c.Param("name")
	model := h.manager.GetModel(name)

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model not found",
		})
		return
	}

	if model.IsExternal() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "external models cannot be started",
		})
		return
	}

	if err := h.manager.Start(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "model start initiated",
		"model":   model.ToDTO(),
	})
}

// Stop handles POST /api/models/:name/stop
func (h *ModelsHandler) Stop(c *gin.Context) {
	name := c.Param("name")
	model := h.manager.GetModel(name)

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model not found",
		})
		return
	}

	if model.IsExternal() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "external models cannot be stopped",
		})
		return
	}

	if err := h.manager.Stop(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "model stop initiated",
		"model":   model.ToDTO(),
	})
}

// Health handles GET /api/models/:name/health
func (h *ModelsHandler) Health(c *gin.Context) {
	name := c.Param("name")
	model := h.manager.GetModel(name)

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"model":   model.Name(),
		"healthy": model.IsHealthy(),
		"message": model.HealthMessage(),
		"status":  model.Status(),
	})
}

// Restart handles POST /api/models/:name/restart
func (h *ModelsHandler) Restart(c *gin.Context) {
	name := c.Param("name")
	model := h.manager.GetModel(name)

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model not found",
		})
		return
	}

	if model.IsExternal() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "external models cannot be restarted",
		})
		return
	}

	if err := h.manager.Restart(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "model restart initiated",
		"model":   model.ToDTO(),
	})
}

// Logs handles GET /api/models/:name/logs
func (h *ModelsHandler) Logs(c *gin.Context) {
	name := c.Param("name")
	model := h.manager.GetModel(name)

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model not found",
		})
		return
	}

	// Check for 'since' parameter (Unix timestamp in milliseconds)
	var logs []models.LogEntry
	sinceStr := c.Query("since")
	if sinceStr != "" {
		sinceMs, err := strconv.ParseInt(sinceStr, 10, 64)
		if err == nil {
			since := time.UnixMilli(sinceMs)
			logs = model.GetLogsSince(since)
		} else {
			logs = model.GetLogs()
		}
	} else {
		logs = model.GetLogs()
	}

	c.JSON(http.StatusOK, gin.H{
		"model":  model.Name(),
		"status": model.Status(),
		"logs":   logs,
	})
}

// LogsStream handles GET /api/models/:name/logs/stream (SSE)
func (h *ModelsHandler) LogsStream(c *gin.Context) {
	name := c.Param("name")
	model := h.manager.GetModel(name)

	if model == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "model not found",
		})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Track last sent timestamp
	lastSent := time.Now().Add(-1 * time.Hour) // Start from an hour ago to get existing logs

	// Send initial logs
	logs := model.GetLogs()
	for _, entry := range logs {
		c.SSEvent("log", entry)
		if entry.Timestamp.After(lastSent) {
			lastSent = entry.Timestamp
		}
	}
	c.Writer.Flush()

	// Poll for new logs
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Also send status updates
	lastStatus := model.Status()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			// Check for new logs
			newLogs := model.GetLogsSince(lastSent)
			for _, entry := range newLogs {
				c.SSEvent("log", entry)
				if entry.Timestamp.After(lastSent) {
					lastSent = entry.Timestamp
				}
			}

			// Check for status change
			currentStatus := model.Status()
			if currentStatus != lastStatus {
				c.SSEvent("status", gin.H{
					"status":      currentStatus,
					"statusError": model.StatusError(),
				})
				lastStatus = currentStatus
			}

			c.Writer.Flush()

			// Stop streaming if model reached a terminal state
			if currentStatus == models.StatusRunning ||
				currentStatus == models.StatusStopped ||
				currentStatus == models.StatusError {
				// Give it one more second to capture final logs
				time.Sleep(1 * time.Second)
				newLogs = model.GetLogsSince(lastSent)
				for _, entry := range newLogs {
					c.SSEvent("log", entry)
				}
				c.SSEvent("done", gin.H{"status": currentStatus})
				c.Writer.Flush()
				return
			}
		}
	}
}
