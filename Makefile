-include .env
# .env may quote values; make would export them verbatim (quotes included),
# which breaks both the Mongo connection string and dev-db's credential
# parsing — strip double quotes once, at include time.
DATABASE_URI := $(subst ",,$(DATABASE_URI))
# The runtime configuration the Go server reads, resolved from the root .env.
APP_ENV_KEYS = PORT APP_NAME DATABASE_URI DATABASE_NAME CORS_ALLOWED_ORIGINS DATABASE_PING_TIMEOUT DATABASE_SERVER_SELECTION_TIMEOUT DATABASE_CONNECT_TIMEOUT DATABASE_CLOSE_TIMEOUT JWT_SECRET JWT_ACCESS_TTL AUTH_REFRESH_TTL AUTH_BCRYPT_COST AUTH_RATE_LIMIT_ATTEMPTS AUTH_RATE_LIMIT_WINDOW EMAIL_CODE_SECRET AUTH_OTP_TTL AUTH_OTP_CEILING AUTH_OTP_RESEND_COOLDOWN AUTH_OTP_MAX_ATTEMPTS AUTH_OTP_DAILY_SEND_CAP AUTH_OTP_USER_WINDOW AUTH_OTP_USER_LIMIT CSP_REPORT_RATE_LIMIT_ATTEMPTS CSP_REPORT_RATE_LIMIT_WINDOW TRUSTED_PROXY_CIDRS EMAIL_PROVIDER RESEND_API_KEY EMAIL_FROM EMAIL_SEND_TIMEOUT RABBITMQ_URL EMAIL_RELAY_POLL_INTERVAL EMAIL_RELAY_LEASE EMAIL_RELAY_BATCH EMAIL_RELAY_CONFIRM_TIMEOUT EMAIL_RELAY_MAX_ATTEMPTS EMAIL_OUTBOX_RETENTION EMAIL_RELAY_METRICS_ADDR
export $(APP_ENV_KEYS)

# A shell variable outranks a compose environment file, so the local-dev values
# exported above (read from the root .env for `make run`) must be stripped from
# the deploy recipes or they silently override deploy/.env — an unset
# CORS_ALLOWED_ORIGINS, for example, fell through to the compose default
# `http://localhost:3000` and made the deployed API refuse the real frontend
# origin. `env -u` on every key leaves deploy/.env authoritative.
COMPOSE_ENV_UNSET = $(foreach k,$(APP_ENV_KEYS),-u $(k))

.PHONY: run relay build test test-race vet lint lint-specs fmt mod-check mocks dev-db deploy-up deploy-down deploy-local net-check net-fix lint-web test-web sec-scan migrate-emails

## --- Development ------------------------------------------------------------

run:            ## Run the API locally
	@go run ./cmd/api.go

relay:          ## Run the email relay publisher locally (needs DATABASE_URI and RABBITMQ_URL)
	@go run ./cmd/relay

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

mod-check:      ## Verify go.mod/go.sum are tidy
	@go mod tidy -diff

mocks:          ## Generate mocks (after interface changes)
	@go tool mockery

migrate-emails: ## Repair legacy email addresses (dry run; ARGS="-apply" writes)
	@go run ./cmd/canonicalize-emails $(ARGS)

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

## --- Web client (React/Vite/TypeScript PWA) ---------------------------------
## Local parity with the ci-web.yml pipeline; run after any change under web/.

lint-web:       ## Client format-check, ESLint (anti-XSS) and type-check
	@cd web && npm run format:check && npm run lint && npm run typecheck

test-web:       ## Client unit and rendering tests (Vitest)
	@cd web && npm test

sec-scan:       ## Client security gates: SCA, secret scan, lockfile integrity
	@cd web && npm audit --audit-level=high
	@cd web && npm audit signatures
	@cd web && npx lockfile-lint --path package-lock.json --type npm --validate-https --allowed-hosts npm
	@if command -v gitleaks >/dev/null 2>&1; then \
		gitleaks detect --no-banner --redact --source . ; \
	else \
		echo "sec-scan: gitleaks not installed locally; CI runs it (ci-web secrets job)" ; \
	fi

## --- Deploy (deploy/ stack) -------------------------------------------------

deploy-up:      ## Start the production stack
	@env $(COMPOSE_ENV_UNSET) docker compose -f deploy/docker-compose.yml up -d --build

deploy-down:    ## Stop the production stack
	@env $(COMPOSE_ENV_UNSET) docker compose -f deploy/docker-compose.yml down

deploy-local:   ## Start the stack locally via Cloudflare Tunnel
	@env $(COMPOSE_ENV_UNSET) docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.local.yml up -d --build

# Host networking of the Docker-in-Docker dev container, not the stack: a stale
# iptables backend can silently drop egress from the compose bridges, which
# takes the tunnel down (see deploy/README.md §13). Both targets need root.
net-check:      ## Diagnose container egress in the dev container (sudo)
	@sudo scripts/dind_network.sh check

net-fix:        ## Repair stale iptables-legacy rules that block container egress (sudo)
	@sudo scripts/dind_network.sh fix
