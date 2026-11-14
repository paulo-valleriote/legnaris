#!/bin/sh
# Brings a MongoDB member started with --replSet and --keyFile to its first-run
# state: it initiates replica set rs0, creates the root user, then runs the given
# scripts as that root user. Every stack runs it after the member is up.
#
# The official image's first-run path cannot do this: its temporary init server
# drops --replSet and --keyFile and binds only to 127.0.0.1, so a replica-set
# initiate there fails. The member is therefore configured first and bootstrapped
# afterwards, over the localhost exception, which is the only way to act before the
# first user exists. The script must run in a localhost-connected shell: the
# one-shot Compose service shares the member's network namespace, and dev stacks
# use docker exec.
#
# Idempotent: rs.initiate runs only while the member does not report rs0, the root
# user is created only when the root credentials do not authenticate, and the
# scripts after the first must check for existing users themselves. A root user
# with a different password makes the createUser fail loudly, never silently.
#
# usage: bootstrap.sh <replset.js> [<script.js>...]
# env:   MONGO_INITDB_ROOT_USERNAME, MONGO_INITDB_ROOT_PASSWORD (required),
#        MONGO_RS_MEMBER (the address the member is reached at), MONGO_PORT (27017)
set -eu

replset="${1:?usage: bootstrap.sh <replset.js> [<script.js>...]}"
shift
admin="mongodb://localhost:${MONGO_PORT:-27017}/admin"
user="${MONGO_INITDB_ROOT_USERNAME:?MONGO_INITDB_ROOT_USERNAME is required}"
pass="${MONGO_INITDB_ROOT_PASSWORD:?MONGO_INITDB_ROOT_PASSWORD is required}"

mongosh --quiet "$admin" --file "$replset" >/dev/null

if ! mongosh --quiet "$admin" -u "$user" -p "$pass" --authenticationDatabase admin \
	--eval '1' >/dev/null 2>&1; then
	mongosh --quiet "$admin" --eval \
		'db.getSiblingDB("admin").createUser({user: process.env.MONGO_INITDB_ROOT_USERNAME, pwd: process.env.MONGO_INITDB_ROOT_PASSWORD, roles: ["root"]})' \
		>/dev/null
fi

for script in "$@"; do
	mongosh --quiet "$admin" -u "$user" -p "$pass" --authenticationDatabase admin \
		--file "$script" >/dev/null
done
