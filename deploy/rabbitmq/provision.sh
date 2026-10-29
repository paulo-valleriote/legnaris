#!/bin/sh
# Idempotent broker provisioning, run by legnaris-entrypoint once the broker is
# up and on every start: the delivery vhost, the operator's console account and
# the application account's rights. Every step may be repeated; the
# application's rights come last because the health check reads them as the
# signal that provisioning finished (BRK-09, BRK-10, BRK-11).
set -eu

vhost=legnaris
operator=operator

attempts=0
until rabbitmq-diagnostics -q ping >/dev/null 2>&1; do
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 120 ]; then
    echo "the broker did not come up within 120s" >&2
    exit 1
  fi
  sleep 1
done
rabbitmqctl await_startup

rabbitmqctl -q --no-table-headers list_vhosts | grep -qx "$vhost" \
  || rabbitmqctl add_vhost "$vhost"

if [ -n "${RABBITMQ_OPS_PASSWORD:-}" ]; then
  if rabbitmqctl -q --no-table-headers list_users | cut -f1 | grep -qx "$operator"; then
    rabbitmqctl change_password "$operator" "$RABBITMQ_OPS_PASSWORD"
  else
    rabbitmqctl add_user "$operator" "$RABBITMQ_OPS_PASSWORD"
  fi
  rabbitmqctl set_user_tags "$operator" management
  rabbitmqctl set_permissions -p "$vhost" "$operator" ".*" ".*" ".*"
else
  echo "RABBITMQ_OPS_PASSWORD is empty: the operator account is left as it is" >&2
fi

rabbitmqctl set_user_tags "$vhost"
rabbitmqctl clear_permissions -p / "$vhost"
rabbitmqctl set_permissions -p "$vhost" "$vhost" ".*" ".*" ".*"
