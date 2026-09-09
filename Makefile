-include .env

MIGRATION_DATABASE_URL ?= postgres://postgres:postgres@postgres:5432/legnaris?sslmode=disable

db-up:
	@docker compose up -d postgres

db-down:
	@docker compose down

migrate-up:
	@docker compose run --rm migrate -path=/migrations -database="$(MIGRATION_DATABASE_URL)" up

migrate-down:
	@docker compose run --rm migrate -path=/migrations -database="$(MIGRATION_DATABASE_URL)" down 1

mocks:
	@echo "Generating mocks..."
	@go tool mockery