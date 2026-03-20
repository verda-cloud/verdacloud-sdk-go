# Verda Cloud Go SDK — Codebase Guide

> AI-readable skill file. For detailed architecture, see [architecture.md](architecture.md).

## Overview

Go SDK for the Verda cloud platform (previously DataCrunch). Single-package SDK at `pkg/verda/` with OAuth2 auth, middleware pipeline, and type-safe generic request helpers.

- **Module**: `github.com/verda-cloud/verdacloud-sdk-go`
- **Go version**: 1.25+ (uses generics)
- **Single dependency**: `github.com/go-ozzo/ozzo-validation/v4`

## Directory Structure

```
pkg/verda/              # All SDK code lives here
├── client.go           # Client struct, NewClient(), ClientOptions, Do()
├── request.go          # Generic getRequest[T], postRequest[T], etc.
├── auth.go             # OAuth2 client credentials + token refresh
├── middleware.go        # Request/Response middleware implementations
├── middleware_manager.go # Thread-safe middleware chain management
├── errors.go           # APIError, ValidationError
├── logger.go           # Logger interface + NoOp/Std/Slog implementations
├── version.go          # SDK version, user-agent builder
├── types.go            # Shared types (FlexibleFloat, etc.)
├── validation.go       # Validation helpers
├── test_helpers.go     # NewTestClient for unit tests
├── testutil/
│   └── mock_server.go  # httptest-based mock server
│
│  # Service files — one per API resource:
├── instances.go / instances_types.go
├── volumes.go / volumes_types.go
├── clusters.go / clusters_types.go
├── container_deployments.go / container_deployments_types.go
├── serverless_jobs.go / serverless_jobs_types.go
├── ssh_keys.go
├── images.go
├── locations.go
├── balance.go
├── startup_scripts.go
├── instance_types.go
├── instance_availability.go
├── volume_types.go
├── container_types.go
└── long_term.go

test/integration/       # Integration tests (build tag: integration)
├── helpers.go          # getTestClient, FindAvailable*, WaitFor*
└── *_test.go           # Per-resource integration tests

example/main.go         # Usage examples
```

## Core Architecture

### 1. Client (`client.go`)

`Client` is the central struct. Created via functional options pattern:

```go
client, err := verda.NewClient(
    verda.WithClientID("..."),
    verda.WithClientSecret("..."),
)
```

Key fields: `BaseURL`, `HTTPClient`, `Logger`, `Middleware`, and embedded service structs (`Instances`, `Volumes`, `Clusters`, etc.).

### 2. Type-Safe Request Helpers (`request.go`)

All HTTP calls go through generic functions:

```go
getRequest[T](ctx, client, url) (T, *Response, error)
postRequest[T](ctx, client, url, body) (T, *Response, error)
putRequest[T](ctx, client, url, body) (T, *Response, error)
patchRequest[T](ctx, client, url, body) (T, *Response, error)
deleteRequest[T](ctx, client, url) (T, *Response, error)
```

`*AllowEmptyResponse` variants exist for endpoints returning empty bodies.

### 3. Middleware Pipeline (`middleware.go`, `middleware_manager.go`)

- **RequestMiddleware**: `func(next RequestHandler) RequestHandler`
- **ResponseMiddleware**: `func(next ResponseHandler) ResponseHandler`
- Built-in: auth injection, JSON content-type, user-agent, exponential backoff retry, error handling, debug logging
- `Snapshot()` ensures thread-safe middleware chain reads

### 4. Authentication (`auth.go`)

- OAuth2 client credentials flow via `/oauth2/token`
- Automatic token refresh (30s before expiry)
- Optional `WithAuthBearerToken()` to bypass OAuth

### 5. Services Pattern

Each API resource follows the same pattern:

```go
type InstancesService struct {
    client *Client
}

func (s *InstancesService) Get(ctx context.Context, id string) ([]Instance, error) {
    result, _, err := getRequest[[]Instance](ctx, s.client, "/instances")
    return result, err
}
```

Services are initialized in `NewClient()` and accessed as `client.Instances`, `client.Volumes`, etc.

## Adding a New API Endpoint

1. **Create types** in `<resource>_types.go`:
   - Request structs with `json` tags and `Validate()` method (ozzo-validation)
   - Response structs with `json` tags

2. **Create service** in `<resource>.go`:
   - Struct with `client *Client` field
   - Methods using `getRequest[T]`, `postRequest[T]`, etc.

3. **Wire into Client** in `client.go`:
   - Add field to `Client` struct
   - Initialize in `NewClient()`

4. **Write unit tests** using `testutil.NewMockServer()` + `NewTestClient()`

5. **Write integration tests** in `test/integration/` with `//go:build integration`

## Testing Patterns

### Unit Tests

```go
func TestSomething(t *testing.T) {
    mockServer := testutil.NewMockServer()
    defer mockServer.Close()
    mockServer.AddResponse("/path", http.StatusOK, `{"key": "value"}`)

    client := verda.NewTestClient(mockServer)
    // test client methods...
}
```

### Integration Tests

- Build tag: `//go:build integration`
- Use `getTestClient(t)` which reads `VERDA_CLIENT_ID` / `VERDA_CLIENT_SECRET` from env
- Helper functions: `FindAvailableInstanceType()`, `WaitForInstanceStatus()`, etc.
- `make test-integration` to run, `make test-smoke` for critical CRUD flows

## Key Make Targets

| Target | Purpose |
|--------|---------|
| `make test` | Unit tests with race detection + coverage |
| `make test-integration` | Integration tests (needs env vars) |
| `make test-smoke` | Smoke tests (instance + container + job CRUD) |
| `make lint` | golangci-lint |
| `make fmt` | gofmt + goimports |
| `make check` | Format + lint + test |
| `make ci` | Full CI pipeline |
| `make coverage` | HTML coverage report |

## Conventions

- **Error types**: `*APIError` for HTTP errors, `*ValidationError` for input validation
- **Context**: All service methods take `context.Context` as first parameter
- **Naming**: Service files match the API resource name; `_types.go` suffix for type definitions
- **Validation**: Request structs implement `Validate() error` using ozzo-validation
- **Logging**: `Logger` interface with `Debug/Info/Warn/Error` — `NoOpLogger` by default
