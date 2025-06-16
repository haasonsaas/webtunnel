# WebTunnel Makefile

.PHONY: build build-all run run-local run-demo test clean docker docker-build docker-run deps lint format help

# Build variables
BINARY_NAME=webtunnel
BUILD_DIR=bin
GO_FILES=$(shell find . -name "*.go" -type f)
VERSION=$(shell git describe --tags --always --dirty)
COMMIT=$(shell git rev-parse HEAD)
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -w -s"

## Build all WebTunnel variants
build: build-main build-local build-demo
	@echo "✅ All WebTunnel binaries built successfully!"

## Build main application (requires database)
build-main:
	@echo "🔨 Building $(BINARY_NAME) (full stack)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/webtunnel

## Build local version (no database required)
build-local:
	@echo "🔨 Building $(BINARY_NAME)-local (no dependencies)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-local ./cmd/webtunnel-local

## Build demo version (mock API)
build-demo:
	@echo "🔨 Building $(BINARY_NAME)-demo (demo mode)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-demo ./cmd/webtunnel-demo

## Run full stack (requires PostgreSQL + Redis)
run: build-main
	@echo "🚀 Starting WebTunnel full stack..."
	@./$(BUILD_DIR)/$(BINARY_NAME) serve

## Run local version (RECOMMENDED - real terminals, no dependencies)
run-local: build-local
	@echo "🚀 Starting WebTunnel local (real terminal functionality)..."
	@echo "📱 Open http://127.0.0.1:8081 in your browser"
	@./$(BUILD_DIR)/$(BINARY_NAME)-local

## Run demo version (mock API for testing)
run-demo: build-demo
	@echo "🚀 Starting WebTunnel demo (mock API)..."
	@echo "📱 Open http://localhost:8080 in your browser"
	@./$(BUILD_DIR)/$(BINARY_NAME)-demo

## Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

## Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

## Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

## Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

## Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run

## Format code
format:
	@echo "Formatting code..."
	@gofmt -w .
	@go mod tidy

## Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t webtunnel:$(VERSION) .
	@docker tag webtunnel:$(VERSION) webtunnel:latest

## Run Docker container
docker-run: docker-build
	@echo "Running Docker container..."
	@docker run -p 8443:8443 webtunnel:latest

## Start with Docker Compose
docker:
	@echo "Starting with Docker Compose..."
	@docker-compose up -d

## Stop Docker Compose
docker-down:
	@echo "Stopping Docker Compose..."
	@docker-compose down

## View Docker logs
docker-logs:
	@echo "Viewing Docker logs..."
	@docker-compose logs -f webtunnel

## Test WebTunnel functionality
test-local: build-local
	@echo "🧪 Testing WebTunnel local functionality..."
	@./$(BUILD_DIR)/$(BINARY_NAME)-local &
	@SERVER_PID=$$!; \
	sleep 2; \
	echo "Testing health endpoint:"; \
	curl -s http://127.0.0.1:8081/health || echo "❌ Health check failed"; \
	echo "Testing authentication:"; \
	curl -s -X POST http://127.0.0.1:8081/api/v1/auth/login \
		-H "Content-Type: application/json" \
		-d '{"email":"test@example.com","password":"test"}' || echo "❌ Auth failed"; \
	kill $$SERVER_PID 2>/dev/null || true; \
	echo "✅ Local test completed"

## Quick test to verify build
test-build:
	@echo "🧪 Testing all builds..."
	@./$(BUILD_DIR)/$(BINARY_NAME) version || echo "❌ Main binary failed"
	@./$(BUILD_DIR)/$(BINARY_NAME)-local --help >/dev/null || echo "❌ Local binary failed"  
	@./$(BUILD_DIR)/$(BINARY_NAME)-demo --help >/dev/null || echo "❌ Demo binary failed"
	@echo "✅ All binaries working"

## Setup development environment
dev-setup:
	@echo "🔧 Setting up development environment..."
	@mkdir -p /tmp/webtunnel-local
	@go mod download
	@go mod tidy
	@echo "✅ Development environment ready"

## Build for multiple platforms
build-platforms:
	@echo "🔨 Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/webtunnel-local
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/webtunnel-local
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/webtunnel-local
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/webtunnel-local
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/webtunnel-local
	@echo "✅ Cross-platform builds completed"

## Generate database migrations
migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations $$name

## Run database migrations
migrate-up:
	@migrate -database "$(WEBTUNNEL_DATABASE_URL)" -path migrations up

## Rollback database migrations
migrate-down:
	@migrate -database "$(WEBTUNNEL_DATABASE_URL)" -path migrations down 1

## Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest

## Show help
help:
	@echo "🌐 WebTunnel - Enhanced Remote Terminal Access"
	@echo "Inspired by VibeTunnel with modern enhancements"
	@echo ""
	@echo "🚀 Quick Start Commands:"
	@echo "  make run-local    - Start WebTunnel with real terminals (RECOMMENDED)"
	@echo "  make run-demo     - Start demo mode with mock API"
	@echo "  make docker       - Start full stack with Docker Compose"
	@echo ""
	@echo "📋 Available Commands:"
	@grep -E '^## .*' $(MAKEFILE_LIST) | sed 's/## /  /'
	@echo ""
	@echo "💡 Most useful for testing: make run-local"

# Default target
.DEFAULT_GOAL := help