package server

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/kenmoini/msaki/internal/api/handlers"
	"github.com/kenmoini/msaki/internal/auth"
	"github.com/kenmoini/msaki/internal/config"
	"github.com/kenmoini/msaki/internal/logging"
	"github.com/kenmoini/msaki/internal/metrics"
	"github.com/kenmoini/msaki/internal/models"
	"github.com/kenmoini/msaki/internal/proxy"
)

// Server represents the MSAKI HTTP server
type Server struct {
	config       *config.Config
	router       *gin.Engine
	httpServer   *http.Server
	staticFS     fs.FS
	logManager   *logging.LogManager
	metrics      *metrics.Metrics
	authManager  *auth.AuthManager
	modelManager *models.Manager
}

// Option is a functional option for Server configuration
type Option func(*Server)

// WithStaticFS sets the embedded filesystem for static file serving
func WithStaticFS(staticFS fs.FS) Option {
	return func(s *Server) {
		s.staticFS = staticFS
	}
}

// WithLogManager sets the log manager for the server
func WithLogManager(lm *logging.LogManager) Option {
	return func(s *Server) {
		s.logManager = lm
	}
}

// WithMetrics sets the metrics collector for the server
func WithMetrics(m *metrics.Metrics) Option {
	return func(s *Server) {
		s.metrics = m
	}
}

// WithAuthManager sets the authentication manager for the server
func WithAuthManager(am *auth.AuthManager) Option {
	return func(s *Server) {
		s.authManager = am
	}
}

// WithModelManager sets the model manager for the server
func WithModelManager(mm *models.Manager) Option {
	return func(s *Server) {
		s.modelManager = mm
	}
}

// New creates a new Server instance
func New(cfg *config.Config, opts ...Option) *Server {
	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Configure CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	s := &Server{
		config: cfg,
		router: router,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Add logging middleware if log manager is provided
	if s.logManager != nil {
		router.Use(logging.AccessLogMiddleware(s.logManager))
		router.Use(logging.ErrorLogMiddleware(s.logManager))
	}

	// Add metrics middleware if metrics is provided
	if s.metrics != nil {
		router.Use(s.metrics.Middleware())
	}

	s.setupRoutes()

	// Setup static file serving if filesystem is provided
	if s.staticFS != nil {
		s.setupStaticRoutes(s.staticFS)
		log.Println("Static file serving enabled")
	}

	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.healthHandler)

	// Metrics endpoint (if enabled)
	if s.config.Global.Observability.Metrics.Enabled {
		metricsPath := s.config.Global.Observability.Metrics.Prometheus.Path
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		s.router.GET(metricsPath, metrics.Handler())
	}

	// Internal API routes (moved to /msaki/api to distinguish from proxy routes)
	api := s.router.Group("/msaki/api")
	{
		// Auth routes (no authentication required for login)
		authGroup := api.Group("/auth")
		{
			if s.authManager != nil {
				authHandler := handlers.NewAuthHandler(s.authManager)
				authGroup.POST("/login", authHandler.Login)
				authGroup.POST("/logout", authHandler.Logout)
				// /me requires authentication
				authGroup.GET("/me", auth.Middleware(s.authManager), authHandler.Me)
			} else {
				authGroup.POST("/login", s.noAuthHandler())
				authGroup.POST("/logout", s.noAuthHandler())
				authGroup.GET("/me", s.noAuthHandler())
			}
		}

		// Protected routes - require authentication if auth manager is set
		var protectedMiddleware []gin.HandlerFunc
		if s.authManager != nil && s.authManager.HasProvider() {
			protectedMiddleware = append(protectedMiddleware, auth.Middleware(s.authManager))
		}

		// Model routes - list endpoint conditionally public based on allowPublicModelList
		if s.modelManager != nil {
			modelsHandler := handlers.NewModelsHandler(s.modelManager)

			// Model list endpoint - conditionally public
			if s.config.Global.Server.Access.AllowPublicModelList {
				api.GET("/models", modelsHandler.List)
			} else {
				api.GET("/models", append(protectedMiddleware, modelsHandler.List)...)
			}

			// Protected model routes
			modelsGroup := api.Group("/models", protectedMiddleware...)
			{
				modelsGroup.GET("/:name", modelsHandler.Get)
				modelsGroup.GET("/:name/health", modelsHandler.Health)

				// Logs endpoint (protected but not admin-only)
				modelsGroup.GET("/:name/logs", modelsHandler.Logs)
				modelsGroup.GET("/:name/logs/stream", modelsHandler.LogsStream)

				// Start/stop/restart require admin role
				adminMiddleware := []gin.HandlerFunc{}
				if s.authManager != nil {
					adminMiddleware = append(adminMiddleware, auth.RequireRole("administrator"))
				}
				modelsGroup.POST("/:name/start", append(adminMiddleware, modelsHandler.Start)...)
				modelsGroup.POST("/:name/stop", append(adminMiddleware, modelsHandler.Stop)...)
				modelsGroup.POST("/:name/restart", append(adminMiddleware, modelsHandler.Restart)...)
			}
		} else {
			modelsGroup := api.Group("/models", protectedMiddleware...)
			{
				modelsGroup.GET("", s.placeholderHandler("list models"))
				modelsGroup.GET("/:name", s.placeholderHandler("get model"))
				modelsGroup.POST("/:name/start", s.placeholderHandler("start model"))
				modelsGroup.POST("/:name/stop", s.placeholderHandler("stop model"))
				modelsGroup.GET("/:name/health", s.placeholderHandler("model health"))
			}
		}

		// Chat routes
		chat := api.Group("/chat", protectedMiddleware...)
		{
			chat.POST("", s.placeholderHandler("chat"))
			chat.GET("/history", s.placeholderHandler("chat history"))
		}
	}

	// OpenAI-compatible proxy routes (protected)
	var proxyMiddleware []gin.HandlerFunc
	// var proxyMiddlewareNoAuth []gin.HandlerFunc
	if s.authManager != nil && s.authManager.HasProvider() {
		proxyMiddleware = append(proxyMiddleware, auth.Middleware(s.authManager))
	}

	v1 := s.router.Group("/v1")
	{
		if s.modelManager != nil {
			p := proxy.New(s.modelManager, s.metrics)
			if s.config.Global.Server.Access.AllowPublicModelList {
				// Public access to model list - no auth required
				v1.GET("/models", p.ModelsListHandler())
			} else {
				// Protected access - requires authentication
				v1.GET("/models", p.ModelsListHandler())
				// v1.GET("/models", append(proxyMiddleware, p.ModelsListHandler())...)
			}

			v1.POST("/chat/completions", append(proxyMiddleware, p.ChatCompletionsHandler())...)
			v1.POST("/completions", append(proxyMiddleware, p.CompletionsHandler())...)
		} else {
			v1.GET("/models", s.placeholderHandler("list openai models"))
			v1.POST("/chat/completions", append(proxyMiddleware, s.placeholderHandler("chat completions"))...)
			v1.POST("/completions", append(proxyMiddleware, s.placeholderHandler("completions"))...)
		}
	}

	// Ollama-compatible proxy routes (protected)
	ollama := s.router.Group("/api/ollama", proxyMiddleware...)
	{
		if s.modelManager != nil {
			p := proxy.New(s.modelManager, s.metrics)
			ollama.POST("/generate", p.OllamaGenerateHandler())
			ollama.POST("/chat", p.OllamaChatHandler())
			ollama.GET("/tags", p.OllamaTagsHandler())
		} else {
			ollama.POST("/generate", s.placeholderHandler("ollama generate"))
			ollama.POST("/chat", s.placeholderHandler("ollama chat"))
			ollama.GET("/tags", s.placeholderHandler("ollama tags"))
		}
	}
}

// healthHandler returns the health status of the server
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "msaki",
	})
}

// placeholderHandler returns a placeholder response for unimplemented routes
func (s *Server) placeholderHandler(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error":   "not implemented",
			"handler": name,
		})
	}
}

// noAuthHandler returns a response indicating auth is not configured
func (s *Server) noAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "authentication not configured",
		})
	}
}

// Start begins listening for HTTP requests
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d",
		s.config.Global.Server.Listen.Address,
		s.config.Global.Server.Listen.Port,
	)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting MSAKI server on %s", addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down MSAKI server...")

	// Shutdown model manager
	if s.modelManager != nil {
		s.modelManager.Shutdown()
	}

	// Close log manager
	if s.logManager != nil {
		s.logManager.Close()
	}

	return s.httpServer.Shutdown(ctx)
}

// Router returns the Gin router for testing or additional configuration
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Config returns the server configuration
func (s *Server) Config() *config.Config {
	return s.config
}

// LogManager returns the log manager
func (s *Server) LogManager() *logging.LogManager {
	return s.logManager
}

// Metrics returns the metrics collector
func (s *Server) Metrics() *metrics.Metrics {
	return s.metrics
}

// AuthManager returns the authentication manager
func (s *Server) AuthManager() *auth.AuthManager {
	return s.authManager
}

// ModelManager returns the model manager
func (s *Server) ModelManager() *models.Manager {
	return s.modelManager
}
