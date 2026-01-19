package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kenmoini/msaki/internal/auth"
	"github.com/kenmoini/msaki/internal/config"
	"github.com/kenmoini/msaki/internal/logging"
	"github.com/kenmoini/msaki/internal/metrics"
	"github.com/kenmoini/msaki/internal/models"
	"github.com/kenmoini/msaki/internal/server"
	"github.com/kenmoini/msaki/internal/tracing"
	"github.com/kenmoini/msaki/web"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	cleanup := tracing.InitTracer()
	defer cleanup(context.Background())

	// Parse command line flags
	configPath := flag.String("config", "configs/msaki.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	noStatic := flag.Bool("no-static", false, "Disable embedded static file serving")
	healthCheck := flag.Bool("health-check", false, "Perform health check and exit")
	healthCheckURL := flag.String("health-check-url", "http://localhost:8080/health", "URL for health check")
	flag.Parse()

	if *showVersion {
		log.Printf("MSAKI %s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
		os.Exit(0)
	}

	// Health check mode for container health checks
	if *healthCheck {
		os.Exit(performHealthCheck(*healthCheckURL))
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded from %s", *configPath)
	log.Printf("Loaded %d model(s)", len(cfg.Models))

	// Build server options
	var opts []server.Option

	// Initialize logging
	logManager, err := logging.NewLogManager(&cfg.Global.Observability)
	if err != nil {
		log.Printf("Warning: Could not initialize logging: %v", err)
	} else {
		opts = append(opts, server.WithLogManager(logManager))
		log.Println("Logging initialized")
	}

	// Initialize metrics
	if cfg.Global.Observability.Metrics.Enabled {
		m := metrics.New()
		opts = append(opts, server.WithMetrics(m))
		m.SetModelsTotal(len(cfg.Models))
		log.Printf("Metrics enabled at %s", cfg.Global.Observability.Metrics.Prometheus.Path)
	}

	// Initialize authentication
	if len(cfg.Global.Server.Authentication) > 0 {
		authManager, err := auth.NewAuthManager(cfg.Global.Server.Authentication)
		if err != nil {
			log.Printf("Warning: Could not initialize authentication: %v", err)
		} else {
			opts = append(opts, server.WithAuthManager(authManager))
			log.Printf("Authentication initialized with %d provider(s)", len(cfg.Global.Server.Authentication))
		}
	} else {
		log.Println("Warning: No authentication providers configured - API endpoints will be unprotected")
	}

	// Initialize model manager
	modelManager := models.NewManager(cfg)
	opts = append(opts, server.WithModelManager(modelManager))
	log.Printf("Model manager initialized with %d model(s)", len(cfg.Models))

	// Add embedded static files if not disabled
	if !*noStatic {
		staticFS, err := web.StaticFiles()
		if err != nil {
			log.Printf("Warning: Could not load embedded static files: %v", err)
		} else {
			opts = append(opts, server.WithStaticFS(staticFS))
		}
	}

	// Create and start server
	srv := server.New(cfg, opts...)

	// Channel to listen for errors from the server
	errChan := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Channel to listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
	}

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// performHealthCheck makes an HTTP request to the health endpoint and returns exit code
func performHealthCheck(url string) int {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check failed: status %d\n", resp.StatusCode)
		return 1
	}

	fmt.Println("Health check passed")
	return 0
}
