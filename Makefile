# WebTunnel Makefile

.PHONY: build run test clean docker docker-build docker-run deps lint format help

# Build variables
BINARY_NAME=webtunnel
BUILD_DIR=bin
GO_FILES=$(shell find . -name "*.go" -type f)
VERSION=$(shell git describe --tags --always --dirty)
COMMIT=$(shell git rev-parse HEAD)
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -w -s"

## Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/webtunnel

## Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BUILD_DIR)/$(BINARY_NAME) serve

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

## Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/webtunnel
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/webtunnel
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/webtunnel
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/webtunnel
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/webtunnel

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
	@echo "Available commands:"
	@grep -E '^## .*' $(MAKEFILE_LIST) | sed 's/## /  /'

# Default target
.DEFAULT_GOAL := help