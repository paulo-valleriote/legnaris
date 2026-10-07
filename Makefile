-include .env
# .env may quote values; make would export them verbatim (quotes included),
# which breaks both the Mongo connection string and dev-db's credential
# parsing — strip double quotes once, at include time.
DATABASE_URI := $(subst ",,$(DATABASE_URI))
export PORT APP_NAME DATABASE_URI DATABASE_NAME CORS_ALLOWED_ORIGINS DATABASE_PING_TIMEOUT DATABASE_SERVER_SELECTION_TIMEOUT DATABASE_CONNECT_TIMEOUT DATABASE_CLOSE_TIMEOUT JWT_SECRET JWT_ACCESS_TTL AUTH_REFRESH_TTL AUTH_BCRYPT_COST

.PHONY: run build test test-race vet lint lint-specs fmt mocks dev-db deploy-up deploy-down deploy-local

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

lint-specs:     ## Run spec/test lint rules (CI parity: assertions, requirement IDs, citations)
	@python3 scripts/test_lints.py
	@python3 scripts/check_test_assertions.py
	@python3 scripts/check_requirement_ids.py
	@python3 scripts/check_validation_citations.py

fmt:            ## Format code
	@go fmt ./...

mocks:          ## Generate mocks (after interface changes)
	@go tool mockery

# dev-db derives MongoDB's root credentials from DATABASE_URI (exported from
# .env above), so the container can never diverge from what the app and
# `make test-race` connect with. A URI without credentials (see .env.example)
# starts an unauthenticated MongoDB; credentials are used verbatim — do not
# percent-encode them.
dev-db:         ## Run only MongoDB locally with the credentials from DATABASE_URI; stop with: docker stop legnaris-mongo
	@creds=$$(printf '%s' '$(DATABASE_URI)' | sed -nE 's#^mongodb://([^/@]+)@.*#\1#p'); \
	if [ -n "$$creds" ]; then \
		set -- -e "MONGO_INITDB_ROOT_USERNAME=$${creds%%:*}" -e "MONGO_INITDB_ROOT_PASSWORD=$${creds#*:}"; \
	else \
		set --; \
	fi; \
	docker run --rm --name legnaris-mongo "$$@" \
		-p 27017:27017 \
		-d mongo:7.0

## --- Deploy (deploy/ stack) -------------------------------------------------

deploy-up:      ## Start the production stack
	@docker compose -f deploy/docker-compose.yml up -d --build

deploy-down:    ## Stop the production stack
	@docker compose -f deploy/docker-compose.yml down

deploy-local:   ## Start the stack locally via Cloudflare Tunnel
	@docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.local.yml up -d --build
