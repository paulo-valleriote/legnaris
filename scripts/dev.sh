#!/usr/bin/env bash
# Runs the whole development stack with one command: a clean MongoDB, a clean
# RabbitMQ with the email relay and worker, the API and the web client, all
# reachable on http://localhost:<port>. No Traefik, no OAuth. The email worker
# runs the dev sender, which logs each message body (OTP codes included) so a
# sign-up can be completed locally. Containers are recreated on every run and
# removed on exit.
set -euo pipefail

cd "$(dirname "$0")/.."

readonly MONGO_CONTAINER="legnaris-mongo"
readonly MONGO_IMAGE="mongo:7.0"
readonly MONGO_PORT="${MONGO_PORT:-27017}"
readonly MONGO_KEYFILE_VOLUME="legnaris-dev-mongo-keyfile"
# Development-only root account; the database is published on localhost only.
readonly MONGO_ROOT_USER="root"
readonly MONGO_ROOT_PASSWORD="legnaris-dev-only"
readonly RABBIT_CONTAINER="legnaris-rabbitmq"
# The pinned broker release of deploy/rabbitmq/Dockerfile, so dev matches prod.
readonly RABBIT_IMAGE="rabbitmq:4.4.0"
readonly RABBIT_PORT="${RABBITMQ_PORT:-5672}"
readonly RABBIT_VHOST="legnaris"
readonly RABBIT_USER="legnaris"
# Development-only broker password; the broker is published on localhost only.
readonly RABBIT_PASSWORD="legnaris-dev-only"
readonly BROKER_READY_TIMEOUT=60
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

# shellcheck disable=SC2329 # invoked through the EXIT trap
cleanup() {
	trap - EXIT
	log "shutting down"
	for pid in "${pids[@]:-}"; do
		[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	done
	wait 2>/dev/null || true
	docker rm -fv "$MONGO_CONTAINER" "$RABBIT_CONTAINER" >/dev/null 2>&1 || true
	docker volume rm -f "$MONGO_KEYFILE_VOLUME" >/dev/null 2>&1 || true
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

# start_mongo runs a clean replica set rs0 with a keyfile, which turns authorization
# on, then bootstraps it with the development root account (deploy/mongodb/bootstrap.sh),
# the same first-run path as production.
start_mongo() {
	docker rm -fv "$MONGO_CONTAINER" >/dev/null 2>&1 || true
	docker volume rm -f "$MONGO_KEYFILE_VOLUME" >/dev/null 2>&1 || true
	docker volume create "$MONGO_KEYFILE_VOLUME" >/dev/null
	docker run --rm --entrypoint sh -v "$MONGO_KEYFILE_VOLUME:/etc/mongo-keyfile" \
		-v "$PWD/deploy/mongodb:/provision:ro" "$MONGO_IMAGE" \
		/provision/keyfile.sh /etc/mongo-keyfile >/dev/null || die "could not provision the MongoDB keyfile"

	log "starting a clean MongoDB replica set on :$MONGO_PORT"
	docker run -d --rm --name "$MONGO_CONTAINER" -p "$MONGO_PORT:27017" \
		-v "$MONGO_KEYFILE_VOLUME:/etc/mongo-keyfile:ro" \
		-v "$PWD/deploy/mongodb:/provision:ro" \
		"$MONGO_IMAGE" --replSet rs0 --keyFile /etc/mongo-keyfile/keyfile >/dev/null

	for _ in $(seq 1 60); do
		if docker exec "$MONGO_CONTAINER" mongosh --quiet --eval 'db.adminCommand({ping: 1}).ok' >/dev/null 2>&1; then
			docker exec -e "MONGO_INITDB_ROOT_USERNAME=$MONGO_ROOT_USER" \
				-e "MONGO_INITDB_ROOT_PASSWORD=$MONGO_ROOT_PASSWORD" \
				-e "MONGO_RS_MEMBER=localhost:$MONGO_PORT" \
				"$MONGO_CONTAINER" sh /provision/bootstrap.sh /provision/init/00-replset.js >/dev/null \
				|| die "could not bootstrap the rs0 replica set"
			return 0
		fi
		sleep 1
	done
	die "MongoDB did not become ready within 60s"
}

# start_broker runs a clean RabbitMQ, provisions the delivery vhost and the
# application account, and fails with a clear message if the broker does not
# answer within BROKER_READY_TIMEOUT seconds. Every exec runs as the broker's own
# user: a root exec during boot creates /var/lib/rabbitmq/.erlang.cookie owned by
# root, the broker cannot read it and exits.
start_broker() {
	docker rm -fv "$RABBIT_CONTAINER" >/dev/null 2>&1 || true
	log "starting a clean RabbitMQ on :$RABBIT_PORT"
	docker run -d --rm --name "$RABBIT_CONTAINER" \
		-e "RABBITMQ_DEFAULT_USER=$RABBIT_USER" \
		-e "RABBITMQ_DEFAULT_PASS=$RABBIT_PASSWORD" \
		-p "$RABBIT_PORT:5672" "$RABBIT_IMAGE" >/dev/null

	for _ in $(seq 1 "$BROKER_READY_TIMEOUT"); do
		if docker exec -u rabbitmq "$RABBIT_CONTAINER" rabbitmq-diagnostics -q ping >/dev/null 2>&1; then
			provision_broker
			return 0
		fi
		sleep 1
	done
	die "RabbitMQ did not become ready within ${BROKER_READY_TIMEOUT}s; check: docker logs $RABBIT_CONTAINER"
}

provision_broker() {
	docker exec -u rabbitmq "$RABBIT_CONTAINER" rabbitmqctl -q await_startup >/dev/null
	docker exec -u rabbitmq "$RABBIT_CONTAINER" rabbitmqctl -q add_vhost "$RABBIT_VHOST" >/dev/null 2>&1 || true
	docker exec -u rabbitmq "$RABBIT_CONTAINER" rabbitmqctl -q set_permissions -p "$RABBIT_VHOST" "$RABBIT_USER" ".*" ".*" ".*" >/dev/null \
		|| die "could not grant $RABBIT_USER its rights on the $RABBIT_VHOST vhost"
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
start_broker

export PORT="$API_PORT"
export DATABASE_URI="mongodb://$MONGO_ROOT_USER:$MONGO_ROOT_PASSWORD@localhost:$MONGO_PORT/?replicaSet=rs0&retryWrites=true&authSource=admin"
export CORS_ALLOWED_ORIGINS="http://localhost:$WEB_PORT"
export JWT_SECRET="${JWT_SECRET:-$DEV_JWT_SECRET}"
export EMAIL_CODE_SECRET="${EMAIL_CODE_SECRET:-$DEV_EMAIL_CODE_SECRET}"
export VITE_API_BASE_URL="http://localhost:$API_PORT"
export RABBITMQ_URL="amqp://$RABBIT_USER:$RABBIT_PASSWORD@localhost:$RABBIT_PORT/$RABBIT_VHOST"
export EMAIL_PROVIDER=dev
export APP_ENV=development
# The disposable-domain check and the register limits would reject the
# throwaway addresses and repeated sign-ups a developer makes by hand.
export AUTH_DISPOSABLE_EMAIL_CHECK=false
export AUTH_REGISTER_IP_RATE_LIMIT_ATTEMPTS=100000
export AUTH_REGISTER_EMAIL_RATE_LIMIT_ATTEMPTS=100000

log "building the API, the relay and the worker"
go build -o "$workdir/api" ./backend/cmd/api.go
go build -o "$workdir/relay" ./backend/cmd/relay
go build -o "$workdir/worker" ./backend/cmd/worker

[ -d web/node_modules ] || (log "installing web dependencies" && npm --prefix web ci)

prefixed api "$workdir/api"
prefixed relay "$workdir/relay"
prefixed worker "$workdir/worker"
# shellcheck disable=SC2016 # $1 is expanded by the inner bash, not here
prefixed web bash -c 'cd web && exec node_modules/.bin/vite --host localhost --port "$1" --strictPort' _ "$WEB_PORT"

log "web  http://localhost:$WEB_PORT"
log "api  http://localhost:$API_PORT"
log "mail codes are printed by the [worker] lines (dev sender)"
log "db   mongodb://localhost:$MONGO_PORT/ replica set rs0, authenticated, retryable writes (wiped on exit; Ctrl-C to stop)"

wait -n || true
die "a process exited; stopping the stack"
