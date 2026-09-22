-include .env

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