#!/bin/sh
# Publishes the browser-facing runtime settings as /config.json from the
# container environment: the API origin from API_BASE_URL and the AdSense client
# and slot ids. An unset or invalid value is omitted so the client keeps its own
# secure default; backslashes and quotes are escaped so an env value cannot break
# the JSON. A valid AdSense client also publishes /ads.txt; without one any stale
# file is removed so the route answers 404.
set -eu

target_dir="${RUNTIME_CONFIG_DIR:-/usr/share/nginx/html}"
config="$target_dir/config.json"
ads_txt="$target_dir/ads.txt"

is_adsense_client() {
  printf '%s' "$1" | grep -Eq '^ca-pub-[0-9]{10,20}$'
}

is_adsense_slot() {
  printf '%s' "$1" | grep -Eq '^[0-9]{1,20}$'
}

client="${ADSENSE_CLIENT:-}"
if ! is_adsense_client "$client"; then
  client=""
fi

slots=""
add_slot() {
  is_adsense_slot "$2" || return 0
  if [ -n "$slots" ]; then
    slots="$slots,"
  fi
  slots="$slots\"$1\":\"$2\""
}

document=""

api_value="${API_BASE_URL:-}"
if [ -n "$api_value" ]; then
  escaped=$(printf '%s' "$api_value" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\n\r')
  document="\"apiBaseUrl\":\"$escaped\""
fi

if [ -n "$client" ]; then
  add_slot catalog "${ADSENSE_SLOT_CATALOG:-}"
  add_slot campaigns "${ADSENSE_SLOT_CAMPAIGNS:-}"
  add_slot landing "${ADSENSE_SLOT_LANDING:-}"

  adsense="\"client\":\"$client\""
  if [ -n "$slots" ]; then
    adsense="$adsense,\"slots\":{$slots}"
  fi
  if [ -n "$document" ]; then
    document="$document,"
  fi
  document="$document\"adsense\":{$adsense}"

  printf 'google.com, pub-%s, DIRECT, f08c47fec0942fa0\n' "${client#ca-pub-}" > "$ads_txt"
else
  rm -f "$ads_txt"
fi

if [ -n "$document" ]; then
  printf '{%s}\n' "$document" > "$config"
else
  printf '{}\n' > "$config"
fi
