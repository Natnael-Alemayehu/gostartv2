# gostartv2 — Development Plan

A backend Go starter project. This file tracks the phased roadmap for turning
the barebones skeleton into a usable, opinionated starter. Checkboxes track
progress. This file can be deleted once all phases are complete.

## Decisions

| Area | Decision |
|---|---|
| Architecture | Layered: handlers → services → repositories → models |
| DB queries | sqlc (generate type-safe Go from SQL) |
| Migrations | goose (SQL files, embedded via `go:embed`) |
| Migration trigger | Explicit CLI only (`cmd/migrate`, `make migrate-up`) |
| Auth | JWT access + refresh tokens (refresh stored in DB, hashed) |
| Observability | slog + chi Recoverer + RequestID |
| Example resource | Full User CRUD (model, migration, repo, service, handler, tests) |
| Go version | Latest stable, pinned across go.mod / Dockerfile / CI |
| Config | Lightweight typed struct over godotenv (no viper) |
| Password hashing | `golang.org/x/crypto/bcrypt` |
| Validation | `go-playground/validator/v10` |
| JWT | `golang-jwt/jwt/v5` |
| Code quality skills | `samber/cc-skills-golang` — Code Quality category (8 skills) |
| Doc comments | Required on all exported symbols + package declarations |
| Lint config | Full golangci-lint v2 config (33+ linters) per `golang-lint` skill |
| Verify gate | Strict: `go build ./...` + `make lint` + `make test` all green per phase |
| Commit cadence | One commit per completed phase, only after user approval |

## Target Structure

```
cmd/
  api/main.go              # bootstrap: config, logger, db, server
  migrate/main.go          # goose migration runner subcommand
internal/
  config/                  # typed Config struct + env validation/defaults
  logging/                 # slog setup (JSON/prod, text/dev by APP_ENV)
  httpx/                   # respondJSON/respondError + error envelope
  middleware/              # Recoverer, RequestID, auth, CORS wrapper
  database/                # pgx pool + ping (Health bug fixed)
  db/sqlc/                 # sqlc-generated, do not edit
  repositories/            # hand-written repos over sqlc queries + tx helpers
  services/                # business logic
  handlers/                # HTTP handlers (DTOs + validation)
  auth/                    # JWT sign/verify, refresh, middleware
  models/                  # domain types
db/queries/*.sql           # sqlc query sources
migrations/*.sql           # goose SQL migrations (embedded)
sqlc.yaml                  # at root
```

---

## Phase 1 — Foundation (bug fixes + core plumbing)

- [x] **1.1 Mechanical fixes**
  - Unify Go version across `go.mod` / `Dockerfile` / CI workflows
  - Fix `.goreleaser.yml` `<user>/<repo>` placeholder
  - Add `LICENSE` file
- [x] **1.2 `internal/config`**
  - Typed `Config` struct (AppEnv, Port, DB, JWT, CORS, etc.)
  - Load from env via godotenv with defaults + validation of required keys
  - Actually use `APP_ENV`
- [x] **1.3 `internal/logging`**
  - slog logger setup (JSON in prod, text in dev)
  - Wired in `main`, available via context
- [x] **1.4 `internal/httpx`**
  - `respondJSON(w, status, data)` — sets Content-Type, encodes JSON
  - `respondError(w, status, code, msg)` — consistent `{error:{code,message}}` envelope
- [x] **1.5 `internal/middleware`**
  - chi `Recoverer` (panics no longer crash the process)
  - chi `RequestID` (correlation IDs)
  - Config-driven CORS wrapper (safer default, not wildcard+credentials)
- [x] **1.6 Refactor `internal/server`**
  - Wire config, logging, httpx, new middleware into server
  - Fix `log.Fatalf` in `database.Health()` — return "down" status, don't crash
  - Fix `log.Fatalf` in `HelloWorldHandler` — return error response
  - Set `Content-Type: application/json` on all responses
  - Split `/health` (liveness) vs `/ready` (DB readiness)
  - Stop leaking pool stats publicly
- [x] **1.7 Linting setup**
  - Add `.golangci.yml` config
  - Add `make lint` target to Makefile
- [x] **1.8 Verify gate**
  - `go build ./...` passes
  - `make lint` passes
  - `make test` passes

---

## Phase 2 — DB layer + migrations + User CRUD

- [x] **2.1 sqlc setup**
  - `sqlc.yaml` config at root
  - `db/queries/user.sql` query sources
  - Generate type-safe Go into `internal/db/sqlc`
  - `make sqlc-gen` target
- [x] **2.2 goose migrations**
  - `migrations/00001_users.sql` (embedded via `go:embed`)
  - `cmd/migrate` runner with `up/down/status` subcommands
  - `make migrate-up`, `make migrate-down` targets
- [x] **2.3 `internal/models`**
  - `User` domain type
- [x] **2.4 `internal/repositories`**
  - `UserRepository` over sqlc queries
  - `WithTx` transaction helper
  - Context-aware methods
- [x] **2.5 `internal/services`**
  - `UserService` (create/get/list/update/delete)
  - bcrypt password hashing
- [x] **2.6 `internal/handlers`**
  - `UserHandler` CRUD endpoints
  - Request DTOs + `validator/v10` validation
- [x] **2.7 Routes**
  - `/api/v1/users` route group
- [x] **2.8 Tests**
  - Service unit tests (mock repo)
  - Handler `httptest` tests
  - Repository integration tests (testcontainers)
- [x] **2.9 Verify gate** — build + lint + test all green

---

## Phase 2.5 — Code Quality standards adoption

Adopt the `samber/cc-skills-golang` Code Quality category (8 skills) as the project's code quality authority. See AGENTS.md for the full conventions.

- [x] **2.5.1 Doc comments**
  - Add godoc comments to all exported functions, methods, types, constants
  - Add `// Package foo ...` declarations to every package
  - Follow `golang-documentation` skill conventions
- [x] **2.5.2 Blank imports cleanup**
  - Move `_ "github.com/jackc/pgx/v5/stdlib"` to `cmd/api/main.go` and `cmd/migrate/main.go`
  - Remove blank imports from `internal/database` and `internal/testutil`
- [x] **2.5.3 Sentinel error package prefix**
  - Update existing sentinel errors to include package prefix
  - e.g. `errors.New("services: user not found")`
- [x] **2.5.4 Expand `.golangci.yml`**
  - Adopt `golang-lint` skill's recommended 33-linter config
  - Add gosec, bodyclose, sqlclosecheck, errorlint, nolintlint, etc.
  - Add gofumpt formatter
- [x] **2.5.5 Fix lint issues**
  - Run `golangci-lint run --fix ./...` for auto-fixable issues
  - Manually fix remaining issues from expanded linter set
- [x] **2.5.6 Verify gate** — build + lint + test all green

---

## Phase 3 — Auth (JWT access + refresh)

- [x] **3.1 `internal/auth`**
  - JWT sign/verify (`golang-jwt/jwt/v5`)
  - Claims struct, token generation
  - Refresh token generation
- [x] **3.2 Refresh token storage**
  - `refresh_tokens` migration
  - `RefreshTokenRepository` (DB-stored, hashed)
- [x] **3.3 `AuthService`**
  - login, refresh, logout, logout-all
  - **Deviation:** registration lives at `POST /api/v1/users` (public, from Phase 2)
    rather than `/api/v1/auth/register`. The auth route group does not
    expose a register endpoint.
- [x] **3.4 Auth routes**
  - `/api/v1/auth/{login,refresh,logout,logout-all}`
- [x] **3.5 Auth middleware**
  - Verify access token, inject user into context
  - Protect `/users` routes (except create/register as appropriate)
- [x] **3.6 Tests**
  - Auth flow integration tests (E2E against real Postgres)
  - Middleware unit tests
- [x] **3.7 Verify gate** — build + lint + test all green

---

## Phase 4 — CI, docs, hardening

Split into 4a (critical fixes discovered by the Phase 3 audit) and 4b
(docs, CI, and hardening from the original plan).

### 4a — Critical fixes (audit findings)

- [ ] **4a.1 `.dockerignore`**
  - Exclude `.env`, `.git/`, `tmp/`, `main` from Docker build context
  - Prevents `COPY . .` from copying secrets into image layers
- [ ] **4a.2 Fix goreleaser version injection**
  - `-X` ldflag path is `github.com/Natnael-Alemayehu/gostartv2/cmd/api` but
    module is `gostartv2`; released binaries report `Version = "dev"`
  - Either rename module to `github.com/Natnael-Alemayehu/gostartv2` or
    fix the ldflag path
- [ ] **4a.3 Add lint job to CI**
  - `.github/workflows/go-test.yml` only runs build + test; no lint job
  - Add `golangci/golangci-lint-action` job so the verify gate is enforced on PRs
- [ ] **4a.4 README rewrite**
  - Current README is Melkey Go Blueprint boilerplate
  - Needs: project description, architecture overview, env var table, API
    endpoint docs, dev setup guide, migration instructions, "How to add a
    resource" guide, auth flow documentation
- [ ] **4a.5 `.env.example`**
  - 18 env vars exist in `.env` but are undiscoverable without reading
    `config.go`; provide a template with defaults and comments
- [ ] **4a.6 Update PLAN.md**
  - Phase 3 checkboxes were unchecked but code was committed; fixed here

### 4b — Docs, CI, hardening (original Phase 4 scope)

- [ ] **4b.1 CI improvements**
  - `govulncheck` security scan
  - `gosec` SAST scan (per `golang-security` skill)
  - Fix `go-test` Go version to match go.mod (currently pinned `1.26.x`)
  - Add `-race` flag to CI test job (currently missing)
  - Add integration test job (`-tags=integration`) with Docker-skip guard
  - Add `go mod tidy` check (`git diff --exit-code`)
  - Add sqlc-freshness check (`sqlc generate` + `git diff --exit-code`)
- [ ] **4b.2 Documentation (per `golang-documentation` skill)**
  - CONTRIBUTING.md (prerequisites, clone, build, test, PR process)
  - CHANGELOG.md (Keep a Changelog format)
  - llms.txt (AI-friendly project overview)
- [ ] **4b.3 Docker hardening**
  - Non-root user in final image
  - Add `ca-certificates`
  - Pin base image versions consistently (build stage unpinned, runtime pinned)
  - Add app healthcheck in compose
  - Document `.env` DB_HOST trap (compose `psql_bp` vs host `localhost`)
  - Document migration step (`docker compose up` starts app with no tables)
- [ ] **4b.4 Project hygiene**
  - Dependabot config
  - `.editorconfig`
  - Optional: Docker image publish job in CI
  - Rename module to `github.com/Natnael-Alemayehu/gostartv2` (repo-qualified)
  - Run `go mod tidy` (jwt/v5 is in indirect require block)
  - Add `make verify` aggregate target (build + lint + test)
  - Add `make help` self-documenting target
- [ ] **4b.5 Security & observability hardening**
  - Add request body size limit (`http.MaxBytesReader`) to `DecodeJSON`
  - Add rate limiting middleware (login brute force, public registration)
  - Replace chi Logger with slog-based request logging (JSON in prod)
  - Wire request-scoped logger via middleware (request IDs in log records)
  - Add minimum-length validation for `JWT_SECRET` in prod
  - Fix dead required-var validation (defaults applied before emptiness check)
  - Document authorization non-goal (any authenticated user can access any
    other user's data; no ownership check or roles)
  - Optional: security-headers middleware (X-Content-Type-Options, etc.)
  - Optional: expired-refresh-token purge job
- [ ] **4b.6 Missing tests**
  - Unit tests for `config`, `logging`, `httpx` packages
- [ ] **4b.7 Final verify gate** — build + lint + test + itest all green
