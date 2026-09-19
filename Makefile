-include .env

MIGRATION_DATABASE_URL ?= postgres://postgres:postgres@postgres:5432/legnaris?sslmode=disable

up:
	@docker compose up -d

down:
	@docker compose down

mocks:
	@echo "Generating mocks..."
	@go tool mockery

fmt:
	@echo "Formatting code..."
	@go fmt ./...