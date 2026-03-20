# AGENTS.md

Instructions for AI agents working with this codebase.

## Codebase Skill

Read [`.ai/SKILL.md`](.ai/SKILL.md) first to understand the project structure, patterns, and conventions. It covers:

- Directory layout and file purposes
- Core architecture (Client, type-safe generics, middleware pipeline)
- Service pattern for each API resource
- How to add new API endpoints (step-by-step)
- Testing patterns (unit with mocks, integration with real API)
- Key Make targets

## Architecture Deep Dive

For detailed reference on request flow, struct layouts, middleware internals, auth flow, and error hierarchy, see [`.ai/architecture.md`](.ai/architecture.md).

## Key Entry Points

| What | Where |
|------|-------|
| Client creation & options | `pkg/verda/client.go` |
| HTTP request helpers (generics) | `pkg/verda/request.go` |
| Authentication (OAuth2) | `pkg/verda/auth.go` |
| Middleware chain | `pkg/verda/middleware.go`, `middleware_manager.go` |
| Error types | `pkg/verda/errors.go` |
| Service example | `pkg/verda/instances.go` + `instances_types.go` |
| Unit test example | `pkg/verda/instances_test.go` |
| Integration test helpers | `test/integration/helpers.go` |
| Usage example | `example/main.go` |

## Conventions

- **One service per file**: `<resource>.go` for methods, `<resource>_types.go` for types
- **All methods take `context.Context`** as first parameter
- **Request validation**: Implement `Validate() error` using `ozzo-validation`
- **Error handling**: Return `*APIError` for HTTP errors, `*ValidationError` for input issues
- **Tests**: Unit tests use `testutil.NewMockServer()`, integration tests use `//go:build integration`
- **Commits**: Conventional commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`)
