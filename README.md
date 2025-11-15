# Verda Cloud Go SDK

[![CI](https://github.com/verda-cloud/verdacloud-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/verda-cloud/verdacloud-sdk-go/actions/workflows/ci.yml)
[![Security](https://github.com/verda-cloud/verdacloud-sdk-go/actions/workflows/security.yml/badge.svg)](https://github.com/verda-cloud/verdacloud-sdk-go/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/verda-cloud/verdacloud-sdk-go)](https://goreportcard.com/report/github.com/verda-cloud/verdacloud-sdk-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/verda-cloud/verdacloud-sdk-go.svg)](https://pkg.go.dev/github.com/verda-cloud/verdacloud-sdk-go)

> **⚠️ Important Notice**: This SDK was previously known as DataCrunch Go SDK. We are transitioning from DataCrunch.io to Verda.com as part of our company rebranding. All functionality remains the same, only the naming has changed.

Official Go SDK for the Verda cloud platform API. This SDK provides easy access to Verda's cloud infrastructure services including instances, volumes, SSH keys, and more.

## Features

- **Type-Safe Operations**: Built with Go generics for compile-time type safety
- **Comprehensive Service Coverage**: Instances, volumes, SSH keys, startup scripts, containers, and more
- **Modern Architecture**: Standalone generic request functions with middleware support
- **Flexible Authentication**: OAuth2 client credentials with automatic token refresh
- **Extensive Testing**: 40+ unit tests plus integration test suite
- **Production Ready**: Used in production environments with proper error handling

## Software Architecture

### Standalone Generic Request Functions

The SDK uses a modern architecture with standalone generic request functions that provide type-safe HTTP operations:

```go
// All service methods use these internally for consistent behavior
func getRequest[T any](ctx context.Context, client *Client, url string) (T, *Response, error)
func postRequest[T any](ctx context.Context, client *Client, url string, reqBody any) (T, *Response, error)
func putRequest[T any](ctx context.Context, client *Client, url string, reqBody any) (T, *Response, error)
func deleteRequest[T any](ctx context.Context, client *Client, url string) (T, *Response, error)
func deleteRequestNoResult(ctx context.Context, client *Client, url string) (*Response, error)
```

**Benefits:**
- **Type Safety**: Compile-time type checking with Go generics
- **Consistent Behavior**: All requests automatically use client middleware (auth, JSON content-type, error handling)
- **Thread Safety**: Each request gets isolated middleware chains
- **Future Ready**: Complete HTTP method coverage for new API endpoints

**Service Integration:**
All service methods (e.g., `client.Instances.Get()`, `client.SSHKeys.Create()`) use these functions internally, ensuring consistent behavior across the entire SDK.

### Migration from Legacy Patterns

The SDK has been migrated from legacy `NewRequest[T]` patterns to standalone generic request functions. This migration provides:

- **Better Type Safety**: Explicit generic type parameters
- **Cleaner API**: Simplified function signatures
- **Consistent Error Handling**: Unified error response patterns
- **Future Compatibility**: Ready for new API endpoints

**Note**: Legacy `makeRequest()` methods are still used internally for APIs that return plain text instead of JSON (e.g., some create operations that return only an ID).

### Middleware Management

The SDK provides a clean middleware system for request/response processing:

```go
// Create client with default middleware (auth, JSON content-type, error handling)
client, err := verda.NewClient(
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
)

// Add global middleware for all requests
client.Middleware.AddRequestMiddleware(func(next verda.RequestHandler) verda.RequestHandler {
    return func(ctx *verda.RequestContext) error {
        log.Printf("Request: %s %s", ctx.Method, ctx.Path)
        return next(ctx)
    }
})

// Global middleware are applied to all service method calls
ctx := context.Background()
instances, err := client.Instances.Get(ctx, "")
```

Key features:

- **Thread-safe**: All operations protected by mutex
- **Request isolation**: Each request gets its own middleware copy
- **Clean management**: Centralized CRUD operations via `client.Middleware`
- **Backward compatible**: Existing client methods still work

## Quick Start

### Dependency Check

- **Go**: Version 1.21 or higher (required for generics support)
- **Verda API Credentials**: Client ID and Client Secret from your Verda account

### Build

```bash
# Install the SDK
go get github.com/verda-cloud/verdacloud-sdk-go

# Build your project
go build
```

### Run

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func main() {
    // Create client with credentials
    client, err := verda.NewClient(
        verda.WithBaseURL("https://api.verda.com/v1"), // Optional, uses default
        verda.WithClientID(os.Getenv("VERDA_CLIENT_ID")),
        verda.WithClientSecret(os.Getenv("VERDA_CLIENT_SECRET")),
    )
    if err != nil {
        log.Fatal(err)
    }

    // List instances using service method
    ctx := context.Background()
    instances, err := client.Instances.Get(ctx, "")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d instances\n", len(instances))
}
```

## User Guide

### Authentication

The SDK uses OAuth2 client credentials flow. You need to provide your Verda API credentials:

1. Set environment variables:
   ```bash
   export VERDA_BASE_URL="https://api.verda.com/v1" # Optional, uses default
   export VERDA_CLIENT_ID="your_client_id"
   export VERDA_CLIENT_SECRET="your_client_secret"
   ```

2. Or pass them directly to the client:
   ```go
   client, err := verda.NewClient(
       verda.WithBaseURL("https://api.verda.com/v1"), // Optional, uses default
       verda.WithClientID("your_client_id"),
       verda.WithClientSecret("your_client_secret"),
   )
   ```

### Services

#### Instances

Manage cloud instances:

```go
ctx := context.Background()

// List all instances
instances, err := client.Instances.Get(ctx, "")

// Get specific instance
instance, err := client.Instances.GetByID(ctx, "instance_id")

// Create new instance
createRequest := verda.CreateInstanceRequest{
    InstanceType: "1V100.6V",
    Image:        "ubuntu-24.04-cuda-12.8-open-docker",
    Hostname:     "my-instance",
    Description:  "My GPU instance",
    SSHKeyIDs:    []string{"ssh_key_id"},
    Location:     verda.LocationFIN01,
}
newInstance, err := client.Instances.Create(ctx, createRequest)

// Instance actions
err = client.Instances.Shutdown(ctx, "instance_id")
err = client.Instances.Delete(ctx, "instance_id", nil)
err = client.Instances.Hibernate(ctx, "instance_id")

// Check availability
available, err := client.Instances.IsAvailable(ctx, "1V100.6V", false, "")
```

#### SSH Keys

Manage SSH keys:

```go
ctx := context.Background()

// List SSH keys
keys, err := client.SSHKeys.Get(ctx)

// Get specific SSH key
key, err := client.SSHKeys.GetByID(ctx, "key_id")

// Create SSH key
createKeyRequest := verda.CreateSSHKeyRequest{
    Name:      "My Key",
    PublicKey: "ssh-rsa AAAAB3NzaC1yc2E...",
}
newKey, err := client.SSHKeys.Create(ctx, createKeyRequest)

// Delete SSH key
err = client.SSHKeys.Delete(ctx, "key_id")
```

#### Volumes

Manage storage volumes:

```go
ctx := context.Background()

// List volumes
volumes, err := client.Volumes.Get(ctx)

// Get specific volume
volume, err := client.Volumes.GetByID(ctx, "volume_id")
```

#### Startup Scripts

Manage startup scripts:

```go
ctx := context.Background()

// List startup scripts
scripts, err := client.StartupScripts.Get(ctx)

// Create startup script
script, err := client.StartupScripts.Create(ctx, verda.CreateStartupScriptRequest{
    Name:   "Setup Script",
    Script: "#!/bin/bash\necho 'Hello World'",
})

// Delete startup script
err = client.StartupScripts.Delete(ctx, "script_id")
```

#### Balance

Check account balance:

```go
ctx := context.Background()
balance, err := client.Balance.Get(ctx)
fmt.Printf("Balance: %.2f %s\n", balance.Amount, balance.Currency)
```

#### Locations

List available datacenter locations:

```go
ctx := context.Background()
locations, err := client.Locations.Get(ctx)
for _, loc := range locations {
    fmt.Printf("%s (%s): %s\n", loc.Name, loc.Code, loc.Country)
}
```

#### Containers

Manage containers (inference service):

```go
ctx := context.Background()

// List containers
containers, err := client.Containers.Get(ctx)

// Create container
container, err := client.Containers.Create(ctx, verda.CreateContainerRequest{
    Name:  "my-container",
    Image: "nginx:latest",
    Environment: map[string]string{
        "ENV_VAR": "value",
    },
})

// Delete container
err = client.Containers.Delete(ctx, "container_id")
```

### Error Handling

The SDK returns structured errors:

```go
ctx := context.Background()
instances, err := client.Instances.Get(ctx, "")
if err != nil {
    if apiErr, ok := err.(*verda.APIError); ok {
        fmt.Printf("API Error %d: %s\n", apiErr.StatusCode, apiErr.Message)
    } else {
        fmt.Printf("Other error: %v\n", err)
    }
}
```

### Configuration Options

```go
client, err := verda.NewClient(
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
    verda.WithBaseURL("https://api.verda.com/v1"), // Optional, uses default
    verda.WithAuthBearerToken("bearer_token"),         // Optional, for direct token auth
    verda.WithDebugLogging(true),                      // Optional, enable debug logging
    verda.WithLogger(customLogger),                    // Optional, use custom logger
)
```

#### Environment Configuration

The SDK supports different environments through the base URL configuration:

```bash
# Production (default)
export VERDA_CLIENT_ID="your_client_id"
export VERDA_CLIENT_SECRET="your_client_secret"

# Production (alternative)
export VERDA_BASE_URL="https://api.verda.com/v1"

# Staging Environment
export VERDA_BASE_URL="https://api-staging.verda.com/v1"

# Testing Environment
export VERDA_BASE_URL="https://api-testing.verda.com/v1"

```

You can also override the base URL programmatically:

```go
client, err := verda.NewClient(
    verda.WithClientID(os.Getenv("VERDA_CLIENT_ID")),
    verda.WithClientSecret(os.Getenv("VERDA_CLIENT_SECRET")),
    verda.WithBaseURL("https://api.staging.verda.com/v1"), // Use staging
)
```

### Testing

#### Unit Tests

Unit tests mock the Verda API and run without requiring credentials. The test suite includes comprehensive coverage of:

- **Standalone Request Functions**: All generic request functions (`getRequest`, `postRequest`, etc.)
- **Service Methods**: All service operations (instances, SSH keys, volumes, etc.)
- **Middleware System**: Request/response middleware functionality
- **Authentication**: OAuth2 token management and refresh
- **Error Handling**: API error responses and edge cases

```bash
# Run unit tests
make test-unit

# Run with coverage (generates build/coverage.html)
make coverage
open build/coverage.html  # View coverage report
```

**Test Coverage**: 40+ test cases covering all major functionality

**Note**: All build artifacts (test binaries, coverage reports) are organized in the `build/` directory to keep the root clean.

#### Integration Tests

Integration tests run against the real API and require credentials. **WARNING: May create real resources and incur costs.**

```bash
# Set required environment variables
export VERDA_CLIENT_ID="your_client_id"
export VERDA_CLIENT_SECRET="your_client_secret"

# Run integration tests
make test-integration

# Run all tests
make test-all
```

#### Available Make Commands

```bash
# Development Setup
make setup            # Complete development environment setup
make install-tools    # Install golangci-lint and pre-commit
make setup-hooks      # Install pre-commit Git hooks

# Testing
make test-unit        # Run unit tests
make test-integration # Run integration tests (requires credentials)
make coverage         # Generate coverage report

# Code Quality
make check            # Run all quality checks (format, lint, test)
make lint             # Run golangci-lint static analysis
make fmt              # Format all Go code

# Maintenance
make clean            # Clean build artifacts and caches
make mod-tidy         # Tidy Go module dependencies

# For more commands and detailed explanations, see DEVELOPMENT.md
```

### Examples

See the [example/](example/) directory for complete usage examples:

```bash
# Run the example (requires API credentials)
make example
```

### Logging

The SDK provides a flexible logging architecture using the **Strategy Pattern** with dependency injection, allowing you to use any logging library without forcing dependencies:

#### Logger Interface Design

```go
// Core logger interface - implement this for any logging library
type Logger interface {
    Debug(msg string, args ...interface{})
    Info(msg string, args ...interface{})
    Warn(msg string, args ...interface{})
    Error(msg string, args ...interface{})
}
```

#### Built-in Logger Implementations

```go
// 1. No-op Logger (default) - zero overhead when logging is disabled
client, err := verda.NewClient(
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
    // No logger specified = NoOpLogger used
)

// 2. Standard Library Logger - simple console output
client, err := verda.NewClient(
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
    verda.WithDebugLogging(true), // Uses built-in standard logger
)

// 3. Structured Logger (slog) - modern structured logging
slogLogger := verda.NewSlogLogger(true) // true = enable debug
client, err := verda.NewClient(
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
    verda.WithLogger(slogLogger),
)
```

For more advanced logging configurations including custom adapters for Logrus, Zap, and other logging libraries, see the detailed logging documentation in the codebase.

## How to Contribute

We welcome contributions to the Verda Go SDK! Here's how you can help:

### Development Setup

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/your-username/verdacloud-sdk-go.git
   cd verdacloud-sdk-go
   ```

3. **Set up your development environment**:
   ```bash
   make setup
   ```

   This will:
   - Install `golangci-lint` automatically for code quality checks
   - Check for `pre-commit` and guide you to install it if needed
   - Configure pre-commit hooks to run automatically on commit

   **Note:** If pre-commit is not installed, `make setup` will show OS-specific installation instructions:
   - macOS: `brew install pre-commit`
   - Linux: `pip install pre-commit`
   - Windows: `pip install pre-commit`

4. **Configure API credentials** for testing:
   ```bash
   export VERDA_BASE_URL="https://api-testing.verda.com/v1" # Optional, uses default
   export VERDA_CLIENT_ID="your_client_id"
   export VERDA_CLIENT_SECRET="your_client_secret"
   ```

5. **Verify your setup**:
   ```bash
   make check  # Run all quality checks
   ```

📖 **For detailed development workflows, available commands, and tool explanations, see [DEVELOPMENT.md](DEVELOPMENT.md)**

### Making Changes

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following our coding standards:
   - Write clear, documented code
   - Add unit tests for new functionality
   - Follow Go conventions and best practices
   - Update documentation as needed

3. **Commit your changes** (pre-commit hooks run automatically):
   ```bash
   git add .
   git commit -m "feat: add new feature description"
   ```

   Pre-commit hooks will automatically:
   - Format your code with `gofmt` and `goimports`
   - Run `golangci-lint` for static analysis
   - Run unit tests
   - Tidy `go.mod` and `go.sum`

   If any checks fail, fix the issues and commit again.

4. **Optionally run checks manually**:
   ```bash
   make check            # Run all quality checks
   make test-unit        # Run unit tests
   make test-integration # Run integration tests (requires credentials)
   ```

5. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request** on GitHub

### Contribution Guidelines

- **Code Quality**: All code must pass CI checks (linting, tests, formatting)
- **Documentation**: Update README.md and code comments as needed
- **Testing**: Add unit tests for new features and bug fixes (maintain >80% coverage)
- **Commit Messages**: Use conventional commit format (feat:, fix:, docs:, etc.)
- **Breaking Changes**: Clearly document any breaking changes
- **CI/CD**: All PRs must pass automated checks before merge

See [.github/CONTRIBUTING.md](.github/CONTRIBUTING.md) for detailed contribution guidelines.

### Reporting Issues

- Use GitHub Issues to report bugs or request features
- Provide clear reproduction steps for bugs
- Include relevant system information and error messages

## About the Author

This SDK is developed and maintained by the Verda team. Verda (formerly DataCrunch) provides high-performance GPU cloud infrastructure for AI/ML workloads.

- **Website**: [https://verda.com](https://verda.com)
- **Documentation**: [https://docs.verda.com](https://docs.verda.com)
- **Support**: [support@verda.com](mailto:support@verda.com)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

```
MIT License

Copyright (c) 2024 Verda (formerly DataCrunch)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
