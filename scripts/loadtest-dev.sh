#!/usr/bin/env bash
# One command for the whole local load test, entirely in containers: MongoDB
# and the API (rate limits lifted), a dev Prometheus + Grafana, and the k6 PATCH
# run streaming into the dashboards. Everything is torn down on exit. DEV ONLY.
#
# Tune with RATE, DURATION and USERS (see loadtest/README.md). KEEP_UP=1 leaves
# Grafana running afterwards so the dashboards can be inspected.
set -euo pipefail

cd "$(dirname "$0")/.."

readonly COMPOSE=(docker compose --progress plain -f deploy/docker-compose.dev-observability.yml --profile loadtest)
readonly GRAFANA_URL="http://localhost:3000/d/legnaris-loadtest"

log() { printf '[loadtest] %s\n' "$*"; }

cleanup() {
	trap - EXIT
	if [ "${KEEP_UP:-0}" = 1 ]; then
		"${COMPOSE[@]}" stop app mongodb >/dev/null 2>&1 || true
		log "Grafana left running: $GRAFANA_URL (stop with: make dev-observability-down)"
	else
		"${COMPOSE[@]}" down -v >/dev/null 2>&1 || true
	fi
}

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }

trap cleanup EXIT
trap 'exit 130' INT TERM

log "building and starting MongoDB, the API, Prometheus and Grafana"
"${COMPOSE[@]}" up -d --build --wait mongodb app prometheus grafana

log "dashboards: $GRAFANA_URL"
"${COMPOSE[@]}" run --rm k6
