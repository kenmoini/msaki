.PHONY: all build backend frontend dev clean test lint help embed-frontend

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Directories
BIN_DIR := bin
FRONTEND_DIR := frontend
FRONTEND_OUT := $(FRONTEND_DIR)/out
WEB_STATIC := web/static

# Default target
all: build

## help: Show this help message
help:
	@echo "MSAKI - Model Swiss Army Knife Interface"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@awk '/^## / {sub(/^## /, ""); print}' $(MAKEFILE_LIST) | column -t -s ':'

## build: Build both frontend and backend (production)
build: frontend embed-frontend backend

## backend: Build the Go backend binary
backend:
	@echo "Building MSAKI backend..."
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/msaki ./cmd/msaki

## backend-dev: Build backend without embedded frontend (for development)
backend-dev:
	@echo "Building MSAKI backend (dev mode)..."
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(WEB_STATIC)
	@echo "Creating placeholder for embed..."
	@touch $(WEB_STATIC)/.gitkeep
	go build $(LDFLAGS) -o $(BIN_DIR)/msaki ./cmd/msaki

## frontend: Build the NextJS frontend
frontend:
	@echo "Building MSAKI frontend..."
	@if [ -d "$(FRONTEND_DIR)" ]; then \
		cd $(FRONTEND_DIR) && npm ci && npm run build; \
	else \
		echo "Frontend directory not found. Skipping frontend build."; \
	fi

## embed-frontend: Copy frontend build to web/static for embedding
embed-frontend:
	@echo "Copying frontend to web/static for embedding..."
	@rm -rf $(WEB_STATIC)
	@mkdir -p $(WEB_STATIC)
	@if [ -d "$(FRONTEND_OUT)" ]; then \
		cp -r $(FRONTEND_OUT)/* $(WEB_STATIC)/; \
		echo "Frontend copied successfully."; \
	else \
		echo "Warning: Frontend build not found at $(FRONTEND_OUT)"; \
		touch $(WEB_STATIC)/.gitkeep; \
	fi

## dev-backend: Run the backend in development mode
dev-backend:
	@echo "Starting MSAKI backend in development mode..."
	go run ./cmd/msaki -config configs/msaki.yaml -no-static

## dev-frontend: Run the frontend in development mode
dev-frontend:
	@echo "Starting MSAKI frontend in development mode..."
	@cd $(FRONTEND_DIR) && npm run dev

## test: Run tests
test:
	@echo "Running tests..."
	go test -v ./...

## lint: Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, skipping..."; \
	fi

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -rf $(FRONTEND_OUT)
	rm -rf $(FRONTEND_DIR)/.next
	rm -rf $(WEB_STATIC)

## tidy: Run go mod tidy
tidy:
	go mod tidy

## deps: Install dependencies
deps:
	@echo "Installing Go dependencies..."
	go mod download
	@if [ -d "$(FRONTEND_DIR)" ]; then \
		echo "Installing frontend dependencies..."; \
		cd $(FRONTEND_DIR) && npm ci; \
	fi

## run: Run the application with example config
run: backend
	./$(BIN_DIR)/msaki -config configs/msaki.example.yaml
