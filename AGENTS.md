# AGENTS.md

Instructions for AI agents working with this codebase.

## Conventions (stable)

These rarely change. Follow them for all contributions:

- **One service per file**: `<resource>.go` for methods, `<resource>_types.go` for types/constants/validation
- **Service naming**: Singular form (`InstanceService`, not `InstancesService`)
- **All methods take `context.Context`** as first parameter
- **Request validation**: Implement `Validate() error` using `ozzo-validation`; called by service methods before HTTP requests
- **Type organization**: `types.go` for cross-domain utilities only; domain types in `*_types.go`
- **Error handling**: Return `*APIError` for HTTP errors; `Validate()` returns ozzo-validation errors
- **Tests**: Unit tests use `testutil.NewMockServer()` + `NewTestClient()`, integration tests use `//go:build integration`
- **Commits**: Conventional commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`)

## Codebase Knowledge (auto-generated)

The `.ai/` directory contains auto-generated codebase knowledge. It is **gitignored** and local-only — never commit it. Agents generate and maintain it by scanning the actual code.

### Versioning

`.ai/.meta` tracks when knowledge was last generated:

```
commit: <git short hash>
generated: <ISO 8601 timestamp>
files: <comma-separated list of structural files that were hashed>
```

### Agent Startup Flow

On every session, before doing any work:

1. **Check freshness**: Read `.ai/.meta`. If it exists, run:
   ```
   git diff --name-only <stored-commit>..HEAD -- pkg/verda/ Makefile go.mod
   ```
2. **If fresh** (no structural files changed, or `.meta` commit matches HEAD): Read `.ai/SKILL.md` and `.ai/architecture.md` for context, then proceed
3. **If stale or missing** (structural files changed, `.meta` absent, or `.ai/` doesn't exist): Run the **Codebase Scan** below, then proceed

This means non-structural commits (README edits, CI config, docs) skip the rescan.

### Codebase Scan

When `.ai/` is stale or missing, scan the codebase and generate knowledge files. This should be done as a background/setup step — don't ask the user, just do it.

**What to scan** (read these files/patterns):

| What | Where | Extract |
|------|-------|---------|
| Module & deps | `go.mod` | Module path, Go version, dependencies |
| Client & options | `pkg/verda/client.go` | Client struct fields, services, option functions |
| Request helpers | `pkg/verda/request.go` | Generic function signatures, patterns |
| Auth flow | `pkg/verda/auth.go` | OAuth2 flow, token management |
| Middleware | `pkg/verda/middleware.go`, `middleware_manager.go` | Types, built-in middleware, chain management |
| Error types | `pkg/verda/errors.go` | Error structs, hierarchy |
| Shared types | `pkg/verda/types.go` | Cross-domain utility types |
| Validation helpers | `pkg/verda/validation.go` | Shared validation functions |
| Service files | `pkg/verda/*.go` (non-test) | Service structs, method signatures, patterns |
| Type files | `pkg/verda/*_types.go` | Request/response structs, Validate() patterns, constants |
| Test helpers | `pkg/verda/test_helpers.go`, `testutil/mock_server.go` | Test utilities |
| Integration helpers | `test/integration/helpers.go` | Integration test patterns |
| Makefile | `Makefile` | Available targets |

**What to generate**:

1. **`.ai/SKILL.md`** — Codebase guide covering:
   - Directory structure (list all `pkg/verda/*.go` files with their service types)
   - Core architecture: Client, request helpers, middleware, auth
   - Service pattern (with a real example from the codebase)
   - Validation pattern (how `Validate()` is used, ozzo-validation)
   - Type organization (types.go vs *_types.go)
   - How to add a new API endpoint (step-by-step)
   - Testing patterns (unit + integration)
   - Key Make targets

2. **`.ai/architecture.md`** — Detailed reference covering:
   - Request flow (full chain from service method → middleware → HTTP → response)
   - Client struct layout (exact fields and service types from current code)
   - Middleware struct and methods
   - Service file pattern with real examples
   - Request/response struct conventions with real examples
   - Validation flow
   - Authentication flow
   - Error hierarchy
   - Request helper signatures (all functions from request.go)
   - Test infrastructure

3. **`.ai/.meta`** — Version metadata:
   ```
   commit: <output of git rev-parse --short HEAD>
   generated: <current ISO 8601 timestamp>
   ```

### Keeping Knowledge Fresh During a Session

After making **structural changes** during a session, re-run the scan and update `.ai/` before continuing. Structural changes include:

- Adding, removing, or renaming a service file (`*.go`, `*_types.go`)
- Changing the `Client` struct (new fields, new service wiring)
- Modifying `request.go` signatures or middleware patterns
- Changing `go.mod` dependencies

Non-structural changes (bug fixes within existing methods, test additions, comment edits) do **not** require a rescan.

### Keeping Knowledge Fresh Across Sessions

The `git diff` check in the startup flow handles this automatically:
- If only non-structural files changed since last scan → skip rescan, use cached `.ai/`
- If any file in `pkg/verda/`, `Makefile`, or `go.mod` changed → rescan
- If `.ai/` is missing entirely (new clone, new machine) → full scan

No manual maintenance needed — the code is the source of truth.

## Key Entry Points

Quick reference for navigating the codebase:

| What | Where |
|------|-------|
| Client creation & options | `pkg/verda/client.go` |
| HTTP request helpers (generics) | `pkg/verda/request.go` |
| Authentication (OAuth2) | `pkg/verda/auth.go` |
| Middleware types & implementations | `pkg/verda/middleware.go` |
| Middleware chain management | `pkg/verda/middleware_manager.go` |
| Error types | `pkg/verda/errors.go` |
| Cross-domain types | `pkg/verda/types.go` |
| Shared validation helpers | `pkg/verda/validation.go` |
| Service example | `pkg/verda/instances.go` + `instances_types.go` |
| Test helpers | `pkg/verda/test_helpers.go`, `testutil/mock_server.go` |
| Integration test helpers | `test/integration/helpers.go` |
| Usage example | `example/main.go` |
