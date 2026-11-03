#!/bin/sh
# Publishes the browser-facing API origin as /config.json from API_BASE_URL.
# An unset or blank value yields {} so the client applies its secure default;
# backslashes and quotes are escaped so the env value cannot break the JSON.
set -eu

target=/usr/share/nginx/html/config.json
value="${API_BASE_URL:-}"

if [ -z "$value" ]; then
  printf '{}\n' > "$target"
  exit 0
fi

escaped=$(printf '%s' "$value" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\n\r')
printf '{"apiBaseUrl":"%s"}\n' "$escaped" > "$target"
