package handlers

import (
	"net/http"

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
