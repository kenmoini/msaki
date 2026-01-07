package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKeyUser is the key for storing user info in context
	ContextKeyUser = "msaki_user"
)

// Middleware creates a Gin middleware for authentication
func Middleware(am *AuthManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authorization header required",
			})
			return
		}

		// Check for Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format",
			})
			return
		}

		token := parts[1]

		// Validate token
		user, err := am.ValidateToken(token)
		if err != nil {
			status := http.StatusUnauthorized
			message := "invalid token"
			if err == ErrExpiredToken {
				message = "token has expired"
			}
			c.AbortWithStatusJSON(status, gin.H{
				"error": message,
			})
			return
		}

		// Store user in context
		c.Set(ContextKeyUser, user)
		c.Next()
	}
}

// OptionalMiddleware creates a Gin middleware that extracts user info if present
// but does not require authentication
func OptionalMiddleware(am *AuthManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		token := parts[1]
		user, err := am.ValidateToken(token)
		if err == nil {
			c.Set(ContextKeyUser, user)
		}

		c.Next()
	}
}

// RequireRole creates a middleware that requires a specific role
func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool)
	for _, role := range roles {
		roleSet[role] = true
	}

	return func(c *gin.Context) {
		userVal, exists := c.Get(ContextKeyUser)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		user, ok := userVal.(*User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "invalid user context",
			})
			return
		}

		if !roleSet[user.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			return
		}

		c.Next()
	}
}

// GetUser retrieves the authenticated user from the Gin context
func GetUser(c *gin.Context) *User {
	userVal, exists := c.Get(ContextKeyUser)
	if !exists {
		return nil
	}
	user, ok := userVal.(*User)
	if !ok {
		return nil
	}
	return user
}
