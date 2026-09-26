-include .env
export PORT APP_NAME DATABASE_URI DATABASE_NAME CORS_ALLOWED_ORIGINS DATABASE_PING_TIMEOUT DATABASE_SERVER_SELECTION_TIMEOUT DATABASE_CONNECT_TIMEOUT DATABASE_CLOSE_TIMEOUT

.PHONY: run build test test-race vet lint fmt mocks deploy-up deploy-down deploy-local

## --- Development ------------------------------------------------------------

run:            ## Run the API locally
	@go run ./cmd/api.go

build:          ## Build the API binary
	@go build ./cmd/api.go

test:           ## Run tests (integration tests self-skip without MongoDB)
	@go test ./...

test-race:      ## Run tests with the race detector
	@go test -race ./...

vet:            ## Run go vet
	@go vet ./...

lint:           ## Run golangci-lint
	@golangci-lint run

fmt:            ## Format code
	@go fmt ./...

mocks:          ## Generate mocks (after interface changes)
	@go tool mockery

## --- Deploy (deploy/ stack) -------------------------------------------------

deploy-up:      ## Start the production stack
	@docker compose -f deploy/docker-compose.yml up -d --build

deploy-down:    ## Stop the production stack
	@docker compose -f deploy/docker-compose.yml down

deploy-local:   ## Start the stack locally via Cloudflare Tunnel
	@docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.local.yml up -d --build
