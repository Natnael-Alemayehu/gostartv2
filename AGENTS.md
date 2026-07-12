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

## Conventions

### Error handling
- **Never use `log.Fatalf` or `log.Fatal` in request handlers or health checks** — return an error response instead. `log.Fatal` is only acceptable in `main` during bootstrap if a required dependency fails to start.
- Use `internal/httpx` helpers for all HTTP responses:
  - `httpx.RespondJSON(w, status, data)` — success responses
  - `httpx.RespondError(w, status, code, message)` — error responses
- Error envelope shape: `{"error":{"code":"...","message":"..."}}`
- Always set `Content-Type: application/json` on JSON responses.

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
- `make test` runs all tests; `make itest` runs only DB integration tests.

## Build & Verify Commands

```bash
make build          # go build -o main cmd/api/main.go
make run            # go run cmd/api/main.go
make test           # go test ./... -v
make itest          # go test ./internal/database -v
make lint           # golangci-lint run
make clean          # rm -f main
make docker-run     # docker compose up --build
make docker-down    # docker compose down
make watch          # air live reload
# Phase 2 additions:
# make migrate-up    # run goose migrations up
# make migrate-down  # run goose migrations down
# make sqlc-gen      # regenerate sqlc code
```

**Verify gate (per phase):** `go build ./...` + `make lint` + `make test` must all pass before advancing to the next phase.

## File editing rules
- Do not edit `internal/db/sqlc/` — it is sqlc-generated.
- Do not add comments to Go files unless explicitly asked.
- Follow existing import grouping: stdlib first, third-party second, local last.
- Mimic existing code style and naming conventions.
