# Artemis - Multi-Goal Repository Makefile

# Variables
BINARY_NAME = artemis-backtest
BACKTEST_DIR = backtesting
BUILD_DIR = $(BACKTEST_DIR)
BINARY_PATH = $(BUILD_DIR)/$(BINARY_NAME)

# Discord Bot Variables
DISCORD_BOT_NAME = artemis-discord-bot
DISCORD_BOT_DIR = discord-bot
DISCORD_BOT_BINARY = bootstrap
DISCORD_BOT_ZIP = $(DISCORD_BOT_NAME).zip

# Trading Bot Variables
TRADING_BOT_NAME = artemis-trading-bot
TRADING_BOT_DIR = trading-bot
TRADING_BOT_BINARY = bootstrap
TRADING_BOT_ZIP = $(TRADING_BOT_NAME).zip

# Go build flags
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
LDFLAGS = -ldflags="-s -w"

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Artemis - Multi-Goal Repository"
	@echo ""
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Backtesting targets
.PHONY: build
build: ## Build the backtesting tool
	@echo "Building backtesting tool..."
	cd $(BACKTEST_DIR) && go build -o $(BINARY_NAME) ./cmd
	@echo "Build complete: $(BINARY_PATH)"

.PHONY: run
run: build ## Build and run the backtesting tool
	@echo "Running backtesting tool..."
	cd $(BACKTEST_DIR) && ./$(BINARY_NAME)

.PHONY: run-only
run-only: ## Run the backtesting tool (assumes it's already built)
	@echo "Running backtesting tool..."
	cd $(BACKTEST_DIR) && ./$(BINARY_NAME)

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_PATH)
	@echo "Clean complete"

.PHONY: test
test: ## Run tests for the backtesting project
	@echo "Running tests..."
	cd $(BACKTEST_DIR) && go test ./...

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output
	@echo "Running tests with verbose output..."
	cd $(BACKTEST_DIR) && go test -v ./...

.PHONY: deps
deps: ## Download and tidy dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated"

.PHONY: install
install: ## Install the backtesting tool to GOPATH
	@echo "Installing backtesting tool..."
	cd $(BACKTEST_DIR) && go install ./cmd
	@echo "Installation complete"

# Development targets
.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting Go code..."
	cd $(BACKTEST_DIR) && go fmt ./...

.PHONY: vet
vet: ## Run go vet on the code
	@echo "Running go vet..."
	cd $(BACKTEST_DIR) && go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (if installed)
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		cd $(BACKTEST_DIR) && golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Cross-compilation targets
.PHONY: build-linux
build-linux: ## Build for Linux
	@echo "Building for Linux..."
	cd $(BACKTEST_DIR) && GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 ./cmd

.PHONY: build-darwin
build-darwin: ## Build for macOS
	@echo "Building for macOS..."
	cd $(BACKTEST_DIR) && GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 ./cmd

.PHONY: build-windows
build-windows: ## Build for Windows
	@echo "Building for Windows..."
	cd $(BACKTEST_DIR) && GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe ./cmd

.PHONY: build-all
build-all: build-linux build-darwin build-windows ## Build for all platforms

# Data management
.PHONY: validate-data
validate-data: ## Validate CSV data format
	@echo "Validating CSV data..."
	@if [ -f "$(BACKTEST_DIR)/data/example/stock_signals.csv" ]; then \
		echo "✓ CSV file found"; \
		head -5 "$(BACKTEST_DIR)/data/example/stock_signals.csv"; \
	else \
		echo "✗ CSV file not found at $(BACKTEST_DIR)/data/example/stock_signals.csv"; \
		exit 1; \
	fi

# Quick development workflow
.PHONY: dev
dev: deps fmt vet build ## Development workflow: deps, fmt, vet, build

.PHONY: full-test
full-test: deps fmt vet test build run-only ## Full test workflow

# Environment setup
.PHONY: check-env
check-env: ## Check if required environment variables are set
	@echo "Checking environment variables..."
	@if [ -z "$$ALPACA_API_KEY" ]; then \
		echo "⚠️  ALPACA_API_KEY not set"; \
	else \
		echo "✓ ALPACA_API_KEY is set"; \
	fi
	@if [ -z "$$ALPACA_SECRET_KEY" ]; then \
		echo "⚠️  ALPACA_SECRET_KEY not set"; \
	else \
		echo "✓ ALPACA_SECRET_KEY is set"; \
	fi

# Documentation
.PHONY: docs
docs: ## Generate documentation (if godoc is available)
	@echo "Generating documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "Starting godoc server at http://localhost:6060"; \
		cd $(BACKTEST_DIR) && godoc -http=:6060; \
	else \
		echo "godoc not found. Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Discord Bot targets
.PHONY: build-discord
build-discord: ## Build the Discord bot for AWS Lambda
	@echo "Building Discord bot for AWS Lambda..."
	cd $(DISCORD_BOT_DIR) && GOOS=linux GOARCH=amd64 go build -o $(DISCORD_BOT_BINARY) ./cmd
	@echo "Discord bot build complete: $(DISCORD_BOT_DIR)/$(DISCORD_BOT_BINARY)"

.PHONY: package-discord
package-discord: build-discord ## Build and package Discord bot for Lambda deployment
	@echo "Packaging Discord bot for Lambda deployment..."
	cd $(DISCORD_BOT_DIR) && zip $(DISCORD_BOT_ZIP) $(DISCORD_BOT_BINARY)
	@echo "Discord bot package complete: $(DISCORD_BOT_DIR)/$(DISCORD_BOT_ZIP)"

.PHONY: clean-discord
clean-discord: ## Clean Discord bot build artifacts
	@echo "Cleaning Discord bot build artifacts..."
	rm -f $(DISCORD_BOT_DIR)/$(DISCORD_BOT_BINARY)
	rm -f $(DISCORD_BOT_DIR)/$(DISCORD_BOT_ZIP)
	@echo "Discord bot clean complete"

.PHONY: test-discord
test-discord: ## Run tests for the Discord bot
	@echo "Running Discord bot tests..."
	cd $(DISCORD_BOT_DIR) && go test ./...

.PHONY: deps-discord
deps-discord: ## Download and tidy Discord bot dependencies
	@echo "Downloading Discord bot dependencies..."
	go mod download
	go mod tidy
	@echo "Discord bot dependencies updated"

.PHONY: dev-discord
dev-discord: deps-discord build-discord ## Development workflow for Discord bot

.PHONY: deploy-discord
deploy-discord: package-discord ## Prepare Discord bot for deployment (builds and packages)
	@echo "Discord bot ready for deployment!"
	@echo "Upload $(DISCORD_BOT_DIR)/$(DISCORD_BOT_ZIP) to AWS Lambda"
	@echo "Set environment variables:"
	@echo "  - DYNAMODB_REGION"
	@echo "  - TABLE_NAME"
	@echo "  - DISCORD_PUBLIC_KEY"

.PHONY: check-discord-env
check-discord-env: ## Check if Discord bot environment variables are set
	@echo "Checking Discord bot environment variables..."
	@if [ -z "$$DISCORD_PUBLIC_KEY" ]; then \
		echo "⚠️  DISCORD_PUBLIC_KEY not set"; \
	else \
		echo "✓ DISCORD_PUBLIC_KEY is set"; \
	fi
	@if [ -z "$$DYNAMODB_REGION" ]; then \
		echo "⚠️  DYNAMODB_REGION not set (will use default: us-east-1)"; \
	else \
		echo "✓ DYNAMODB_REGION is set: $$DYNAMODB_REGION"; \
	fi
	@if [ -z "$$TABLE_NAME" ]; then \
		echo "⚠️  TABLE_NAME not set (will use default: artemis-data)"; \
	else \
		echo "✓ TABLE_NAME is set: $$TABLE_NAME"; \
	fi

# Trading Bot Lambda targets
.PHONY: build-trading
build-trading: ## Build the trading bot for AWS Lambda
	@echo "Building trading bot for AWS Lambda..."
	cd $(TRADING_BOT_DIR) && GOOS=linux GOARCH=amd64 go build -o $(TRADING_BOT_BINARY) ./cmd
	@echo "Trading bot build complete: $(TRADING_BOT_DIR)/$(TRADING_BOT_BINARY)"

.PHONY: package-trading
package-trading: build-trading ## Build and package trading bot for Lambda deployment
	@echo "Packaging trading bot for Lambda deployment..."
	cd $(TRADING_BOT_DIR) && zip $(TRADING_BOT_ZIP) $(TRADING_BOT_BINARY)
	@echo "Trading bot package complete: $(TRADING_BOT_DIR)/$(TRADING_BOT_ZIP)"

.PHONY: clean-trading
clean-trading: ## Clean trading bot build artifacts
	@echo "Cleaning trading bot build artifacts..."
	rm -f $(TRADING_BOT_DIR)/$(TRADING_BOT_BINARY)
	rm -f $(TRADING_BOT_DIR)/$(TRADING_BOT_ZIP)
	@echo "Trading bot clean complete"

.PHONY: deploy-trading
deploy-trading: package-trading ## Prepare trading bot for deployment (builds and packages)
	@echo "Trading bot ready for deployment!"
	@echo "Upload $(TRADING_BOT_DIR)/$(TRADING_BOT_ZIP) to AWS Lambda"
	@echo "Set environment variables:"
	@echo "  - ALPACA_API_KEY"
	@echo "  - ALPACA_SECRET_KEY"
	@echo "  - DYNAMODB_REGION"
	@echo "  - TABLE_NAME"
	@echo "  - DISCORD_WEBHOOK_URL (optional)"

# Combined targets
.PHONY: build-all-bots
build-all-bots: build-discord build-trading ## Build both Discord and trading bots

.PHONY: package-all-bots
package-all-bots: package-discord package-trading ## Package both bots for Lambda deployment

.PHONY: clean-all-bots
clean-all-bots: clean-discord clean-trading ## Clean all bot build artifacts

.PHONY: deploy-all-bots
deploy-all-bots: deploy-discord deploy-trading ## Prepare both bots for deployment
