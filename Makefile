# ============================================================================
# Verda Cloud Go SDK - Makefile
# ============================================================================
# This Makefile provides common development tasks for the Verda Cloud Go SDK.
# For static analysis and formatting, use pre-commit hooks (see setup-hooks).
# ============================================================================

.PHONY: help build test-unit test-integration clean coverage
.PHONY: lint fmt setup pre-commit
.DEFAULT_GOAL := help

# ============================================================================
# Help Target - Shows all available commands
# ============================================================================

help: ## Show this help message with all available commands
	@echo '╔════════════════════════════════════════════════════════════════╗'
	@echo '║          Verda Cloud Go SDK - Development Commands            ║'
	@echo '╚════════════════════════════════════════════════════════════════╝'
	@echo ''
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Tip: Run "make setup" to install tools and configure hooks!'

# ============================================================================
# Development Setup
# ============================================================================

setup: ## Complete development environment setup (installs only what's needed)
	@echo "→ Setting up development environment..."
	@echo ""
	@# Check and install golangci-lint if needed
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint v2.5.0..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0; \
		echo "✓ golangci-lint installed"; \
	else \
		echo "✓ golangci-lint already installed ($$(golangci-lint --version))"; \
	fi
	@echo ""
	@# Check and install pre-commit hooks if pre-commit is available
	@if command -v pre-commit >/dev/null 2>&1; then \
		if [ ! -f .git/hooks/pre-commit ]; then \
			echo "Installing pre-commit hooks..."; \
			pre-commit install >/dev/null 2>&1; \
			pre-commit install --hook-type commit-msg >/dev/null 2>&1; \
			echo "✓ Pre-commit hooks installed"; \
		else \
			echo "✓ Pre-commit hooks already installed"; \
		fi; \
	else \
		echo "⚠ pre-commit not found (optional)"; \
		echo "  Install: brew install pre-commit (macOS) or pip install pre-commit"; \
	fi
	@echo ""
	@echo "✓ Development environment ready!"

# ============================================================================
# Code Quality - CI Targets
# ============================================================================
# Note: Pre-commit hooks auto-format and lint on commit.
# These targets are used by CI workflows, not for manual use.
# ============================================================================

lint: ## Run golangci-lint (used by CI, pre-commit handles this locally)
	@echo "→ Running golangci-lint..."
	@golangci-lint run ./...
	@echo "✓ Linting complete!"

security: ## Run security checks (gosec + govulncheck)
	@echo "→ Running security checks..."
	@echo "  1. Running gosec (via golangci-lint)..."
	@golangci-lint run --no-config -E gosec ./...
	@echo "  2. Running govulncheck..."
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "    Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@govulncheck ./...
	@echo "✓ Security checks complete!"

fmt: ## Format Go code (used by CI, pre-commit handles this locally)
	@echo "→ Formatting Go code..."
	@gofmt -w -s .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "  ℹ goimports not found, skipping import formatting"; \
	fi
	@echo "✓ Formatting complete!"

# ============================================================================
# Building
# ============================================================================

build: ## Build the SDK to verify compilation (no binary output for libraries)
	@echo "→ Building Verda Cloud Go SDK..."
	@go build ./pkg/verda
	@echo "✓ Build successful!"

# ============================================================================
# Testing
# ============================================================================
# Why: Comprehensive testing ensures reliability and catches regressions.
# - Unit tests: Fast, no external dependencies, run in CI/CD
# - Integration tests: Require API credentials, test real API interaction
# ============================================================================

test-unit: ## Run unit tests (fast, no external dependencies)
	@echo "→ Running unit tests..."
	@mkdir -p build
	@go test -v -race -coverprofile=build/coverage.out ./pkg/verda
	@echo "✓ Unit tests passed!"

test-integration: ## Run integration tests (requires VERDA_CLIENT_ID and VERDA_CLIENT_SECRET env vars)
	@echo "→ Running integration tests..."
	@echo "  Required: VERDA_CLIENT_ID and VERDA_CLIENT_SECRET"
	@echo "  Optional: VERDA_BASE_URL (defaults to production)"
	@if [ -z "$$VERDA_CLIENT_ID" ] || [ -z "$$VERDA_CLIENT_SECRET" ]; then \
		echo "  ✗ Missing required environment variables"; \
		echo "  Set VERDA_CLIENT_ID and VERDA_CLIENT_SECRET before running"; \
		exit 1; \
	fi
	@if [ -n "$$VERDA_BASE_URL" ]; then \
		echo "  Using custom API: $$VERDA_BASE_URL"; \
	else \
		echo "  Using production API: https://api.verda.com/v1"; \
	fi
	@mkdir -p build
	@go test -tags=integration -v -timeout=30m -c -o build/integration.test ./test/integration >/dev/null 2>&1 || true
	@go test -tags=integration -v -timeout=30m ./test/integration
	@echo "✓ Integration tests passed!"

test-e2e: ## Run e2e tests only: instance, serverless container, serverless job (cheapest resources; cleanup on success and failure)
	@echo "→ Running e2e tests (instance + serverless container + serverless job)..."
	@echo "  Required: VERDA_CLIENT_ID and VERDA_CLIENT_SECRET (env only; never commit)"
	@echo "  Optional: VERDA_BASE_URL (defaults to https://api.verda.com/v1)"
	@echo "  Timeout: 30 minutes"
	@# Load .env file if it exists (runs in same shell as subsequent commands)
	@if [ -f .env ]; then \
		echo "  Loading .env file..."; \
		set -a; . ./.env; set +a; \
		if [ -z "$$VERDA_CLIENT_ID" ] || [ -z "$$VERDA_CLIENT_SECRET" ]; then \
			echo "  ✗ Missing required environment variables (check .env file)"; \
			exit 1; \
		fi; \
		if [ -n "$$VERDA_BASE_URL" ]; then \
			echo "  ✅ API URL: $$VERDA_BASE_URL"; \
		else \
			echo "  ✅ API URL: https://api.verda.com/v1 (default)"; \
		fi; \
		mkdir -p build; \
		go test -tags=integration -v -timeout=30m -run 'TestInstanceCRUDIntegration|TestContainerDeploymentsCRUDWithScalingAndEnvVars|TestServerlessJobsCRUDWithScalingAndEnvVars' ./test/integration; \
		echo "✓ E2E tests passed!"; \
	elif [ -z "$$VERDA_CLIENT_ID" ] || [ -z "$$VERDA_CLIENT_SECRET" ]; then \
		echo "  ✗ Missing required environment variables"; \
		echo "  Either create a .env file or set VERDA_CLIENT_ID and VERDA_CLIENT_SECRET"; \
		exit 1; \
	elif [ -n "$$VERDA_BASE_URL" ]; then \
		echo "  ✅ API URL: $$VERDA_BASE_URL"; \
		mkdir -p build; \
		go test -tags=integration -v -timeout=30m -run 'TestInstanceCRUDIntegration|TestContainerDeploymentsCRUDWithScalingAndEnvVars|TestServerlessJobsCRUDWithScalingAndEnvVars' ./test/integration; \
		echo "✓ E2E tests passed!"; \
	else \
		echo "  ✅ API URL: https://api.verda.com/v1 (default)"; \
		mkdir -p build; \
		go test -tags=integration -v -timeout=30m -run 'TestInstanceCRUDIntegration|TestContainerDeploymentsCRUDWithScalingAndEnvVars|TestServerlessJobsCRUDWithScalingAndEnvVars' ./test/integration; \
		echo "✓ E2E tests passed!"; \
	fi

coverage: ## Generate test coverage report (HTML output)
	@echo "→ Generating coverage report..."
	@mkdir -p build
	@go test -v -race -coverprofile=build/coverage.out -covermode=atomic ./pkg/verda
	@go tool cover -html=build/coverage.out -o build/coverage.html
	@go tool cover -func=build/coverage.out | grep total | awk '{print "  Coverage: " $$3}'
	@echo "✓ Coverage report generated: build/coverage.html"

# ============================================================================
# Maintenance
# ============================================================================

clean: ## Clean build artifacts, coverage reports, test caches, and Go build cache
	@echo "→ Cleaning build artifacts and caches..."
	@rm -rf build/
	@rm -f coverage.out coverage.html integration.test  # Legacy locations
	@go clean -cache -testcache 2>/dev/null || true
	@echo "✓ Clean complete!"

mod-tidy: ## Tidy Go module dependencies (go mod tidy)
	@echo "→ Tidying Go modules..."
	@go mod tidy
	@echo "✓ Go modules tidied!"

update-deps: ## Update all Go dependencies to their latest versions
	@echo "→ Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "✓ Dependencies updated!"

# ============================================================================
# Release Management
# ============================================================================

release: ## Prepare a new release by updating CHANGELOG.md (usage: make release VERSION=v1.0.0)
	@scripts/release.sh $(VERSION)

# ============================================================================
# Pre-commit Management
# ============================================================================

pre-commit: ## Run all pre-commit hooks on all files (stages modified files first)
	@echo "→ Staging modified files..."
	@git add -u
	@echo "→ Running pre-commit hooks on all files..."
	@pre-commit run --all-files

# ============================================================================
# CI/CD Targets
# ============================================================================

ci: ## Run all CI checks (matches GitHub Actions: format, mod-tidy, lint, build, test)
	@echo "→ Running CI checks (same as GitHub Actions)..."
	@echo ""
	@echo "1. Format check..."
	@$(MAKE) fmt
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "  ✗ Code is not formatted"; \
		exit 1; \
	else \
		echo "  ✓ Code is formatted"; \
	fi
	@echo ""
	@echo "2. Mod tidy check..."
	@$(MAKE) mod-tidy
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "  ✗ go.mod is not tidy"; \
		exit 1; \
	else \
		echo "  ✓ go.mod is tidy"; \
	fi
	@echo ""
	@echo "3. Linting..."
	@$(MAKE) lint
	@echo ""
	@echo "4. Building..."
	@$(MAKE) build
	@echo ""
	@echo "5. Testing..."
	@$(MAKE) test-unit
	@echo ""
	@echo "✓ All CI checks passed! Ready to push."

.PHONY: setup lint fmt security
.PHONY: build test-unit test-integration coverage clean mod-tidy update-deps release
.PHONY: pre-commit ci
