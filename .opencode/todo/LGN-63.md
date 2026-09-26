# LGN-63 Implementation Todo

## Files to modify

- [x] shared/env/env.go - Add DatabaseName and 4 timeout durations, remove default credentials from DATABASE_URI
- [x] shared/env/env_test.go - Add tests for lookupDurationEnv and lookupRequiredStringEnv
- [x] shared/db/database.go - Read timeouts from env, use timeout for Disconnect on ping failure
- [x] internal/app/bootstrap.go - NewModules accepts dbName parameter
- [x] cmd/api.go - Pass DatabaseName to NewModules, use DatabaseCloseTimeout for Close
- [x] internal/app/integration_test.go - Update NewModules call, use timeout for Close
- [x] Makefile - Add export for env vars
- [x] .env.example - Add DATABASE_NAME and timeout variables
- [x] AGENTS.md - Update env vars list and Quick Start note
- [x] deploy/docker-compose.yml - Add DATABASE_NAME to app service

## Validation
- [x] make fmt && make vet && make lint (lint skipped - golangci-lint not installed)
- [x] make test && make test-race
- [x] Smoke test: make run with .env works (fails with connection refused since no MongoDB), fails without DATABASE_URI