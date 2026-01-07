package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// setupStaticRoutes configures serving of embedded static files
func (s *Server) setupStaticRoutes(staticFS fs.FS) {
	// Serve root path explicitly
	s.router.GET("/", func(c *gin.Context) {
		serveStaticFile(c, staticFS, "index.html")
	})

	// Serve static files and handle SPA routing
	s.router.NoRoute(func(c *gin.Context) {
		urlPath := c.Request.URL.Path

		// Skip API routes - they should return 404
		if strings.HasPrefix(urlPath, "/api/") ||
			strings.HasPrefix(urlPath, "/v1/") ||
			urlPath == "/health" ||
			urlPath == "/metrics" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "endpoint not found",
			})
			return
		}

		// Clean the path and try to serve
		cleanPath := strings.TrimPrefix(urlPath, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Try exact file
		if tryServeFile(c, staticFS, cleanPath) {
			return
		}

		// Try with .html extension
		if !strings.Contains(path.Base(cleanPath), ".") {
			if tryServeFile(c, staticFS, cleanPath+".html") {
				return
			}
		}

		// Try as directory index
		if tryServeFile(c, staticFS, path.Join(cleanPath, "index.html")) {
			return
		}

		// SPA fallback: serve index.html
		serveStaticFile(c, staticFS, "index.html")
	})
}

// tryServeFile attempts to serve a file, returns true if successful
func tryServeFile(c *gin.Context, staticFS fs.FS, filePath string) bool {
	content, err := fs.ReadFile(staticFS, filePath)
	if err != nil {
		return false
	}

	contentType := getContentType(filePath)
	c.Data(http.StatusOK, contentType, content)
	return true
}

// serveStaticFile serves a file or returns 404
func serveStaticFile(c *gin.Context, staticFS fs.FS, filePath string) {
	content, err := fs.ReadFile(staticFS, filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	contentType := getContentType(filePath)
	c.Data(http.StatusOK, contentType, content)
}

// getContentType returns the MIME type for a file based on its extension
func getContentType(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
