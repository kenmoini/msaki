package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// setupStaticRoutes configures serving of embedded static files
func (s *Server) setupStaticRoutes(staticFS fs.FS) {
	// Create a file server for the embedded filesystem
	fileServer := http.FileServer(http.FS(staticFS))

	// Serve static files and handle SPA routing
	s.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API routes - they should return 404
		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/v1/") ||
			path == "/health" ||
			path == "/metrics" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "endpoint not found",
			})
			return
		}

		// Try to serve the file directly
		// Check if file exists in the embedded filesystem
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Try to open the file
		file, err := staticFS.Open(cleanPath)
		if err == nil {
			file.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// Try with trailing index.html for directories
		if !strings.HasSuffix(cleanPath, ".html") {
			indexPath := strings.TrimSuffix(cleanPath, "/") + "/index.html"
			file, err := staticFS.Open(indexPath)
			if err == nil {
				file.Close()
				c.Request.URL.Path = "/" + indexPath
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		// SPA fallback: serve index.html for client-side routing
		c.Request.URL.Path = "/index.html"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	// Serve static assets explicitly
	s.router.GET("/", func(c *gin.Context) {
		c.Request.URL.Path = "/index.html"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
