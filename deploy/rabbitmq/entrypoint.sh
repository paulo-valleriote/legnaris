#!/bin/sh
# Starts the broker through the image's own entrypoint and provisions it beside
# it. rabbitmqctl reaches the broker by its Erlang node name, which a separate
# container cannot know, so the provisioning runs in the broker's container.
# A failed provisioning stops the broker so the restart policy retries it; a
# broker that exits ends the container with the broker's status.
set -u

docker-entrypoint.sh "$@" &
broker=$!
trap 'kill -TERM "$broker" 2>/dev/null' TERM INT

gosu rabbitmq legnaris-provision &
provision=$!
if ! wait "$provision"; then
  kill -TERM "$broker" 2>/dev/null
fi

wait "$broker"
status=$?
if kill -0 "$broker" 2>/dev/null; then
  wait "$broker"
  status=$?
fi
exit "$status"
