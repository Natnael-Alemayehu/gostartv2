# gostartv2

An opinionated Go backend starter project — layered architecture, JWT auth,
sqlc + goose, and strict code-quality tooling. Built to be a reusable
foundation for future backend projects.

## Features

- **Layered architecture** — handlers → services → repositories → models
- **JWT auth** — access tokens + rotating refresh tokens with reuse detection
- **sqlc** — type-safe Go from SQL, no ORM
- **goose migrations** — SQL files, embedded, explicit CLI runner
- **golangci-lint v2** — 40+ linters including gosec, bodyclose, sqlclosecheck
- **Testcontainers** — integration tests against real Postgres
- **slog** — structured logging (JSON in prod, text in dev)
- **chi router** — lightweight, idiomatic Go HTTP
- **validator/v10** — request body validation via struct tags

## Prerequisites

- **Go 1.26+**
- **Docker** (for integration tests and docker-compose dev environment)
- **sqlc** (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
- **golangci-lint v2** (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- **goose** (`go install github.com/pressly/goose/v3/cmd/goose@latest`) — optional, `make migrate-*` uses `go run`

## Quick Start

### With Docker Compose (recommended)

```bash
# 1. Copy env template and adjust
cp .env.example .env
# For docker compose, set DB_HOST=psql_bp in .env

# 2. Start Postgres + app
make docker-run

# 3. Run migrations (in another terminal, from host)
#    Note: DB_HOST must point to the compose service
DB_HOST=localhost DB_PORT=5432 make migrate-up
#    Or exec inside the container:
docker compose exec app ./cmd/migrate up
```

### Local development

```bash
# 1. Start Postgres only
docker compose up -d psql_bp

# 2. Copy env template
cp .env.example .env
# Ensure DB_HOST=localhost for local dev

# 3. Run migrations
make migrate-up

# 4. Run the server
make run
```

The server starts on `http://localhost:8080`.

## Environment Variables

Copy `.env.example` to `.env` and adjust. All vars have sensible defaults
except where noted.

| Variable | Default | Required | Description |
|---|---|---|---|
| `APP_ENV` | `local` | — | `local`, `development`, `staging`, `production`, `test` |
| `PORT` | `8080` | — | HTTP server port |
| `DB_HOST` | `localhost` | — | Postgres host (`psql_bp` in compose, `localhost` for local dev) |
| `DB_PORT` | `5432` | — | Postgres port |
| `DB_NAME` | `gostartv2` | — | Database name |
| `DB_USER` | `postgres` | — | Database user |
| `DB_PASSWORD` | — | — | Database password |
| `DB_SCHEMA` | `public` | — | Postgres schema |
| `DB_SSLMODE` | `disable` | — | Postgres SSL mode |
| `DB_MAX_CONNS` | `25` | — | Connection pool max open connections |
| `DB_MAX_IDLE` | `5` | — | Connection pool max idle connections |
| `JWT_SECRET` | — | **prod** | HMAC-SHA256 signing secret (required in `production`) |
| `JWT_ACCESS_TTL` | `15m` | — | Access token lifetime |
| `JWT_REFRESH_TTL` | `168h` | — | Refresh token lifetime (7 days) |
| `JWT_ISSUER` | `gostartv2` | — | JWT issuer claim |
| `CORS_ALLOWED_ORIGINS` | `*` | — | Comma-separated origins (`*` forbidden in prod) |
| `CORS_ALLOW_CREDENTIALS` | `false` | — | Allow cookies in CORS |

## API Endpoints

### Health

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | — | Liveness check (process is up) |
| `GET` | `/ready` | — | Readiness check (DB is reachable) |

### Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/auth/login` | — | Exchange credentials for tokens |
| `POST` | `/api/v1/auth/refresh` | Cookie + Bearer | Rotate refresh token, issue new access token |
| `POST` | `/api/v1/auth/logout` | — | Revoke refresh token, clear cookie |
| `POST` | `/api/v1/auth/logout-all` | Bearer | Revoke all sessions for the user |

### Users

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/users` | — | Register a new user |
| `GET` | `/api/v1/users` | Bearer | List users (cursor pagination) |
| `GET` | `/api/v1/users/{id}` | Bearer | Get a user by ID |
| `PUT` | `/api/v1/users/{id}` | Bearer | Update a user |
| `DELETE` | `/api/v1/users/{id}` | Bearer | Delete a user |

## Auth Flow

The auth system uses **JWT access tokens** + **rotating refresh tokens**:

1. **Login** (`POST /api/v1/auth/login`):
   - Validates email/password (bcrypt)
   - Returns `{access_token, token_type, expires_in}` in JSON body
   - Sets refresh token in an `HttpOnly; SameSite=Lax` cookie

2. **Refresh** (`POST /api/v1/auth/refresh`):
   - Reads refresh token from cookie
   - Requires `Authorization: Bearer <access>` header (CSRF defense)
   - Revokes old refresh token, issues new one in same chain
   - Returns new access token + new refresh cookie

3. **Logout** (`POST /api/v1/auth/logout`):
   - Revokes the refresh token (idempotent)
   - Clears the cookie

4. **Logout-all** (`POST /api/v1/auth/logout-all`):
   - Requires valid access token
   - Revokes all refresh tokens for the user

**Reuse detection:** if a revoked refresh token is presented again, the entire
session chain is revoked and the client must re-authenticate.

> **Note:** There is no authorization layer — any authenticated user can
> access any other user's data. This is a deliberate scope cut for the
> starter template. Add ownership checks or roles before production use.

## Database & Migrations

Migrations are SQL files in `migrations/`, embedded into the binary via
`go:embed`. The app never auto-runs migrations — use the explicit CLI:

```bash
make migrate-up      # apply all pending migrations
make migrate-down    # roll back one migration
make migrate-status  # show current migration state
```

sqlc queries live in `db/queries/`. Regenerate Go code after editing SQL:

```bash
make sqlc-gen
```

## Testing

```bash
make test    # unit tests with -race (no integration tag)
make itest   # integration tests with -race (requires Docker)
```

Unit tests use stdlib `testing` with function-field mocks. Integration tests
use testcontainers-go to spin up real Postgres containers.

## How to Add a Resource

To add a new resource (e.g. `posts`), follow the User pattern:

1. **Migration** — create `migrations/00003_posts.sql` (goose Up/Down)
2. **sqlc queries** — create `db/queries/post.sql` with `:one`/`:many`/`:exec`
3. **Regenerate** — `make sqlc-gen` (produces `internal/db/sqlc/post.sql.go`)
4. **Model** — create `internal/models/post.go` (plain struct, no JSON tags)
5. **Repository** — create `internal/repositories/post_repository.go`,
   add `Posts` to the `Repositories` struct in `repositories.go`
6. **Service** — create `internal/services/post_service.go` with a
   consumer-side `postRepo` interface, business logic, sentinel errors
7. **Handler** — create `internal/handlers/post_handler.go` with DTOs,
   validation, and a consumer-side `postService` interface
8. **Routes** — mount routes in `internal/server/routes.go`
9. **Tests** — unit tests for service + handler, integration tests for repo

## Project Structure

```
cmd/
  api/main.go              # bootstrap + graceful shutdown
  migrate/main.go          # goose migration CLI
internal/
  config/                  # typed Config, env loading + validation
  logging/                 # slog setup (JSON prod, text dev)
  httpx/                   # JSON response/error helpers
  middleware/              # RequestID, Recoverer, CORS, Auth
  database/                # pgx pool + health check
  db/sqlc/                 # sqlc-generated (do not edit)
  repositories/            # hand-written repos + tx helpers
  services/                # business logic + sentinel errors
  handlers/                # HTTP handlers, DTOs, validation
  auth/                    # JWT sign/verify, refresh token gen/hash
  models/                  # domain types
  testutil/                # testcontainers Postgres helper
db/queries/*.sql           # sqlc query sources
migrations/*.sql           # goose SQL migrations (embedded)
sqlc.yaml                  # sqlc config
```

## Make Targets

| Target | Description |
|---|---|
| `make build` | Build the binary |
| `make run` | Run the server |
| `make test` | Unit tests with `-race` |
| `make itest` | Integration tests (requires Docker) |
| `make lint` | Run golangci-lint |
| `make sqlc-gen` | Regenerate sqlc code |
| `make migrate-up` | Apply migrations |
| `make migrate-down` | Roll back one migration |
| `make migrate-status` | Show migration status |
| `make docker-run` | Start Postgres + app via compose |
| `make docker-down` | Stop compose services |
| `make watch` | Live reload with air |
| `make clean` | Remove build artifacts |

## License

MIT — see [LICENSE](LICENSE).
