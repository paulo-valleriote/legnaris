#!/usr/bin/env bash
#
# Captures the real backend contract into pinned fixtures (API-35).
#
# Run against the local stack (see README.md next to this script). Every file is
# the server's own response body, or the request that produced it —
# byte-faithful and never hand-edited: a shape change here is a backend contract
# change and the suite is expected to go red on it (API-36). No credential is
# ever written: the throwaway access token stays in a temporary directory.

set -euo pipefail

BASE_URL="${API_BASE_URL:-http://localhost:8080}"
OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

STAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
PROVENANCE="captured from the local stack (make dev-db; go run ./backend/cmd/api.go) at ${BASE_URL} on ${STAMP}"
TOKEN=""

command -v jq >/dev/null 2>&1 || {
  echo 'jq is required to capture fixtures' >&2
  exit 1
}

# write_response <file> <operation> <request-json> <status> <body-file>
write_response() {
  jq -n \
    --arg operation "$2" \
    --arg provenance "$PROVENANCE" \
    --argjson request "$3" \
    --argjson status "$4" \
    --slurpfile response "$5" \
    '{operation: $operation, provenance: $provenance, request: $request, response: {status: $status, body: $response[0]}}' \
    >"$OUT_DIR/$1.json"
}

# write_request <file> <operation> <request-json>
write_request() {
  jq -n \
    --arg operation "$2" \
    --arg provenance "$PROVENANCE" \
    --argjson request "$3" \
    '{operation: $operation, provenance: $provenance, request: $request}' \
    >"$OUT_DIR/$1.json"
}

# send <method> <path> <body-or-empty> <body-file> -> prints the status
send() {
  local method="$1" path="$2" body="${3:-}" out="$4"
  local args=(-s -o "$out" -w '%{http_code}' -X "$method" "$BASE_URL$path")

  if [ -n "$body" ]; then
    args+=(-H 'content-type: application/json' --data "$body")
  fi

  if [ -n "$TOKEN" ]; then
    args+=(-H "Authorization: Bearer $TOKEN")
  fi

  curl "${args[@]}"
}

# --- a throwaway session -----------------------------------------------------
SUFFIX="${STAMP//[^0-9]/}"
EMAIL="capture-${SUFFIX}@example.test"
USERNAME="capture${SUFFIX}"
PASSWORD='capture-password-1'

register_body="$(jq -nc --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email: $email, username: $username, password: $password}')"
register_status="$(curl -s -o "$TMP_DIR/register.out" -w '%{http_code}' -X POST "$BASE_URL/auth/register" -H 'content-type: application/json' --data "$register_body")"
[ "$register_status" = "201" ] || {
  echo "register failed ($register_status): $(cat "$TMP_DIR/register.out")" >&2
  exit 1
}

login_body="$(jq -nc --arg email "$EMAIL" --arg password "$PASSWORD" '{email: $email, password: $password}')"
login_status="$(curl -s -o "$TMP_DIR/login.out" -w '%{http_code}' -X POST "$BASE_URL/auth/login" -H 'content-type: application/json' --data "$login_body")"
[ "$login_status" = "200" ] || {
  echo "login failed ($login_status): $(cat "$TMP_DIR/login.out")" >&2
  exit 1
}

TOKEN="$(jq -r '.data.access_token' "$TMP_DIR/login.out")"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || {
  echo 'login returned no access token' >&2
  exit 1
}

# --- success captures --------------------------------------------------------
health_request='{"method":"GET","path":"/health"}'
status="$(send GET /health '' "$TMP_DIR/health.out")"
write_response health health "$health_request" "$status" "$TMP_DIR/health.out"

templates_request='{"method":"GET","path":"/templates"}'
status="$(send GET /templates '' "$TMP_DIR/templates.out")"
write_response templates template_catalog "$templates_request" "$status" "$TMP_DIR/templates.out"

template_detail_request='{"method":"GET","path":"/templates/fantasy_v1"}'
status="$(send GET /templates/fantasy_v1 '' "$TMP_DIR/template_detail.out")"
write_response template_detail template_detail "$template_detail_request" "$status" "$TMP_DIR/template_detail.out"

create_body="$(jq -nc '{template_id: "fantasy_v1", sheet_name: "Capture Hero"}')"
create_request="$(jq -nc --argjson body "$create_body" '{method: "POST", path: "/sheets", body: $body}')"
write_request sheet_create_request sheet_create "$create_request"
status="$(send POST /sheets "$create_body" "$TMP_DIR/sheet_create.out")"
write_response sheet_create sheet_create "$create_request" "$status" "$TMP_DIR/sheet_create.out"

SHEET_ID="$(jq -r '.data.id' "$TMP_DIR/sheet_create.out")"
[ -n "$SHEET_ID" ] && [ "$SHEET_ID" != "null" ] || {
  echo 'sheet creation returned no id' >&2
  exit 1
}

sheet_get_request="$(jq -nc --arg path "/sheets/$SHEET_ID" '{method: "GET", path: $path}')"
status="$(send GET "/sheets/$SHEET_ID" '' "$TMP_DIR/sheet_get.out")"
write_response sheet_get sheet_retrieval "$sheet_get_request" "$status" "$TMP_DIR/sheet_get.out"

patch_body="$(jq -nc '{data: {stats: {dexterity: 14}}}')"
patch_request="$(jq -nc --arg path "/sheets/$SHEET_ID" --argjson body "$patch_body" '{method: "PATCH", path: $path, body: $body}')"
write_request patch_delta_request sheet_update "$patch_request"
status="$(send PATCH "/sheets/$SHEET_ID" "$patch_body" "$TMP_DIR/patch_delta_response.out")"
write_response patch_delta_response sheet_update "$patch_request" "$status" "$TMP_DIR/patch_delta_response.out"

# --- failure captures (while the sheet still exists) -------------------------
invalid_request_body="$(jq -nc '{data: {modifiers: {dexterityMod: 5}}}')"
invalid_request_path="/sheets/$SHEET_ID"
status="$(send PATCH "$invalid_request_path" "$invalid_request_body" "$TMP_DIR/error_invalid_request.out")"
write_response error_invalid_request invalid_edit \
  "$(jq -nc --arg path "$invalid_request_path" --argjson body "$invalid_request_body" '{method: "PATCH", path: $path, body: $body}')" \
  "$status" "$TMP_DIR/error_invalid_request.out"

unknown_field_body="$(jq -nc '{data: {stats: {nonexistentAbility: 3}}}')"
status="$(send PATCH "$invalid_request_path" "$unknown_field_body" "$TMP_DIR/error_unknown_field.out")"
write_response error_unknown_field unknown_field \
  "$(jq -nc --arg path "$invalid_request_path" --argjson body "$unknown_field_body" '{method: "PATCH", path: $path, body: $body}')" \
  "$status" "$TMP_DIR/error_unknown_field.out"

status="$(send POST /sheets '{"template_id": "fantasy_v1", ' "$TMP_DIR/error_invalid_json.out")"
write_response error_invalid_json invalid_json \
  '{"method":"POST","path":"/sheets"}' \
  "$status" "$TMP_DIR/error_invalid_json.out"

SAVED_TOKEN="$TOKEN"
TOKEN=""
status="$(send GET /templates '' "$TMP_DIR/error_unauthorized.out")"
write_response error_unauthorized unauthorized \
  '{"method":"GET","path":"/templates"}' \
  "$status" "$TMP_DIR/error_unauthorized.out"
TOKEN="$SAVED_TOKEN"

# --- a missing resource: delete the sheet, then retrieve it ------------------
send DELETE "/sheets/$SHEET_ID" '' "$TMP_DIR/delete.out" >/dev/null
status="$(send GET "/sheets/$SHEET_ID" '' "$TMP_DIR/error_not_found.out")"
write_response error_not_found not_found "$sheet_get_request" "$status" "$TMP_DIR/error_not_found.out"

# --- an internal fault: an unmatched route surfaces as INTERNAL_ERROR --------
status="$(send GET /no-such-route '' "$TMP_DIR/error_internal.out")"
write_response error_internal internal_error '{"method":"GET","path":"/no-such-route"}' "$status" "$TMP_DIR/error_internal.out"

echo "captured $(find "$OUT_DIR" -maxdepth 1 -name '*.json' | wc -l) fixtures into $OUT_DIR"
