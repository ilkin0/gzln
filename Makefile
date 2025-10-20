# Database configuration
DB_URL=postgresql://postgres:VERY_SECURE_DB_PASSWORD@localhost:5432/gzln_db?sslmode=disable

# Database management
createdb:
	docker exec -it gzln_db createdb --username=postgres gzln_db

dropdb:
	docker exec -it gzln_db dropdb --username=postgres gzln_db

# Goose migrations
goose-up:
	goose -dir db/migration postgres "$(DB_URL)" up

goose-down:
	goose -dir db/migration postgres "$(DB_URL)" down

goose-status:
	goose -dir db/migration postgres "$(DB_URL)" status

goose-reset:
	goose -dir db/migration postgres "$(DB_URL)" reset

goose-create:
	goose -dir db/migration create $(name) sql

# SQL generation
sqlc:
	sqlc generate

# Development
dev:
	@echo "Starting development servers..."
	@make -j2 dev-backend dev-frontend

dev-backend:
	@echo "Starting Go backend with Air..."
	@air

dev-frontend:
	@echo "Starting Svelte frontend..."
	@cd web && npm run dev

air-init:
	air init

# Go commands
build:
	go build -o bin/server cmd/server/main.go

run:
	go run cmd/server/main.go

test:
	go test -v -cover ./...

test-short:
	go test -short -v ./...

# Frontend commands
test-frontend:
	@echo "Running frontend tests..."
	@cd web && npm test

test-frontend-watch:
	@echo "Running frontend tests in watch mode..."
	@cd web && npm run test:watch

# Run all tests
test-all:
	@echo "Running all tests..."
	@make test
	@make test-frontend

vet:
	go vet ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

.PHONY: createdb dropdb goose-up goose-down goose-status goose-reset goose-create sqlc dev dev-backend dev-frontend air-init build run test test-short test-frontend test-frontend-watch test-all vet fmt tidy
