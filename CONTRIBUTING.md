# Contributing to gostartv2

Thanks for contributing! This guide gets you set up in under 10 minutes.

## Prerequisites

- **Go 1.26+** — [install](https://go.dev/doc/install)
- **Docker** — for integration tests and the dev database
- **sqlc** — `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- **golangci-lint v2** — `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`

## Setup

```bash
git clone https://github.com/Natnael-Alemayehu/gostartv2.git
cd gostartv2
cp .env.example .env
make docker-run        # starts Postgres + app
make migrate-up        # apply migrations (in another terminal)
```

## Development Workflow

```bash
make run               # run the server
make test              # unit tests with -race
make itest             # integration tests (requires Docker)
make lint              # golangci-lint (40+ linters)
make verify            # build + lint + test (full gate)
```

## Code Conventions

This project follows the `samber/cc-skills-golang` Code Quality category.
See [AGENTS.md](AGENTS.md) for the full conventions. Key rules:

- **Doc comments** on all exported symbols + package declarations
- **Error handling**: wrap with `%w`, lowercase strings, package-prefix sentinels
- **Naming**: MixedCaps, `ID` not `Id`, `Err` prefix for sentinels
- **Linting**: `//nolint` must specify linter name + justification
- **Testing**: `t.Context()`, `t.Cleanup()`, `//go:build integration` for DB tests
- **No `os.Getenv`** outside `internal/config`
- **No `log.Fatal`** in handlers or services — only in `main` bootstrap

## Adding a New Resource

See the [README](README.md#how-to-add-a-resource) for the 9-step guide.

## Pull Request Process

1. Run `make verify` before pushing — build + lint + test must all pass
2. Write clear commit messages following the existing style (`feat:`, `refactor:`, `docs:`)
3. Keep PRs focused — one feature or fix per PR
4. Add tests for new functionality (unit + integration where applicable)
5. Update documentation if you change public API or env vars

## Project Structure

See [README](README.md#project-structure) for the full directory layout.

## License

MIT — see [LICENSE](LICENSE).
