.PHONY: help
.DEFAULT_GOAL := help

# Go parameters
GOCMD=go
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Directories
PKG_DIR=./...
INTERNAL_DIR=./internal/breaker

# Coverage
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Colors
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_BLUE=\033[34m

##@ General

help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(COLOR_BOLD)Usage:$(COLOR_RESET)\n  make $(COLOR_BLUE)<target>$(COLOR_RESET)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(COLOR_BLUE)%-15s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

install-tools: ## Install development tools
	@echo "$(COLOR_GREEN)Installing development tools...$(COLOR_RESET)"
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@echo "$(COLOR_GREEN)✓ Development tools installed$(COLOR_RESET)"

deps: ## Download dependencies
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	@$(GOMOD) download
	@$(GOMOD) tidy
	@echo "$(COLOR_GREEN)✓ Dependencies downloaded$(COLOR_RESET)"

##@ Testing

test: ## Run all tests (including stress tests)
	@echo "$(COLOR_GREEN)Running all tests...$(COLOR_RESET)"
	@$(GOTEST) -v $(PKG_DIR)

test-short: ## Run tests excluding stress tests
	@echo "$(COLOR_GREEN)Running tests (short mode, excluding stress tests)...$(COLOR_RESET)"
	@$(GOTEST) -v -short $(PKG_DIR)

test-race: ## Run tests with race detector
	@echo "$(COLOR_GREEN)Running tests with race detector...$(COLOR_RESET)"
	@$(GOTEST) -race $(PKG_DIR)

test-coverage: ## Run tests with coverage report
	@echo "$(COLOR_GREEN)Running tests with coverage...$(COLOR_RESET)"
	@$(GOTEST) -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(PKG_DIR)
	@echo "$(COLOR_GREEN)Coverage report generated: $(COVERAGE_FILE)$(COLOR_RESET)"

coverage-html: test-coverage ## Generate HTML coverage report
	@echo "$(COLOR_GREEN)Generating HTML coverage report...$(COLOR_RESET)"
	@$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "$(COLOR_GREEN)✓ Coverage report: $(COVERAGE_HTML)$(COLOR_RESET)"

stress: ## Run stress tests (30 min timeout)
	@echo "$(COLOR_YELLOW)Running stress tests (this will take several minutes)...$(COLOR_RESET)"
	@$(GOTEST) -v -run TestStress $(INTERNAL_DIR) -timeout 30m

##@ Benchmarking

bench: ## Run all benchmarks
	@echo "$(COLOR_GREEN)Running benchmarks...$(COLOR_RESET)"
	@$(GOTEST) -bench=. -benchmem -run=^$$ $(INTERNAL_DIR)

##@ Code Quality

fmt: ## Format all Go files
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	@$(GOFMT) -w .
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

fmt-check: ## Check if code is formatted
	@echo "$(COLOR_GREEN)Checking code formatting...$(COLOR_RESET)"
	@unformatted=$$($(GOFMT) -l . 2>&1); \
	if [ -n "$$unformatted" ]; then \
		echo "$(COLOR_YELLOW)Files need formatting:$(COLOR_RESET)"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "$(COLOR_GREEN)✓ All files properly formatted$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	@$(GOVET) $(PKG_DIR)
	@echo "$(COLOR_GREEN)✓ go vet passed$(COLOR_RESET)"

lint: install-tools ## Run golangci-lint
	@echo "$(COLOR_GREEN)Running golangci-lint...$(COLOR_RESET)"
	@golangci-lint run
	@echo "$(COLOR_GREEN)✓ golangci-lint passed$(COLOR_RESET)"

check: fmt-check vet lint ## Run all code quality checks
	@echo "$(COLOR_GREEN)✓ All quality checks passed$(COLOR_RESET)"

##@ CI/CD

ci: ## Run CI checks (format, lint, race tests, coverage)
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Running CI pipeline...$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)1/4: Formatting check$(COLOR_RESET)"
	@$(MAKE) fmt-check
	@echo ""
	@echo "$(COLOR_BOLD)2/4: Linting$(COLOR_RESET)"
	@$(MAKE) vet
	@echo ""
	@echo "$(COLOR_BOLD)3/4: Tests with race detector$(COLOR_RESET)"
	@$(MAKE) test-race
	@echo ""
	@echo "$(COLOR_BOLD)4/4: Coverage check$(COLOR_RESET)"
	@$(MAKE) test-coverage
	@coverage=$$($(GOCMD) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | sed 's/%//'); \
	threshold=97.0; \
	echo "Coverage: $$coverage% (threshold: $$threshold%)"; \
	if [ $$(echo "$$coverage < $$threshold" | bc -l) -eq 1 ]; then \
		echo "$(COLOR_YELLOW)⚠ Coverage $$coverage% is below threshold $$threshold%$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo ""
	@echo "$(COLOR_GREEN)$(COLOR_BOLD)✓ CI pipeline passed!$(COLOR_RESET)"

##@ Cleanup

clean: ## Clean build artifacts and test files
	@echo "$(COLOR_GREEN)Cleaning build artifacts...$(COLOR_RESET)"
	@$(GOCLEAN)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@echo "$(COLOR_GREEN)✓ Cleaned$(COLOR_RESET)"
