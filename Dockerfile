# Dockerfile for Verda Cloud Go SDK Development
# Provides consistent environment with fixed Go and tooling versions

FROM golang:1.24-alpine AS base

# Install build essentials and tools
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev \
    bash

# Install golangci-lint at a specific version for consistency
# Using v1.62.2 - latest stable version as of Nov 2024
# Pin to specific version to ensure reproducibility
RUN wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin v1.62.2

# Install goimports for import formatting (v0.28.0 is compatible with Go 1.23)
RUN go install golang.org/x/tools/cmd/goimports@v0.28.0

# Install govulncheck for vulnerability scanning
RUN go install golang.org/x/vuln/cmd/govulncheck@latest

# Set working directory (code will be mounted here at runtime)
WORKDIR /workspace

# Default command shows help
CMD ["make", "help"]
