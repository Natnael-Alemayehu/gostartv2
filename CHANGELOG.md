# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Rate limiting middleware on login (5 rps, burst 10) and registration (3 rps, burst 5)
- `make verify` aggregate target (build + lint + test)
- `make help` self-documenting target
- `.editorconfig` for consistent editor configuration
- `.github/dependabot.yml` for automated dependency updates
- `.dockerignore` to prevent secret leakage into Docker image layers
- `.env.example` template with all 18 environment variables documented
- `CONTRIBUTING.md` with setup and PR process
- `llms.txt` for AI-friendly project overview
- Request body size limit (1 MB) on JSON decoding via `http.MaxBytesReader`
- slog-based structured request logging (replaces chi's stdlib Logger)
- JWT_SECRET minimum length validation (16 chars) in production
- App healthcheck in docker-compose.yml
- Non-root user in Dockerfile production image
- `ca-certificates` and `tzdata` in Docker production image
- golangci-lint job in CI
- `-race` flag in CI test job

### Changed
- Dockerfile: pinned both base images (build: `golang:1.26-alpine`, prod: `alpine:3.22`)
- Dockerfile: added version ldflags via `git describe`
- goreleaser: fixed `PACKAGE_PATH` to match actual module path (`gostartv2/cmd/api`)
- goreleaser: added `LICENSE` to archive files
- `config.Validate()`: fixed dead required-var checks (now checks raw env vars)
- `internal/middleware/middleware.go`: `Logger()` now uses slog instead of chi's stdlib Logger
- README: full rewrite with architecture, env vars, API docs, auth flow, dev guide
- `go.mod`: `golang-jwt/jwt/v5` moved from indirect to direct require
- `PLAN.md`: Phase 3 marked complete, Phase 4 split into 4a (critical) + 4b (hardening)

### Security
- `.dockerignore` prevents `.env` (containing JWT_SECRET, DB_PASSWORD) from entering image layers
- Rate limiting mitigates brute-force attacks on login and automated registration
- Body size limit prevents memory exhaustion from unbounded request bodies
- JWT secret validation enforces minimum length in production
- Non-root Docker user reduces container privilege escalation risk

## [0.3.0] — Phase 3: JWT Auth + Rotating Refresh Tokens

### Added
- `internal/auth` package: JWT sign/verify (HS256), refresh token generation (crypto/rand), SHA-256 hashing
- `internal/repositories/refresh_token_repository.go`: Create, GetByHash, Revoke, RevokeChain, RevokeAllForUser
- `internal/services/auth_service.go`: Login, Refresh (rotation + reuse detection), Logout, LogoutAll
- `internal/handlers/auth_handler.go`: HttpOnly cookie transport, CSRF defense via Bearer header
- `internal/middleware/auth.go`: Bearer token verification, user injection into context
- `migrations/00002_refresh_tokens.sql`: refresh_tokens table with chain_id, revoked_at, token_hash UNIQUE
- Auth sentinels: ErrInvalidCredentials, ErrTokenExpired, ErrTokenInvalid, ErrRefreshTokenRevoked, ErrRefreshTokenReuse, ErrRefreshTokenExpired
- E2E integration test: register → login → refresh → logout → reuse detection → logout-all
- 66 unit tests + 36 integration tests

### Security
- Refresh tokens stored as SHA-256 hashes (never plaintext in DB)
- Refresh token rotation with chain reuse detection (revokes entire chain on reuse)
- HttpOnly, SameSite=Lax cookies with Secure flag in production
- CSRF defense: /refresh requires Authorization header alongside cookie
- Generic error messages prevent username enumeration

## [0.2.0] — Phase 2 + 2.5: DB Layer, User CRUD, Code Quality

### Added
- sqlc setup with `db/queries/user.sql` and `internal/db/sqlc/` generated code
- goose migrations with `cmd/migrate` CLI runner (up, down, status, reset, redo)
- `internal/models/user.go`: User, UserCreate, UserUpdate, PageCursor, ListUsersInput
- `internal/repositories/user_repository.go`: CRUD + WithTx transaction helper
- `internal/services/user_service.go`: business logic + bcrypt password hashing
- `internal/handlers/user_handler.go`: CRUD endpoints + validator/v10 validation
- Cursor-based pagination (replaced limit/offset with (created_at, id) tuple cursor)
- `internal/database/database_integration_test.go`: //go:build integration
- `.golangci.yml` with 40+ linters (gosec, bodyclose, sqlclosecheck, errorlint, etc.)
- Code Quality skills adoption: godoc on all exported symbols, sentinel error prefixes
- `Health(ctx)` refactor: context-aware DB health check

### Changed
- Split DB tests into unit (no Docker) and integration (//go:build integration)
- Makefile: `make test` = unit only, `make itest` = -tags=integration

## [0.1.0] — Phase 1: Foundation

### Added
- `internal/config`: typed Config struct, env loading via godotenv, production validation
- `internal/logging`: slog setup (JSON in prod, text in dev)
- `internal/httpx`: RespondJSON, RespondError, RespondNoContent, DecodeJSON
- `internal/middleware`: Recoverer, RequestID, CORS (config-driven)
- `internal/server`: wired config, logging, httpx, middleware
- Split `/health` (liveness) vs `/ready` (DB readiness)
- Fixed `log.Fatalf` in `database.Health()` and `HelloWorldHandler`
- `make lint` target
- Docker Compose with Postgres 16 + app
- GoReleaser config for cross-platform builds
- GitHub Actions CI workflow
