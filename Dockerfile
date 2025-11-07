# Dockerfile for Verda Cloud Go SDK Development
# Provides consistent environment with fixed Go and tooling versions

FROM golang:1.21-alpine AS base

# Install build essentials and tools
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev \
    bash

# Install golangci-lint at a specific version for consistency
# Using v2.6.1 - latest stable version as of Nov 2024
# Pin to specific version to ensure reproducibility
RUN wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin v2.6.1

# Install goimports for import formatting (v0.21.0 is compatible with Go 1.21)
RUN go install golang.org/x/tools/cmd/goimports@v0.21.0

# Set working directory (code will be mounted here at runtime)
WORKDIR /workspace

# Default command shows help
CMD ["make", "help"]

