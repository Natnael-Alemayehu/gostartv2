# Simple Makefile for a Go project

# Build the application
all: build test

build:
	@echo "Building..."


	@go build -o main cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go

# Run goose migrations up
migrate-up:
	@echo "Running migrations up..."
	@go run ./cmd/migrate up

# Run goose migrations down (one step)
migrate-down:
	@echo "Running migrations down..."
	@go run ./cmd/migrate down

# Show goose migration status
migrate-status:
	@echo "Migration status..."
	@go run ./cmd/migrate status

# Regenerate sqlc code
sqlc-gen:
	@echo "Generating sqlc code..."
	@sqlc generate
# Create DB container
docker-run:
	@if docker compose up --build 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose up --build; \
	fi

# Shutdown DB container
docker-down:
	@if docker compose down 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose down; \
	fi

# Test the application (unit tests only, no integration tests)
test:
	@echo "Testing..."
	@go test ./... -v -race
# Integration tests (requires Docker)
itest:
	@echo "Running integration tests..."
	@go test -tags=integration ./internal/database ./internal/repositories ./internal/auth -v -race

# Clean the binary
clean:
	@echo "Cleaning..."
	@rm -f main

# Lint the codebase
lint:
	@golangci-lint run

# Full verify gate: build + lint + test (run before committing)
verify: build lint test
	@echo "Verify gate passed."

# Live Reload
watch:
	@if command -v air > /dev/null; then \
            air; \
            echo "Watching...";\
        else \
            read -p "Go's 'air' is not installed on your machine. Do you want to install it? [Y/n] " choice; \
            if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
                go install github.com/air-verse/air@latest; \
                air; \
                echo "Watching...";\
            else \
                echo "You chose not to install air. Exiting..."; \
                exit 1; \
            fi; \
        fi

# Show all available make targets
help:
	@echo "Available make targets:"
	@echo "  build            Build the binary"
	@echo "  run              Run the server"
	@echo "  test             Unit tests with -race"
	@echo "  itest            Integration tests (requires Docker)"
	@echo "  lint             Run golangci-lint"
	@echo "  verify           Build + lint + test (full gate)"
	@echo "  sqlc-gen         Regenerate sqlc code"
	@echo "  migrate-up       Apply migrations"
	@echo "  migrate-down     Roll back one migration"
	@echo "  migrate-status   Show migration status"
	@echo "  docker-run       Start Postgres + app via compose"
	@echo "  docker-down      Stop compose services"
	@echo "  watch            Live reload with air"
	@echo "  clean            Remove build artifacts"

.PHONY: all build run test clean watch docker-run docker-down itest lint verify help migrate-up migrate-down migrate-status sqlc-gen
