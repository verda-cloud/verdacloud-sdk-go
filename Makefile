# ============================================================================
# Verda Cloud Go SDK - Makefile
# ============================================================================
# This Makefile provides common development tasks for the Verda Cloud Go SDK.
# For static analysis and formatting, use pre-commit hooks (see setup-hooks).
# ============================================================================

.PHONY: help build test test-unit test-integration clean coverage
.PHONY: lint fmt check setup setup-hooks install-tools pre-commit
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
	@echo 'Tip: Run "make setup" first to install all development tools!'

# ============================================================================
# Development Setup
# ============================================================================

setup: install-tools setup-hooks ## Complete development environment setup (tools + hooks)
	@echo "✓ Development environment is ready!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Run 'make check' to verify everything works"
	@echo "  2. Start coding! Pre-commit hooks will run automatically"
	@echo "  3. Use 'make test-unit' to run unit tests"

install-tools: ## Install required development tools (golangci-lint) and check for pre-commit
	@echo "→ Checking development tools..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "  Installing golangci-lint v2.5.0..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0; \
		echo "  ✓ golangci-lint v2.5.0 installed successfully"; \
	else \
		echo "  ✓ golangci-lint already installed ($$(golangci-lint --version))"; \
	fi
	@echo ""
	@if ! command -v pre-commit >/dev/null 2>&1; then \
		echo "  ⚠ pre-commit not found (optional for local development)"; \
		echo ""; \
		echo "  To enable pre-commit hooks, install pre-commit:"; \
		echo ""; \
		echo "  macOS:"; \
		echo "    brew install pre-commit"; \
		echo ""; \
		echo "  Linux (Ubuntu/Debian):"; \
		echo "    pip install pre-commit"; \
		echo "    # or: pip3 install pre-commit"; \
		echo ""; \
		echo "  Linux (Fedora):"; \
		echo "    pip install pre-commit"; \
		echo ""; \
		echo "  Windows:"; \
		echo "    pip install pre-commit"; \
		echo ""; \
		echo "  After installing, run 'make setup-hooks' to configure."; \
		echo ""; \
	else \
		echo "  ✓ pre-commit already installed ($$(pre-commit --version))"; \
	fi

setup-hooks: ## Install and configure pre-commit hooks for automatic checks
	@echo "→ Setting up pre-commit hooks..."
	@if ! command -v pre-commit >/dev/null 2>&1; then \
		echo "  ✗ pre-commit not found"; \
		echo ""; \
		echo "  Please install pre-commit first. See 'make install-tools' for instructions."; \
		echo ""; \
		exit 1; \
	fi
	@pre-commit install
	@pre-commit install --hook-type commit-msg
	@echo "  ✓ Pre-commit hooks installed successfully"
	@echo "  ℹ  Hooks will run automatically on 'git commit'"

# ============================================================================
# Code Quality - Linting and Formatting
# ============================================================================
# Why: These commands help maintain code quality and consistency.
# Note: Pre-commit hooks run these automatically, but you can run manually too.
# ============================================================================

lint: ## Run golangci-lint on all Go code (static analysis, security checks)
	@echo "→ Running golangci-lint..."
	@golangci-lint run ./...
	@echo "✓ Linting complete!"

security: ## Run security checks (gosec via golangci-lint + govulncheck)
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

gosec: ## Run gosec security scanner (respects .golangci.yml exclusions)
	@echo "→ Running gosec via golangci-lint..."
	@golangci-lint run --no-config -E gosec ./...
	@echo "✓ gosec complete!"

govulncheck: ## Run Go vulnerability checker
	@echo "→ Running govulncheck..."
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "  Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@govulncheck ./...
	@echo "✓ govulncheck complete!"

fmt: ## Format all Go code using gofmt and goimports
	@echo "→ Formatting Go code..."
	@gofmt -w -s .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "  ℹ goimports not found, skipping import formatting"; \
	fi
	@echo "✓ Formatting complete!"

check: fmt lint test-unit ## Run all quality checks (format, lint, test) - CI/CD ready
	@echo "✓ All checks passed!"

check-security: check security ## Run all checks including security scans
	@echo "✓ All checks including security passed!"

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
	@go test -tags=integration -v -timeout=10m -c -o build/integration.test ./test/integration >/dev/null 2>&1 || true
	@go test -tags=integration -v -timeout=10m ./test/integration
	@echo "✓ Integration tests passed!"

test: lint test-unit ## Run linting and unit tests (default test target)

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

clean: ## Clean build artifacts, coverage reports, and test caches
	@echo "→ Cleaning build artifacts..."
	@rm -rf build/
	@rm -f coverage.out coverage.html integration.test  # Legacy locations
	@go clean -testcache 2>/dev/null || true
	@echo "✓ Clean complete!"

clean-all: ## Clean everything including Go build cache (may require permissions)
	@echo "→ Deep cleaning (including Go cache)..."
	@rm -rf build/
	@rm -f coverage.out coverage.html integration.test
	@go clean -cache -testcache
	@echo "✓ Deep clean complete!"

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

ci: ## Run all CI checks (matches GitHub Actions checks)
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

ci-local: fmt lint test-unit ## Quick local CI check (without strict format/tidy verification)
	@echo "✓ Local CI checks completed!"

# ============================================================================
# Docker Development Environment
# ============================================================================
# Run commands in a persistent Docker container for faster execution.
# Container stays running in background and reuses same environment.
# ============================================================================

DOCKER_IMAGE := verda-dev
DOCKER_CONTAINER := verda-dev-container

docker-build: ## Build Docker development image with fixed Go and tool versions
	@echo "→ Building Docker development image..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "✓ Docker image '$(DOCKER_IMAGE)' built successfully"
	@echo ""
	@echo "Container will auto-start when you run docker commands"

# Internal target: ensures container is running (auto-starts if needed)
docker-ensure-running:
	@CONTAINER_RUNNING=$$(docker ps -q -f name=$(DOCKER_CONTAINER) 2>/dev/null); \
	CONTAINER_EXISTS=$$(docker ps -aq -f name=$(DOCKER_CONTAINER) 2>/dev/null); \
	if [ -z "$$CONTAINER_RUNNING" ]; then \
		if ! docker image inspect $(DOCKER_IMAGE) >/dev/null 2>&1; then \
			echo "→ Building Docker image..."; \
			docker build -q -t $(DOCKER_IMAGE) .; \
		fi; \
		if [ -n "$$CONTAINER_EXISTS" ]; then \
			echo "→ Restarting existing dev container..."; \
			docker start $(DOCKER_CONTAINER) >/dev/null 2>&1; \
			sleep 1; \
			echo "✓ Dev container restarted"; \
		else \
			echo "→ Starting dev container..."; \
			docker run -d \
				--name $(DOCKER_CONTAINER) \
				-v $(PWD):/workspace \
				-w /workspace \
				-e CGO_ENABLED=0 \
				$(DOCKER_IMAGE) \
				tail -f /dev/null >/dev/null 2>&1; \
			sleep 1; \
			echo "✓ Dev container started"; \
		fi; \
	fi
	@# Verify correct mount: Makefile must be at /workspace/Makefile; otherwise recreate container
	@docker exec $(DOCKER_CONTAINER) sh -lc 'test -f /workspace/Makefile' >/dev/null 2>&1 || ( \
		echo "→ Detected incorrect mount in container; recreating with correct volume..."; \
		docker stop $(DOCKER_CONTAINER) >/dev/null 2>&1 || true; \
		docker rm $(DOCKER_CONTAINER) >/dev/null 2>&1 || true; \
		docker run -d \
			--name $(DOCKER_CONTAINER) \
			-v $(PWD):/workspace \
			-w /workspace \
			-e CGO_ENABLED=0 \
			$(DOCKER_IMAGE) \
			tail -f /dev/null >/dev/null 2>&1; \
		sleep 1; \
		echo "✓ Dev container recreated" )

docker-start: docker-ensure-running ## Start persistent dev container in background (stays running)
	@echo "✓ Dev container is ready"
	@echo ""
	@echo "Now you can run commands instantly:"
	@echo "  make docker-lint"
	@echo "  make docker-test"
	@echo "  make docker-ci"

docker-stop: ## Stop and remove the persistent dev container
	@CONTAINER_EXISTS=$$(docker ps -aq -f name=$(DOCKER_CONTAINER) 2>/dev/null); \
	if [ -n "$$CONTAINER_EXISTS" ]; then \
		echo "→ Stopping dev container..."; \
		docker stop $(DOCKER_CONTAINER) >/dev/null 2>&1 || true; \
		docker rm $(DOCKER_CONTAINER) >/dev/null 2>&1 || true; \
		echo "✓ Dev container stopped"; \
	else \
		echo "✓ Dev container not running"; \
	fi

docker-restart: docker-stop docker-start ## Restart the dev container (useful after image rebuild)

docker-lint: docker-ensure-running ## Run linting in persistent container (auto-starts if needed)
	@echo "→ Running linting in container..."
	@docker exec -w /workspace $(DOCKER_CONTAINER) make lint

docker-test: docker-ensure-running ## Run unit tests in persistent container (auto-starts if needed)
	@echo "→ Running unit tests in container..."
	@docker exec -e CGO_ENABLED=1 -w /workspace $(DOCKER_CONTAINER) make test-unit

docker-test-integration: docker-ensure-running ## Run integration tests (auto-starts, set VERDA_* env vars first)
	@echo "→ Running integration tests in container..."
	@docker exec -w /workspace \
		-e VERDA_CLIENT_ID=$(VERDA_CLIENT_ID) \
		-e VERDA_CLIENT_SECRET=$(VERDA_CLIENT_SECRET) \
		-e VERDA_BASE_URL=$(VERDA_BASE_URL) \
		$(DOCKER_CONTAINER) make test-integration

docker-coverage: docker-ensure-running ## Run tests with coverage in persistent container (auto-starts if needed)
	@echo "→ Running coverage in container..."
	@docker exec -e CGO_ENABLED=1 -w /workspace $(DOCKER_CONTAINER) make coverage

docker-ci: docker-ensure-running ## Run all CI checks in persistent container (auto-starts if needed)
	@echo "→ Running CI checks in container..."
	@docker exec -e CGO_ENABLED=1 -w /workspace $(DOCKER_CONTAINER) make ci

docker-fmt: docker-ensure-running ## Format code in persistent container (auto-starts if needed)
	@echo "→ Formatting code in container..."
	@docker exec -w /workspace $(DOCKER_CONTAINER) make fmt

docker-shell: docker-ensure-running ## Open interactive shell in persistent container (auto-starts if needed)
	@echo "→ Opening shell in container..."
	@docker exec -it -w /workspace $(DOCKER_CONTAINER) /bin/bash

docker-security: docker-ensure-running ## Run security checks in persistent container (auto-starts if needed)
	@echo "→ Running security checks in container..."
	@docker exec -w /workspace $(DOCKER_CONTAINER) make security

docker-status: ## Show status of dev container
	@CONTAINER_RUNNING=$$(docker ps -q -f name=$(DOCKER_CONTAINER) 2>/dev/null); \
	if [ -n "$$CONTAINER_RUNNING" ]; then \
		echo "✓ Dev container is RUNNING"; \
		echo ""; \
		docker ps -f name=$(DOCKER_CONTAINER) --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"; \
	else \
		echo "✗ Dev container is NOT running"; \
		echo ""; \
		echo "Start it with: make docker-start"; \
	fi

docker-clean: docker-stop ## Remove Docker image and container
	@echo "→ Removing Docker image..."
	@docker rmi $(DOCKER_IMAGE) 2>/dev/null || true
	@echo "✓ Docker cleanup complete"

docker-help: ## Show Docker-specific help
	@echo '╔════════════════════════════════════════════════════════════════╗'
	@echo '║          Docker Development Environment                        ║'
	@echo '╚════════════════════════════════════════════════════════════════╝'
	@echo ''
	@echo 'Why Docker?'
	@echo '  • Guaranteed consistency: Same Go 1.24 and golangci-lint v2.5.0'
	@echo '  • No local setup needed: Everything runs in container'
	@echo '  • Matches CI/CD exactly: Same environment as GitHub Actions'
	@echo '  • FAST: Container stays running, commands execute instantly!'
	@echo ''
	@echo 'Quick Start:'
	@echo '  make docker-start        # Start container (once, stays running)'
	@echo '  make docker-lint         # Run linting (instant!)'
	@echo '  make docker-test         # Run tests (instant!)'
	@echo '  make docker-ci           # Run all CI checks (instant!)'
	@echo ''
	@echo 'Management:'
	@echo '  make docker-status       # Check if container is running'
	@echo '  make docker-stop         # Stop the container'
	@echo '  make docker-restart      # Restart container'
	@echo '  make docker-shell        # Open bash in container'
	@echo ''

.PHONY: setup install-tools setup-hooks lint fmt check check-security security gosec govulncheck
.PHONY: build test test-unit test-integration coverage clean clean-all mod-tidy update-deps
.PHONY: pre-commit pre-commit-run pre-commit-update ci ci-local
.PHONY: docker-build docker-start docker-stop docker-restart docker-lint docker-test
.PHONY: docker-test-integration docker-coverage docker-ci docker-fmt docker-shell
.PHONY: docker-status docker-clean docker-help docker-security
