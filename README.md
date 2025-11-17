# Verda Cloud Go SDK

[![CI](https://github.com/verda-cloud/verdacloud-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/verda-cloud/verdacloud-sdk-go/actions/workflows/ci.yml)
[![Security](https://github.com/verda-cloud/verdacloud-sdk-go/actions/workflows/security.yml/badge.svg)](https://github.com/verda-cloud/verdacloud-sdk-go/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/verda-cloud/verdacloud-sdk-go)](https://goreportcard.com/report/github.com/verda-cloud/verdacloud-sdk-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/verda-cloud/verdacloud-sdk-go.svg)](https://pkg.go.dev/github.com/verda-cloud/verdacloud-sdk-go)

> **Note**: Previously known as DataCrunch Go SDK. We're transitioning to Verda.com. Same functionality, new name.

Go SDK for the Verda cloud platform. Manage GPU instances, volumes, SSH keys, and more.

## Quick Start

**Requirements**: Go 1.21+ (needs generics support)

```bash
go get github.com/verda-cloud/verdacloud-sdk-go
```

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
    client, err := verda.NewClient(
        verda.WithClientID(os.Getenv("VERDA_CLIENT_ID")),
        verda.WithClientSecret(os.Getenv("VERDA_CLIENT_SECRET")),
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    instances, err := client.Instances.Get(ctx, "")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d instances\n", len(instances))
}
```

Set your credentials:
```bash
export VERDA_CLIENT_ID="your_client_id"
export VERDA_CLIENT_SECRET="your_client_secret"
```

## Architecture

### Type-Safe Request Functions

All API calls go through generic request functions that enforce type safety at compile time:

```go
func getRequest[T any](ctx context.Context, client *Client, url string) (T, *Response, error)
func postRequest[T any](ctx context.Context, client *Client, url string, reqBody any) (T, *Response, error)
func deleteRequest[T any](ctx context.Context, client *Client, url string) (T, *Response, error)
```

Service methods like `client.Instances.Get()` and `client.SSHKeys.Create()` use these internally. This means:
- Compile-time type checking
- Consistent middleware application (auth, retries, logging)
- Thread-safe request isolation
- Easy to extend for new endpoints

### Middleware System

Add custom behavior to all requests:

```go
client, _ := verda.NewClient(
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
)

// Log all requests
client.Middleware.AddRequestMiddleware(func(next verda.RequestHandler) verda.RequestHandler {
    return func(ctx *verda.RequestContext) error {
        log.Printf("Request: %s %s", ctx.Method, ctx.Path)
        return next(ctx)
    }
})
```

Default middleware includes: authentication, JSON content-type, exponential backoff retries, and error handling.

## Usage

### Instances

```go
ctx := context.Background()

// List instances
instances, err := client.Instances.Get(ctx, "")

// Get specific instance
instance, err := client.Instances.GetByID(ctx, "instance_id")

// Create instance
newInstance, err := client.Instances.Create(ctx, verda.CreateInstanceRequest{
    InstanceType: "1V100.6V",
    Image:        "ubuntu-24.04-cuda-12.8-open-docker",
    Hostname:     "my-gpu-box",
    SSHKeyIDs:    []string{"ssh_key_id"},
    LocationCode: verda.LocationFIN01,
})

// Control instances
err = client.Instances.Shutdown(ctx, "instance_id")
err = client.Instances.Hibernate(ctx, "instance_id")
err = client.Instances.Delete(ctx, "instance_id", nil)

// Check availability
available, err := client.Instances.IsAvailable(ctx, "1V100.6V", false, "")
```

### SSH Keys

```go
// List keys
keys, err := client.SSHKeys.Get(ctx)

// Create key
newKey, err := client.SSHKeys.Create(ctx, verda.CreateSSHKeyRequest{
    Name:      "my-key",
    PublicKey: "ssh-rsa AAAAB3NzaC1yc2E...",
})

// Delete key
err = client.SSHKeys.Delete(ctx, "key_id")
```

### Volumes

```go
// List volumes
volumes, err := client.Volumes.Get(ctx)

// Get specific volume
volume, err := client.Volumes.GetByID(ctx, "volume_id")
```

### Other Services

```go
// Account balance
balance, err := client.Balance.Get(ctx)

// Locations
locations, err := client.Locations.Get(ctx)

// Startup scripts
scripts, err := client.StartupScripts.Get(ctx)
script, err := client.StartupScripts.Create(ctx, verda.CreateStartupScriptRequest{
    Name:   "setup",
    Script: "#!/bin/bash\necho 'Hello'",
})
```

### Error Handling

```go
instances, err := client.Instances.Get(ctx, "")
if err != nil {
    if apiErr, ok := err.(*verda.APIError); ok {
        fmt.Printf("API error %d: %s\n", apiErr.StatusCode, apiErr.Message)
    } else {
        fmt.Printf("Request failed: %v\n", err)
    }
}
```

## Configuration

### Client Options

```go
client, err := verda.NewClient(
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
    verda.WithBaseURL("https://api.verda.com/v1"),    // optional
    verda.WithAuthBearerToken("token"),               // optional, skip OAuth
    verda.WithDebugLogging(true),                     // optional, enable logging
    verda.WithLogger(customLogger),                   // optional, custom logger
)
```

### Debug Logging

**Optional and disabled by default.** There are two ways to enable detailed debug logging:

#### Option 1: Programmatic (Recommended)

```go
client, _ := verda.NewClient(
    verda.WithDebugLogging(true),
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
)

// Add detailed debug logging for all requests/responses
verda.AddDetailedDebugLogging(client)
```

#### Option 2: Environment Variable

```bash
export VERDA_DEBUG=true
go run main.go
```

Both methods log:
- HTTP method, path, query params
- Request/response headers (Authorization redacted)
- Request/response JSON body (truncated at 1000 chars)
- Status codes and errors
- Token refresh calls are automatically skipped

**Note**: `WithDebugLogging(true)` enables basic request timing logs. Use `AddDetailedDebugLogging()` for full request/response payloads including JSON bodies.

Useful for debugging API issues. In production, leave it off.

### Environment Variables

```bash
# Production (default)
export VERDA_CLIENT_ID="your_client_id"
export VERDA_CLIENT_SECRET="your_client_secret"

# Staging
export VERDA_BASE_URL="https://api-staging.verda.com/v1"

# Debug mode
export VERDA_DEBUG=true
```

### Custom Logger

Implement the `Logger` interface to use your own logging library:

```go
type Logger interface {
    Debug(msg string, args ...interface{})
    Info(msg string, args ...interface{})
    Warn(msg string, args ...interface{})
    Error(msg string, args ...interface{})
}
```

Built-in options:
- `NoOpLogger` (default) - no logging, zero overhead
- `StdLogger` - uses Go's standard library
- `SlogLogger` - structured logging support

```go
slogLogger := verda.NewSlogLogger(true)
client, _ := verda.NewClient(
    verda.WithClientID("your_client_id"),
    verda.WithClientSecret("your_client_secret"),
    verda.WithLogger(slogLogger),
)
```

## Testing

### Unit Tests

No credentials needed, uses mocks:

```bash
make test-unit

# With coverage report
make coverage
open build/coverage.html
```

### Integration Tests

**Warning**: Runs against real API, may incur costs.

```bash
export VERDA_CLIENT_ID="your_client_id"
export VERDA_CLIENT_SECRET="your_client_secret"

make test-integration
```

### Development Commands

```bash
# Setup
make setup            # Install tools and configure hooks
make install-tools    # Install golangci-lint and pre-commit
make setup-hooks      # Configure Git hooks

# Testing
make test-unit        # Run unit tests
make test-integration # Run integration tests
make test-all         # Run both

# Code Quality
make check            # Run format, lint, and tests
make lint             # Static analysis with golangci-lint
make fmt              # Format code

# Maintenance
make clean            # Clean build artifacts
make mod-tidy         # Clean up dependencies
```

## Contributing

### Setup

```bash
git clone https://github.com/your-username/verdacloud-sdk-go.git
cd verdacloud-sdk-go
make setup
```

This installs `golangci-lint` and sets up pre-commit hooks. If you don't have `pre-commit`, install it:
- macOS: `brew install pre-commit`
- Linux/Windows: `pip install pre-commit`

### Pre-commit Hooks

Pre-commit hooks run automatically before each commit to ensure code quality. **By default, hooks run locally** (no Docker required).

```bash
# Normal commit - runs checks locally
git commit -m "your message"

# Use Docker for checks (if needed)
PRE_COMMIT_USE_DOCKER=1 git commit -m "your message"

# Skip hooks temporarily (not recommended)
git commit --no-verify -m "your message"
```

The hooks check:
- Code formatting (gofmt, goimports)
- Linting (golangci-lint)
- Security checks (gosec, govulncheck)
- Unit tests

### Making Changes

```bash
git checkout -b feature/your-feature
# make changes
git commit -m "feat: add feature"
git push origin feature/your-feature
```

Pre-commit hooks automatically:
- Format code with `gofmt` and `goimports`
- Run `golangci-lint`
- Run unit tests
- Tidy `go.mod`

### Guidelines

- Add tests for new features (maintain >80% coverage)
- Update docs as needed
- Use conventional commits (`feat:`, `fix:`, `docs:`)
- All PRs must pass CI checks

See [.github/CONTRIBUTING.md](.github/CONTRIBUTING.md) for details.

## Examples

Check the [example/](example/) directory for complete examples:

```bash
make example
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Links

- **Website**: [verda.com](https://verda.com)
- **API Docs**: [docs.verda.com](https://api.datacrunch.io/v1/docs)
- **Support**: [support@verda.com](mailto:support@verda.com)

---

Previously known as DataCrunch. Same team, same service, new name.
