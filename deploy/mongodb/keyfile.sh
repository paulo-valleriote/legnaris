#!/bin/sh
# Provisions the replica-set keyfile into the directory given as $1. An existing
# keyfile is kept, so the script is safe to run on every stack start. mongod
# refuses a keyfile that is readable by group or others, and the official image
# runs mongod as uid 999, so the file is owned by that uid and set to mode 400.
# KEYFILE_OWNER overrides the owner for tests that run without root.
set -eu

dir="${1:?usage: keyfile.sh <dir>}"
owner="${KEYFILE_OWNER:-999:999}"
file="$dir/keyfile"

mkdir -p "$dir"
if [ ! -s "$file" ]; then
	head -c 756 /dev/urandom | base64 -w 0 >"$file"
fi
chown "$owner" "$file"
chmod 400 "$file"
