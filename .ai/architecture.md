# Architecture Reference

## Request Flow

```
Service Method (e.g. client.Instances.Get)
  → getRequest[T](ctx, client, url)
    → client.NewRequest(method, url, body)
    → client.Do(req)
      → Request Middleware Chain:
        1. UserAgentMiddleware    — sets User-Agent header
        2. JSONMiddleware         — sets Content-Type/Accept
        3. AuthMiddleware         — injects Bearer token (auto-refresh)
        4. DebugLoggingMiddleware — logs request details (if enabled)
      → http.Client.Do(req)
      → Response Middleware Chain:
        1. ErrorHandlingMiddleware — converts non-2xx to *APIError
        2. RetryMiddleware         — exponential backoff on 429/5xx
        3. DebugLoggingMiddleware  — logs response details (if enabled)
      → JSON unmarshal into T
    → return (T, *Response, error)
```

## Client Struct Layout

```go
type Client struct {
    BaseURL       string
    HTTPClient    *http.Client
    Logger        Logger
    Middleware    *MiddlewareManager

    // OAuth2
    clientID      string
    clientSecret  string
    token         *oauth2Token
    tokenMu       sync.Mutex

    // Services (initialized in NewClient)
    Balance              *BalanceService
    Clusters             *ClustersService
    ContainerDeployments *ContainerDeploymentsService
    ContainerTypes       *ContainerTypesService
    Images               *ImagesService
    InstanceAvailability *InstanceAvailabilityService
    Instances            *InstancesService
    InstanceTypes        *InstanceTypesService
    Locations            *LocationsService
    LongTerm             *LongTermService
    ServerlessJobs       *ServerlessJobsService
    SSHKeys              *SSHKeysService
    StartupScripts       *StartupScriptsService
    Volumes              *VolumesService
    VolumeTypes          *VolumeTypesService
}
```

## Middleware Manager

Thread-safe middleware management via `MiddlewareManager`:

```go
type MiddlewareManager struct {
    mu                  sync.RWMutex
    requestMiddleware   []RequestMiddleware
    responseMiddleware  []ResponseMiddleware
}
```

- `AddRequestMiddleware()` / `AddResponseMiddleware()` — append
- `SetRequestMiddleware()` / `SetResponseMiddleware()` — replace all
- `Snapshot()` — returns copies for thread-safe iteration during request execution

## Service File Pattern

Each service follows this structure:

```
<resource>.go         — Service struct + methods (Get, GetByID, Create, Update, Delete, actions)
<resource>_types.go   — Request/response structs with json tags + Validate() methods
```

### Request Struct Convention

```go
type CreateInstanceRequest struct {
    InstanceType string   `json:"instance_type"`
    Image        string   `json:"image"`
    Hostname     string   `json:"hostname"`
    SSHKeyIDs    []string `json:"ssh_key_ids"`
    LocationCode string   `json:"location"`
}

func (r CreateInstanceRequest) Validate() error {
    return validation.ValidateStruct(&r,
        validation.Field(&r.InstanceType, validation.Required),
        validation.Field(&r.Image, validation.Required),
    )
}
```

### Response Struct Convention

```go
type Instance struct {
    ID           string   `json:"id"`
    InstanceType string   `json:"instance_type"`
    Image        string   `json:"image"`
    Hostname     string   `json:"hostname"`
    Status       string   `json:"status"`
    IP           string   `json:"ip"`
    Location     string   `json:"location"`
    // ...
}
```

## Authentication Flow

```
NewClient()
  → stores clientID + clientSecret
  → first API call triggers AuthMiddleware
    → token == nil or expired?
      → POST /oauth2/token (client_credentials grant)
      → cache token + expiry
      → refresh 30s before expiry
    → inject Authorization: Bearer <token>
```

## Error Hierarchy

```
error
├── *APIError          — HTTP non-2xx responses
│   ├── StatusCode int
│   ├── Message    string
│   └── Code       string
└── *ValidationError   — Input validation failures
    └── Errors map[string]string
```

## Test Infrastructure

### Mock Server (`testutil/mock_server.go`)

```go
server := testutil.NewMockServer()
defer server.Close()
server.AddResponse("/instances", http.StatusOK, `[{"id": "1"}]`)

client := verda.NewTestClient(server)
```

`NewMockServer()` wraps `httptest.Server` and provides:
- `AddResponse(path, status, body)` — register mock responses
- `AddResponseWithHeaders(path, status, body, headers)` — with custom headers
- Auto-registers `/oauth2/token` returning a test token

### Integration Test Helpers (`test/integration/helpers.go`)

- `getTestClient(t)` — creates real client from env vars, skips if missing
- `FindAvailableInstanceType(ctx, client)` — finds a type with available stock
- `FindAvailableClusterType(ctx, client)` — finds available cluster config
- `WaitForInstanceStatus(ctx, client, id, status, timeout)` — polls until status matches
- `WaitForClusterStatus(ctx, client, id, status, timeout)` — polls until cluster ready

## Location Constants

Defined in service files as string constants:

```go
const (
    LocationFIN01 = "FIN-01"
    LocationICE01 = "ICE-01"
    // ...
)
```
