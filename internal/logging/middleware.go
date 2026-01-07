package logging

import (
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLogMiddleware creates a Gin middleware for access logging
func AccessLogMiddleware(lm *LogManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get client IP
		clientIP := c.ClientIP()

		// Get status code
		statusCode := c.Writer.Status()

		// Get response size
		responseSize := c.Writer.Size()

		// Build query string if present
		if raw != "" {
			path = path + "?" + raw
		}

		// Log the request
		lm.Info("%s | %3d | %13v | %15s | %-7s %s | %d bytes",
			start.Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			c.Request.Method,
			path,
			responseSize,
		)
	}
}

// ErrorLogMiddleware creates a Gin middleware for error logging
func ErrorLogMiddleware(lm *LogManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Log any errors that occurred
		for _, err := range c.Errors {
			lm.Error("%s | %s %s | %v",
				time.Now().Format("2006/01/02 - 15:04:05"),
				c.Request.Method,
				c.Request.URL.Path,
				err.Err,
			)
		}
	}
}
