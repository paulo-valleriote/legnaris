#!/usr/bin/env bash
# Runs the whole development stack with one command: a clean MongoDB, the API
# and the web client, all reachable on http://localhost:<port>. No Traefik, no
# OAuth, no broker. The database is recreated on every run and removed on exit.
set -euo pipefail

cd "$(dirname "$0")/.."

readonly MONGO_CONTAINER="legnaris-mongo"
readonly MONGO_IMAGE="mongo:7.0"
readonly MONGO_PORT="${MONGO_PORT:-27017}"
readonly API_PORT="${PORT:-8080}"
readonly WEB_PORT="${WEB_PORT:-5173}"

# Development-only fallbacks so the stack boots without a .env. Real values in
# the environment or in the root .env win. EMAIL_CODE_SECRET must be >= 32 bytes
# and differ from JWT_SECRET.
readonly DEV_JWT_SECRET="dev-only-jwt-secret-do-not-use-in-production-0123456789"
readonly DEV_EMAIL_CODE_SECRET="dev-only-email-code-secret-do-not-use-in-production-9876543210"

pids=()
workdir=""

log() { printf '[dev] %s\n' "$*"; }

die() {
	printf '[dev] error: %s\n' "$*" >&2
	exit 1
}

require() {
	command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not installed"
}

cleanup() {
	trap - EXIT
	log "shutting down"
	for pid in "${pids[@]:-}"; do
		[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	done
	wait 2>/dev/null || true
	docker rm -fv "$MONGO_CONTAINER" >/dev/null 2>&1 || true
	[ -n "$workdir" ] && rm -rf "$workdir"
}

load_dotenv() {
	[ -f .env ] || return 0
	local key
	for key in JWT_SECRET EMAIL_CODE_SECRET; do
		[ -n "${!key:-}" ] && continue
		local value
		value=$(sed -nE "s/^${key}=//p" .env | head -n1 | tr -d '"')
		[ -n "$value" ] && export "$key=$value"
	done
}

start_mongo() {
	docker rm -fv "$MONGO_CONTAINER" >/dev/null 2>&1 || true
	log "starting a clean MongoDB on :$MONGO_PORT"
	docker run -d --rm --name "$MONGO_CONTAINER" -p "$MONGO_PORT:27017" "$MONGO_IMAGE" >/dev/null

	local attempt
	for attempt in $(seq 1 60); do
		if docker exec "$MONGO_CONTAINER" mongosh --quiet --eval 'db.adminCommand({ping: 1}).ok' >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	die "MongoDB did not become ready within 60s"
}

# prefixed runs a command with its output labelled, in the background, and
# records the PID of the command itself (not of the labelling filter). The API
# logs one warning per unset optional variable; they are dropped as noise.
prefixed() {
	local label="$1"
	shift
	"$@" > >(sed -u -e '/environment variable not set; using default/d' -e "s/^/[$label] /") 2>&1 &
	pids+=("$!")
}

require docker
require go
require node
require npm

trap cleanup EXIT
trap 'exit 130' INT TERM
workdir=$(mktemp -d)

load_dotenv
start_mongo

export PORT="$API_PORT"
export DATABASE_URI="mongodb://localhost:$MONGO_PORT/"
export CORS_ALLOWED_ORIGINS="http://localhost:$WEB_PORT"
export JWT_SECRET="${JWT_SECRET:-$DEV_JWT_SECRET}"
export EMAIL_CODE_SECRET="${EMAIL_CODE_SECRET:-$DEV_EMAIL_CODE_SECRET}"
export VITE_API_BASE_URL="http://localhost:$API_PORT"

log "building the API"
go build -o "$workdir/api" ./backend/cmd/api.go

[ -d web/node_modules ] || (log "installing web dependencies" && npm --prefix web ci)

prefixed api "$workdir/api"
prefixed web bash -c 'cd web && exec node_modules/.bin/vite --host localhost --port "$1" --strictPort' _ "$WEB_PORT"

log "web  http://localhost:$WEB_PORT"
log "api  http://localhost:$API_PORT"
log "db   mongodb://localhost:$MONGO_PORT/ (wiped on exit; Ctrl-C to stop)"

wait -n || true
die "a process exited; stopping the stack"
