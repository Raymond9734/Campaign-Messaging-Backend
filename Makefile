.PHONY: help setup build run-api run-worker test clean docker-up docker-down docker-rebuild migrate-up migrate-down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup: ## Setup .env file from .env.example (if not exists)
	@if [ ! -f .env ]; then \
		cp .env.example .env && \
		echo "✓ Created .env from .env.example"; \
		echo "⚠️  Please review and update .env with your settings"; \
	else \
		echo ".env already exists"; \
	fi

build: ## Build API and worker binaries
	go build -o bin/api cmd/api/main.go
	go build -o bin/worker cmd/worker/main.go

run-api: ## Run the API server
	go run cmd/api/main.go

run-worker: ## Run the worker
	go run cmd/worker/main.go

test: ## Run tests
	go test -v -race -cover ./...

test-coverage: ## Run tests with coverage report
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

docker-up: ## Start all services with docker-compose
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Run 'make setup' first."; \
		exit 1; \
	fi
	docker-compose up 

docker-down: ## Stop all services
	docker-compose down

docker-build: ## Rebuild images and restart all services
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found. Run 'make setup' first."; \
		exit 1; \
	fi
	docker-compose up --build

docker-logs: ## Show docker logs
	docker-compose logs -f

migrate-up: ## Run database migrations (schema + seed data)
	@echo "Running database migrations..."
	@if [ ! -f .env ]; then echo "Error: .env file not found. Copy .env.example to .env first."; exit 1; fi
	@export $$(cat .env | xargs) && \
		PGPASSWORD=$$DB_PASSWORD psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d $$DB_NAME -f migrations/001_initial_schema_up.sql && \
		PGPASSWORD=$$DB_PASSWORD psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d $$DB_NAME -f migrations/002_seed_data_up.sql
	@echo "✓ Migrations completed successfully"

migrate-down: ## Rollback database migrations (removes all data)
	@echo "Rolling back database migrations..."
	@if [ ! -f .env ]; then echo "Error: .env file not found"; exit 1; fi
	@export $$(cat .env | xargs) && \
		PGPASSWORD=$$DB_PASSWORD psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d $$DB_NAME -f migrations/002_seed_data_down.sql && \
		PGPASSWORD=$$DB_PASSWORD psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d $$DB_NAME -f migrations/001_initial_schema_down.sql
	@echo "✓ Rollback completed successfully"

migrate-schema-only: ## Run only schema migration (no seed data)
	@echo "Running schema migration..."
	@if [ ! -f .env ]; then echo "Error: .env file not found"; exit 1; fi
	@export $$(cat .env | xargs) && \
		PGPASSWORD=$$DB_PASSWORD psql -h $$DB_HOST -p $$DB_PORT -U $$DB_USER -d $$DB_NAME -f migrations/001_initial_schema_up.sql
	@echo "✓ Schema created successfully"

deps: ## Download dependencies
	go mod download
	go mod tidy

lint: ## Run linter (requires golangci-lint)
	@which golangci-lint > /dev/null 2>&1 || test -f ~/go/bin/golangci-lint || (echo "❌ golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && echo "Then add ~/go/bin to your PATH or run: export PATH=\$$PATH:~/go/bin" && exit 1)
	@if which golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		~/go/bin/golangci-lint run ./...; \
	fi

.DEFAULT_GOAL := help
