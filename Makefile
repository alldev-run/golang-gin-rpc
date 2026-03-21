# Golang Gin RPC Application Makefile

# Application variables
APP_NAME = alldev-gin-rpc
BUILD_DIR = bin
CONFIG_DIR = configs
LOG_DIR = logs
TEMP_DIR = temp

# Go variables
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# Build variables
BUILD_FLAGS = -v
LDFLAGS = -ldflags "-X main.version=$(shell git describe --tags --always 2>/dev/null || echo 'dev')"

.PHONY: help build run test clean deps fmt vet lint docker-build docker-run

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Scaffold:'
	@echo '  make create-api NAME=<new-api> [TEMPLATE=http-gateway] (templates live in pkg/gateway/templates)'
	@echo '  make export-template NAME=<api-name> [TEMPLATE=http-gateway] (sync api/<name> back into pkg/gateway/templates)'

# Build targets
build: ## Build the application
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) .
	@echo "Build completed: $(BUILD_DIR)/$(APP_NAME)"

build-debug: ## Build the application with debug symbols
	@echo "Building $(APP_NAME) in debug mode..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -gcflags="all=-N -l" -o $(BUILD_DIR)/$(APP_NAME)-debug .
	@echo "Debug build completed: $(BUILD_DIR)/$(APP_NAME)-debug"

# Run targets
run: build ## Build and run the application
	@echo "Starting $(APP_NAME)..."
	@mkdir -p $(LOG_DIR) $(TEMP_DIR)
	./$(BUILD_DIR)/$(APP_NAME)

run-debug: build-debug ## Build debug version and run with debugger
	@echo "Starting $(APP_NAME) in debug mode..."
	@mkdir -p $(LOG_DIR) $(TEMP_DIR)
	dlv --listen=:40000 --api-version=2 --headless=true exec ./$(BUILD_DIR)/$(APP_NAME)-debug

# Development targets
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

fmt: ## Format Go code
	@echo "Formatting Go code..."
	$(GOCMD) fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...

lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Quality targets
check: fmt vet test ## Run all quality checks (format, vet, test)

quality: fmt vet lint test-coverage ## Run full quality checks including lint and coverage

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf $(LOG_DIR)
	rm -rf $(TEMP_DIR)
	$(GOCLEAN)
	rm -f coverage.out coverage.html

clean-all: clean ## Clean everything including caches
	@echo "Cleaning all artifacts and caches..."
	$(GOMOD) cache clean
	rm -f go.sum

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):latest .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 --env-file .env $(APP_NAME):latest

docker-compose-up: ## Start services with docker-compose
	@echo "Starting services with docker-compose..."
	docker-compose up -d

docker-compose-down: ## Stop services with docker-compose
	@echo "Stopping services with docker-compose..."
	docker-compose down

# Setup targets
setup: ## Initial project setup
	@echo "Setting up project..."
	@mkdir -p $(CONFIG_DIR) $(LOG_DIR) $(TEMP_DIR)
	@if [ ! -f $(CONFIG_DIR)/config.yaml ]; then \
		echo "Creating default config file..."; \
		cp $(CONFIG_DIR)/config.example.yaml $(CONFIG_DIR)/config.yaml 2>/dev/null || echo "Please create $(CONFIG_DIR)/config.yaml manually"; \
	fi
	$(MAKE) deps

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Installing delve debugger..."
	go install github.com/go-delve/delve/cmd/dlv@latest
	@echo "Installing air for hot reload..."
	go install github.com/air-verse/air@latest

# Development targets
dev: ## Run with hot reload (requires air)
	@echo "Starting development server with hot reload..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not installed. Install with: make install-tools"; \
		echo "Or run: make run"; \
	fi

# Production targets
prod-build: ## Build for production
	@echo "Building $(APP_NAME) for production..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -a -installsuffix cgo -o $(BUILD_DIR)/$(APP_NAME)-linux .
	@echo "Production build completed: $(BUILD_DIR)/$(APP_NAME)-linux"

# Database targets
db-migrate: ## Run database migrations (placeholder)
	@echo "Running database migrations..."
	@echo "TODO: Add migration logic"

db-seed: ## Seed database (placeholder)
	@echo "Seeding database..."
	@echo "TODO: Add seeding logic"

# Utility targets
version: ## Show version information
	@echo "App: $(APP_NAME)"
	@echo "Go: $(shell go version)"
	@echo "Git: $(shell git describe --tags --always 2>/dev/null || echo 'dev')"
	@echo "Build: $(shell date)"

info: ## Show project information
	@echo "Project: $(APP_NAME)"
	@echo "Build Dir: $(BUILD_DIR)"
	@echo "Config Dir: $(CONFIG_DIR)"
	@echo "Log Dir: $(LOG_DIR)"
	@echo "Temp Dir: $(TEMP_DIR)"

create-api: ## Create a new api project from template (default template: pkg/gateway/templates/http-gateway). Usage: make create-api NAME=foo [TEMPLATE=http-gateway]
	@if [ -z "$(NAME)" ]; then \
		echo "missing NAME. example: make create-api NAME=user-gateway TEMPLATE=http-gateway"; \
		exit 2; \
	fi
	@$(GOCMD) run ./cmd/scaffold create-api --name "$(NAME)" --template "$(or $(TEMPLATE),http-gateway)"

export-template: ## Export api/<NAME> into pkg/gateway/templates/<TEMPLATE> (Go files become .gotmpl and tokens are injected). Usage: make export-template NAME=http-gateway [TEMPLATE=http-gateway]
	@if [ -z "$(NAME)" ]; then \
		echo "missing NAME. example: make export-template NAME=zzz-demo TEMPLATE=http-gateway"; \
		exit 2; \
	fi
	@$(GOCMD) run ./cmd/scaffold export-template --name "$(NAME)" --template "$(or $(TEMPLATE),http-gateway)"
