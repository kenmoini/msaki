package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kenmoini/msaki/internal/auth"
)

// AuthHandler handles authentication-related API endpoints
type AuthHandler struct {
	authManager *auth.AuthManager
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(am *auth.AuthManager) *AuthHandler {
	return &AuthHandler{
		authManager: am,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	Token string     `json:"token"`
	User  *auth.User `json:"user"`
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	user, token, err := h.authManager.Authenticate(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid credentials",
		})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  user,
	})
}

// Logout handles POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// For stateless JWT, logout is handled client-side by discarding the token
	// Here we just return success
	c.JSON(http.StatusOK, gin.H{
		"message": "logged out successfully",
	})
}

// Me handles GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	user := auth.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "not authenticated",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}
