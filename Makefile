-include .env
# .env may quote values; make would export them verbatim (quotes included),
# which breaks both the Mongo connection string and dev-db's credential
# parsing — strip double quotes once, at include time.
DATABASE_URI := $(subst ",,$(DATABASE_URI))
# The runtime configuration the Go server reads, resolved from the root .env.
APP_ENV_KEYS = PORT APP_NAME DATABASE_URI DATABASE_NAME CORS_ALLOWED_ORIGINS DATABASE_PING_TIMEOUT DATABASE_SERVER_SELECTION_TIMEOUT DATABASE_CONNECT_TIMEOUT DATABASE_CLOSE_TIMEOUT JWT_SECRET JWT_ACCESS_TTL AUTH_REFRESH_TTL AUTH_BCRYPT_COST AUTH_RATE_LIMIT_ATTEMPTS AUTH_RATE_LIMIT_WINDOW EMAIL_CODE_SECRET AUTH_OTP_TTL AUTH_OTP_CEILING AUTH_OTP_RESEND_COOLDOWN AUTH_OTP_MAX_ATTEMPTS AUTH_OTP_DAILY_SEND_CAP AUTH_OTP_USER_WINDOW AUTH_OTP_USER_LIMIT AUTH_REQUIRE_EMAIL_VERIFICATION CSP_REPORT_RATE_LIMIT_ATTEMPTS CSP_REPORT_RATE_LIMIT_WINDOW TRUSTED_PROXY_CIDRS EMAIL_PROVIDER RESEND_API_KEY EMAIL_FROM EMAIL_SEND_TIMEOUT RABBITMQ_URL EMAIL_RELAY_POLL_INTERVAL EMAIL_RELAY_LEASE EMAIL_RELAY_BATCH EMAIL_RELAY_CONFIRM_TIMEOUT EMAIL_RELAY_MAX_ATTEMPTS EMAIL_OUTBOX_RETENTION EMAIL_RELAY_METRICS_ADDR EMAIL_RETRY_DELAY_FIRST EMAIL_RETRY_DELAY_SECOND EMAIL_RETRY_DELAY_LATER EMAIL_WORKER_PREFETCH EMAIL_WORKER_MAX_ATTEMPTS EMAIL_WORKER_METRICS_ADDR
export $(APP_ENV_KEYS)

# A shell variable outranks a compose environment file, so the local-dev values
# exported above (read from the root .env for `make run`) must be stripped from
# the deploy recipes or they silently override deploy/.env — an unset
# CORS_ALLOWED_ORIGINS, for example, fell through to the compose default
# `http://localhost:3000` and made the deployed API refuse the real frontend
# origin. `env -u` on every key leaves deploy/.env authoritative.
COMPOSE_ENV_UNSET = $(foreach k,$(APP_ENV_KEYS),-u $(k))

.PHONY: dev run relay worker build test test-race vet lint fmt mod-check mocks dev-db deploy-up deploy-down deploy-local net-check net-fix lint-web test-web sec-scan migrate-emails bench-dag loadtest loadtest-seed dev-observability dev-observability-down loadtest-dev k8s-up k8s-down k8s-observability k8s-rules k8s-loadtest k8s-keda

## --- Development ------------------------------------------------------------

dev:            ## Run a clean MongoDB, the API and the web client with one command (http://localhost:5173)
	@scripts/dev.sh

run:            ## Run the API locally
	@go run ./backend/cmd/api.go

relay:          ## Run the email relay publisher locally (needs DATABASE_URI and RABBITMQ_URL)
	@go run ./backend/cmd/relay

worker:         ## Run the email delivery worker locally (needs RABBITMQ_URL; EMAIL_PROVIDER defaults to log)
	@go run ./backend/cmd/worker

build:          ## Build the API binary
	@go build ./backend/cmd/api.go

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

mod-check:      ## Verify go.mod/go.sum are tidy
	@go mod tidy -diff

bench-dag:      ## Benchmark the DAG recalculation (LGN-33: p95 < 50ms at 200 fields)
	@go test ./backend/internal/sheet/template -run 'P95' -bench 'NormalizeAndRecalculate' -benchmem

loadtest:       ## k6 load test against a running API (SCRIPT=patch-sheet.js|capacity.js|concurrency.js|onboarding.js|soak.js|stress.js|auth-storm.js; BASE_URL, RATE, DURATION, USERS, STAGES, STEP_SECONDS; K6_OUT=experimental-prometheus-rw streams to dev-observability)
	@docker run --rm -i --network host -v $(CURDIR)/loadtest/k6:/scripts:ro \
		-e BASE_URL=$${BASE_URL:-http://localhost:8080} -e RATE -e DURATION -e USERS -e STAGES -e STEP_SECONDS -e MAX_VUS -e P95_BUDGET_MS -e VARIANT -e CONCURRENCY -e ITERATIONS -e ONBOARDING_RATE -e SOAK_RATE -e SOAK_SECONDS -e STRESS_STAGES -e LOGIN_STAGES -e BACKGROUND_RATE \
		-e K6_PROMETHEUS_RW_SERVER_URL=$${K6_PROMETHEUS_RW_SERVER_URL:-http://localhost:9090/api/v1/write} \
		-e K6_PROMETHEUS_RW_TREND_STATS=p\(95\),p\(99\),avg,max -e K6_PROMETHEUS_RW_PUSH_INTERVAL=5s \
		grafana/k6:0.57.0 run $${K6_OUT:+--out $$K6_OUT} /scripts/$${SCRIPT:-patch-sheet.js}

loadtest-seed:  ## Seed USERS accounts (1 sheet each) so load tests start on a populated DB; idempotent (BASE_URL, USERS)
	@docker run --rm -i --network host -v $(CURDIR)/loadtest/k6:/scripts:ro \
		-e BASE_URL=$${BASE_URL:-http://localhost:8080} -e USERS=$${USERS:-1000} -e ACCOUNT_PREFIX -e SEED_PARALLELISM \
		grafana/k6:0.57.0 run /scripts/seed.js

loadtest-dev:   ## EVERYTHING in one command, all in containers: DB + API + Grafana + k6 PATCH run, torn down after (SCRIPT=capacity.js|concurrency.js|onboarding.js|soak.js|stress.js|auth-storm.js; KEEP_UP=1 keeps Grafana; RATE/DURATION/USERS/STAGES)
	@scripts/loadtest-dev.sh

dev-observability: ## Prometheus + Grafana for `make dev` (Grafana: http://localhost:3000, dev only)
	@docker compose -f deploy/docker-compose.dev-observability.yml up -d

dev-observability-down: ## Stop the dev Prometheus + Grafana
	@docker compose -f deploy/docker-compose.dev-observability.yml down

mocks:          ## Generate mocks (after interface changes)
	@go tool mockery

migrate-emails: ## Repair legacy email addresses (dry run; ARGS="-apply" writes)
	@go run ./backend/cmd/canonicalize-emails $(ARGS)

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

## --- Local Kubernetes (k3d) --------------------------------------------------

K8S_OVERLAY := deploy/k8s/overlays/local
K8S_RENDER = kubectl kustomize --load-restrictor LoadRestrictionsNone $(K8S_OVERLAY)

k8s-up:         ## Create the k3d cluster, build+import images and apply the stack (see deploy/k8s/README.md)
	@test -f $(K8S_OVERLAY)/secrets.env || { \
		echo "k8s-up: generating $(K8S_OVERLAY)/secrets.env with random values" ; \
		for k in MONGO_ROOT_PASSWORD MONGO_APP_PASSWORD MONGO_EXPORTER_PASSWORD MONGO_VIEW_PASSWORD JWT_SECRET EMAIL_CODE_SECRET RABBITMQ_PASSWORD RABBITMQ_OPS_PASSWORD ; do \
			echo "$$k=$$(openssl rand -hex 24)" ; \
		done > $(K8S_OVERLAY)/secrets.env ; }
	@k3d cluster list legnaris >/dev/null 2>&1 || k3d cluster create --config $(K8S_OVERLAY)/k3d.yaml
	docker build -t legnaris-app:local .
	docker build -t legnaris-web:local web
	docker build -t legnaris-rabbitmq:local deploy/rabbitmq
	k3d image import -c legnaris legnaris-app:local legnaris-web:local legnaris-rabbitmq:local
	$(K8S_RENDER) | kubectl apply -f -
	kubectl rollout status deployment/app deployment/web deployment/relay deployment/worker --timeout=300s
	@echo "API: http://api.legnaris.localhost:8080  Web: http://app.legnaris.localhost:8080"

k8s-down:       ## Destroy the local k3d cluster and everything in it
	@k3d cluster delete legnaris

K8S_PROM_CHART_VERSION := 92.17.1

k8s-observability: ## Install kube-prometheus-stack, PodMonitors, alert rules and dashboards into the k3d cluster
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
	helm upgrade --install kps prometheus-community/kube-prometheus-stack --version $(K8S_PROM_CHART_VERSION) \
		--namespace monitoring --create-namespace -f deploy/k8s/observability/values.yaml --wait --timeout 10m
	kubectl kustomize --load-restrictor LoadRestrictionsNone deploy/k8s/observability | kubectl apply -n monitoring -f -
	@echo "Grafana: kubectl -n monitoring port-forward svc/kps-grafana 3000:80 (admin/admin)"

k8s-rules:      ## Regenerate the PrometheusRule from deploy/prometheus/rules.yml
	@python3 scripts/gen_prometheusrule.py

K8S_KEDA_CHART_VERSION := 2.21.0

k8s-keda:       ## Install KEDA and the worker/relay ScaledObjects (needs k8s-observability)
	helm repo add kedacore https://kedacore.github.io/charts --force-update
	helm upgrade --install keda kedacore/keda --version $(K8S_KEDA_CHART_VERSION) \
		--namespace keda --create-namespace -f deploy/k8s/keda/values.yaml --wait --timeout 5m
	kubectl apply -f deploy/k8s/keda/scaledobjects.yaml

SCRIPT ?= capacity.js
k8s-loadtest:   ## Run a k6 script as a Job on a DISPOSABLE cluster (rate limits lifted): make k8s-loadtest SCRIPT=capacity.js
	@kubectl delete job k6 --ignore-not-found
	kubectl kustomize --load-restrictor LoadRestrictionsNone deploy/k8s/overlays/loadtest \
		| sed 's#/scripts/capacity.js#/scripts/$(SCRIPT)#' | kubectl apply -f -
	kubectl rollout restart deployment/app && kubectl rollout status deployment/app
	kubectl logs -f job/k6
