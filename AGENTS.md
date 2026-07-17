# AGENTS.md — gostartv2

This file is auto-loaded by opencode at the start of every session. It holds
persistent conventions and decisions for this project. See `PLAN.md` for the
phased roadmap.

## Project Overview

`gostartv2` is an opinionated backend Go starter project. It is being built up
from a barebones skeleton (Melkey Go Blueprint style) into a reusable foundation
for future backend projects.

## Architecture

**Layered (handler → service → repository → model)**

```
cmd/api/main.go              → entry point, bootstrap, graceful shutdown
cmd/migrate/main.go          → goose migration runner (explicit CLI)
internal/
  config/                    → typed Config struct, env loading + validation
  logging/                   → slog setup (JSON prod, text dev)
  httpx/                     → JSON response/error helpers, error envelope
  middleware/                → Recoverer, RequestID, CORS, auth
  database/                  → pgx pool + health check
  db/sqlc/                   → sqlc-generated code (DO NOT EDIT)
  repositories/              → hand-written repos over sqlc queries + tx helpers
  services/                  → business logic layer
  handlers/                  → HTTP handlers, DTOs, request validation
  auth/                      → JWT sign/verify, refresh tokens, auth middleware
  models/                    → domain types
db/queries/*.sql             → sqlc query sources
migrations/*.sql             → goose SQL migrations (embedded via go:embed)
```

### Layering rules
- **Handlers** parse HTTP requests, call services, return HTTP responses. No DB access.
- **Services** contain business logic, call repositories. No HTTP awareness.
- **Repositories** execute SQL via sqlc-generated queries. No business logic.
- **Models** are plain domain types shared across layers.
- Dependencies flow inward: handlers → services → repositories. Never reverse.

## Key Decisions

| Area | Choice |
|---|---|
| HTTP router | `go-chi/chi/v5` |
| DB driver | `github.com/jackc/pgx/v5` via `database/sql` |
| DB queries | sqlc — write SQL, generate type-safe Go |
| Migrations | goose — SQL files, embedded, explicit CLI runner |
| Config | Lightweight typed struct over godotenv (no viper) |
| Logging | `log/slog` (stdlib) — JSON in prod, text in dev |
| JWT | `golang-jwt/jwt/v5` |
| Password hashing | `golang.org/x/crypto/bcrypt` |
| Validation | `go-playground/validator/v10` |
| Testing | stdlib `testing` + testcontainers for DB integration tests |
| Code quality | `samber/cc-skills-golang` — Code Quality category (8 skills) |
| Doc comments | Required on all exported symbols + package declarations |
| Lint config | Full golangci-lint v2 config (33+ linters) per `golang-lint` skill |

## Code Quality Skills

This project adopts the **Code Quality category** of `samber/cc-skills-golang` as its code quality authority. The skills are installed at `~/.agents/skills/cc-skills-golang/skills/` and auto-discovered by opencode from `~/.agents/skills/`.

| Skill | Scope |
|---|---|
| `golang-code-style` | Line length, variable declarations, control flow, function design, code organization |
| `golang-documentation` | godoc comments, README, CONTRIBUTING, CHANGELOG, llms.txt, example functions |
| `golang-error-handling` | Error creation, wrapping (`%w`), `errors.Is`/`AsType`, single handling rule, panic/recover |
| `golang-lint` | golangci-lint config, `//nolint` directives, linter selection, CI integration |
| `golang-naming` | Packages, structs, interfaces, constants, errors, receivers, acronyms, test functions |
| `golang-safety` | nil safety, append aliasing, map concurrent access, numeric overflow, defensive copying |
| `golang-security` | Injection prevention, crypto, filesystem/network safety, secrets management, cookies |
| `golang-structs-interfaces` | Composition, embedding, interface segregation, struct tags, pointer vs value receivers |

When in doubt, defer to the skill. When ignoring a skill rule, add a comment to the code explaining why.

## Conventions

### Error handling
- **Never use `log.Fatalf` or `log.Fatal` in request handlers or health checks** — return an error response instead. `log.Fatal` is only acceptable in `main` during bootstrap if a required dependency fails to start.
- Use `internal/httpx` helpers for all HTTP responses:
  - `httpx.RespondJSON(w, status, data)` — success responses
  - `httpx.RespondError(w, status, code, message)` — error responses
- Error envelope shape: `{"error":{"code":"...","message":"..."}}`
- Always set `Content-Type: application/json` on JSON responses.
- **Single handling rule** (per `golang-error-handling`): errors MUST be either logged OR returned, never both.
- **Wrap with context**: use `fmt.Errorf("{context}: %w", err)`. Avoid "failed to" prefix.
- **Error strings must be lowercase**, no trailing punctuation, include package prefix for sentinels (e.g. `errors.New("services: user not found")`).
- **Use `errors.Is`** for sentinel matching and **`errors.AsType[T]`** (Go 1.26+) for typed chain inspection.
- **Use `errors.Join`** (Go 1.20+) to combine independent errors.
- **Never expose technical errors to users** — translate internal errors to user-friendly messages via `httpx.RespondError`.

### Configuration
- All config is loaded once in `main` via `internal/config.Load()` into a typed `Config` struct.
- No `os.Getenv` calls outside the `config` package.
- `.env` is loaded via `godotenv` in the config package only (not via blank-import autoload scattered across packages).
- Required env vars are validated at startup; missing required vars cause a clear error message and exit.

### Logging
- Use `slog` (from `log/slog`), not the stdlib `log` package.
- Logger is created in `internal/logging` and passed via context or struct fields.
- JSON handler in production (`APP_ENV=production`), text handler in development.

### Database
- sqlc-generated code lives in `internal/db/sqlc` — **do not edit manually**. Regenerate with `make sqlc-gen`.
- Migrations are SQL files in `migrations/`, embedded into the binary via `go:embed`.
- Run migrations via `cmd/migrate` CLI: `make migrate-up`, `make migrate-down`.
- The app never auto-runs migrations on startup.

### API design
- All API routes under `/api/v1/` prefix.
- Health endpoints: `GET /health` (liveness — process is up), `GET /ready` (readiness — DB is reachable). Pool stats are not exposed publicly.
- Request bodies are validated via `validator/v10` struct tags.
- DTOs (request/response types) live in `handlers` package, separate from domain `models`.

### Testing
- Unit tests use stdlib `testing`.
- DB integration tests use testcontainers-go (spin up real Postgres).
- Shared testcontainers helper lives in `internal/testutil`.
- `make test` runs all tests with `-race`; `make itest` runs DB integration tests (`internal/database` + `internal/repositories`).
- Use `t.Context()` (Go 1.24+) instead of `context.Background()` in tests.
- Use `t.Cleanup()` for teardown instead of `defer`.
- Run tests with the race detector: `go test -race ./...`.

### Naming (per `golang-naming` skill)
- All identifiers use **MixedCaps** — uppercase exported, lowercase unexported. Never underscores (except test subcases).
- **Acronyms** are all-caps or all-lower: `ID` not `Id`, `HTTPServer` not `HttpServer`, `urlParser`.
- **Constructors**: `New()` for single-type packages, `NewTypeName()` for multi-type.
- **Getters**: `Name()` not `GetName()`. Boolean predicates keep `Is`/`Has`/`Can` prefix: `IsConnected()`.
- **Receivers**: 1-2 letter abbreviation, consistent across all methods of a type (`s` for Server, `r` for Repository).
- **Error vars**: `Err` prefix (`ErrNotFound`). **Error types**: `Error` suffix (`PathError`).
- **Error strings**: fully lowercase including acronyms (`"invalid user id"`), include package prefix for sentinels.
- **No stuttering**: `http.Client` not `http.HTTPClient`, `user.New()` not `user.NewUser()`.
- **Constants**: MixedCaps (`MaxRetries`), not `ALL_CAPS`.
- **Enum zero values**: place `Unknown`/`Invalid` sentinel at iota position 0.
- **Avoid** `util`/`helper` package names — use specific names that describe the abstraction.

### Safety & Security (per `golang-safety` + `golang-security` skills)
- **nil map writes panic** — always initialize maps with `make` before use.
- **Safe type assertions** — always use comma-ok form (`v, ok := x.(T)`), never bare assertions.
- **Typed nil in interface is not nil** — return bare `nil` for the nil case, not a typed nil pointer.
- **`append` may reuse backing array** — use 3-index slice `s[:len(s):len(s)]` or `slices.Clone` to prevent aliasing.
- **SQL must use parameterized queries** — sqlc generates these; never concatenate SQL strings.
- **Never hardcode secrets** — use env vars or secret managers. The `.env` file is gitignored.
- **bcrypt for passwords** (already adopted) — never MD5/SHA1 for passwords.
- **Constant-time comparison** for secrets: `crypto/subtle.ConstantTimeCompare`, never `==`.
- **HTTP response bodies and `sql.Rows` must be closed** — defer close immediately after error check.
- **Validate all input at trust boundaries** — handlers validate via `validator/v10`; services clamp/validate business rules.

### Structs & Interfaces (per `golang-structs-interfaces` skill)
- **Accept interfaces, return structs** — constructors return concrete types, never interfaces.
- **Define interfaces where consumed** — the consumer defines only what it needs (e.g. `userService` interface in `handlers`, `userRepo` interface in `services`).
- **Keep interfaces small** — 1-3 methods. Compose larger interfaces from small ones.
- **Compile-time interface checks**: `var _ Interface = (*Type)(nil)` near the type definition.
- **Pointer vs value receivers**: be consistent across all methods of a type. Use pointer receivers when modifying, for large structs, or when containing `sync` types.
- **Struct field tags** on all exported fields in serialized types (`json:"name"`, `validate:"required"`).
- **Don't create interfaces prematurely** — start concrete, extract when 2+ implementations or testability demands it.

### Linting (per `golang-lint` skill)
- `.golangci.yml` is the **source of truth** for linters — currently a full 33+ linter config.
- `//nolint` directives MUST specify the linter name and include a justification: `//nolint:errcheck // reason`.
- **Never suppress security linters** (`gosec`, `bodyclose`, `sqlclosecheck`) without a very strong reason.
- Run `golangci-lint run ./...` after every significant change.
- Use `golangci-lint run --fix ./...` for auto-fixable issues.

## Build & Verify Commands

```bash
make build          # go build -o main cmd/api/main.go
make run            # go run cmd/api/main.go
make test           # go test ./... -v -race
make itest          # go test ./internal/database ./internal/repositories -v -race
make lint           # golangci-lint run
make clean          # rm -f main
make docker-run     # docker compose up --build
make docker-down    # docker compose down
make watch          # air live reload
make migrate-up     # run goose migrations up
make migrate-down   # run goose migrations down (one step)
make migrate-status # show goose migration status
make sqlc-gen       # regenerate sqlc code
```

**Verify gate (per phase):** `go build ./...` + `make lint` + `make test` must all pass before advancing to the next phase.

## File editing rules
- Do not edit `internal/db/sqlc/` — it is sqlc-generated.
- **Doc comments required**: every exported function, method, type, constant, and package declaration MUST have a godoc comment starting with the symbol name. Follow `golang-documentation` skill conventions. Package comments use `// Package foo ...`.
- No inline comments inside function bodies unless explaining non-obvious logic. When ignoring a skill rule, add a comment explaining why.
- **Blank imports** (`_ "pkg"`) only in `cmd/` packages, `*_test.go` files, and test utility packages (`internal/testutil`). Internal packages must not register driver side effects.
- **Sentinel errors must include the package name prefix**: `errors.New("services: user not found")`.
- Follow existing import grouping: stdlib first, third-party second, local last.
- Mimic existing code style and naming conventions.
