#!/usr/bin/env bash
#
# Captures the real backend contract into pinned fixtures (API-35).
#
# Run against the local stack (see README.md next to this script). Every file is
# the server's own response body, or the request that produced it —
# byte-faithful and never hand-edited: a shape change here is a backend contract
# change and the suite is expected to go red on it (API-36). No credential is
# ever written: the throwaway access token stays in a temporary directory and a
# credential request body is never recorded.
#
# The stack answers either a session or a code step, and the script follows the
# answer it actually received: the session family on a stack whose gate is off,
# the code-step family on a stack whose gate is on. The other family's files are
# left untouched, so one run per gate state refreshes the whole set (R8).

set -euo pipefail

BASE_URL="${API_BASE_URL:-http://localhost:8080}"
OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"

# The database the stack writes its send intentions to; the code is read from
# there, never from a log line (EML-10, EML-13).
DATABASE_NAME="${DATABASE_NAME:-legnaris}"

# The capture removes the send intentions it recorded, so the day's allowance is
# restored and the next run starts from the same budget.
cleanup() {
  rm -rf "$TMP_DIR"

  if [ -n "${DATABASE_URI:-}" ]; then
    CAPTURE_DATABASE="$DATABASE_NAME" mongosh --quiet "$DATABASE_URI" --eval '
      db.getSiblingDB(process.env.CAPTURE_DATABASE).email_outbox.deleteMany({ to: /^capture-/ });
    ' >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

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

# The value a capture carries in place of a credential: the shape is pinned,
# the secret is not.
REDACTED_TOKEN='redacted-access-token'

# write_response_redacted <file> <operation> <request-json> <status> <body-file>
#
# The verification answer carries an access token. The capture keeps the
# response's own shape and drops the credential, so no token is ever written to
# the repository (API-35).
write_response_redacted() {
  jq --arg placeholder "$REDACTED_TOKEN" '.data.access_token = $placeholder' "$5" \
    >"$TMP_DIR/redacted.out"
  write_response "$1" "$2" "$3" "$4" "$TMP_DIR/redacted.out"
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

# outbox_code <email> <purpose> -> prints the code the stack recorded for that send
#
# The code never reaches a log line or the sender's output, so the durable send
# intention is the only place it can be read (EML-10, EML-13).
outbox_code() {
  local email="$1" purpose="$2"

  CAPTURE_EMAIL="$email" CAPTURE_PURPOSE="$purpose" CAPTURE_DATABASE="$DATABASE_NAME" \
    mongosh --quiet "$DATABASE_URI" --eval '
      const outbox = db.getSiblingDB(process.env.CAPTURE_DATABASE).email_outbox;
      const row = outbox
        .find({ to: process.env.CAPTURE_EMAIL, "payload.purpose": process.env.CAPTURE_PURPOSE })
        .sort({ created_at: -1 })
        .limit(1)
        .next();
      if (row) {
        print(row.payload.code);
      }
    '
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
case "$login_status" in
  200 | 202) ;;
  *)
    echo "login failed ($login_status): $(cat "$TMP_DIR/login.out")" >&2
    exit 1
    ;;
esac

# The answer decides the family: registration answers the same status in both
# gate states, so the payload's own shape is what tells them apart (R8).
if jq -e '.data.challenge_id' "$TMP_DIR/register.out" >/dev/null 2>&1; then
  GATE='code-step'
else
  GATE='session'
fi

# --- the code-step family ----------------------------------------------------
capture_code_step() {
  local challenge_id resend_body resend_status verify_body verify_status
  local wrong_body wrong_status code wrong_code conflict_status index
  local cap_body cap_status saved_token

  challenge_id="$(jq -r '.data.challenge_id' "$TMP_DIR/login.out")"
  [ -n "$challenge_id" ] && [ "$challenge_id" != "null" ] || {
    echo 'the sign-in answer carried no challenge' >&2
    exit 1
  }

  # The credential answers: the request body is never recorded, because it
  # carries the password.
  write_response auth_register_challenge register_challenge \
    '{"method":"POST","path":"/auth/register"}' \
    "$register_status" "$TMP_DIR/register.out"
  write_response auth_login_challenge login_challenge \
    '{"method":"POST","path":"/auth/login"}' \
    "$login_status" "$TMP_DIR/login.out"

  # The resend answer: the same body whether a send was recorded or suppressed.
  resend_body="$(jq -nc --arg challenge_id "$challenge_id" '{challenge_id: $challenge_id}')"
  resend_status="$(send POST /auth/challenge/resend "$resend_body" "$TMP_DIR/challenge_resend.out")"
  write_response auth_challenge_resend challenge_resend \
    '{"method":"POST","path":"/auth/challenge/resend"}' \
    "$resend_status" "$TMP_DIR/challenge_resend.out"

  # The refused code: the same uniform refusal for every wrong code.
  code="$(outbox_code "$EMAIL" login_otp)"
  [ -n "$code" ] || {
    echo 'the stack recorded no code for the sign-in' >&2
    exit 1
  }
  wrong_code='000000'
  [ "$code" != "$wrong_code" ] || wrong_code='111111'
  wrong_body="$(jq -nc --arg challenge_id "$challenge_id" --arg code "$wrong_code" '{challenge_id: $challenge_id, code: $code}')"
  wrong_status="$(send POST /auth/challenge/verify "$wrong_body" "$TMP_DIR/otp_invalid.out")"
  write_response error_otp_invalid otp_invalid \
    '{"method":"POST","path":"/auth/challenge/verify"}' \
    "$wrong_status" "$TMP_DIR/otp_invalid.out"

  # The verification answer: the session the code step was waiting for.
  verify_body="$(jq -nc --arg challenge_id "$challenge_id" --arg code "$code" '{challenge_id: $challenge_id, code: $code}')"
  verify_status="$(send POST /auth/challenge/verify "$verify_body" "$TMP_DIR/challenge_verify.out")"
  write_response_redacted auth_challenge_verify challenge_verify \
    '{"method":"POST","path":"/auth/challenge/verify"}' \
    "$verify_status" "$TMP_DIR/challenge_verify.out"
  TOKEN="$(jq -r '.data.access_token' "$TMP_DIR/challenge_verify.out")"
  [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || {
    echo 'the verification returned no access token' >&2
    exit 1
  }

  # The taken username: the conflict the credential form must name.
  saved_token="$TOKEN"
  TOKEN=""
  conflict_status="$(send POST /auth/register "$register_body" "$TMP_DIR/conflict.out")"
  TOKEN="$saved_token"
  write_response error_conflict conflict \
    '{"method":"POST","path":"/auth/register"}' \
    "$conflict_status" "$TMP_DIR/conflict.out"

  # The unavailable send: the day's allowance is spent, so the next send is
  # refused with the typed outcome.
  saved_token="$TOKEN"
  TOKEN=""
  cap_status=''
  index=0
  while [ "$index" -lt 200 ]; do
    index=$((index + 1))
    cap_body="$(jq -nc \
      --arg email "capture-cap-${SUFFIX}-${index}@example.test" \
      --arg username "capturecap${SUFFIX}${index}" \
      --arg password "$PASSWORD" \
      '{email: $email, username: $username, password: $password}')"
    cap_status="$(send POST /auth/register "$cap_body" "$TMP_DIR/email_unavailable.out")"
    case "$cap_status" in
      503) break ;;
      429) sleep "${CAPTURE_RATE_LIMIT_WAIT:-61}" ;;
      201 | 409) ;;
      *)
        echo "the send attempt was refused ($cap_status): $(cat "$TMP_DIR/email_unavailable.out")" >&2
        exit 1
        ;;
    esac
  done
  TOKEN="$saved_token"
  [ "$cap_status" = "503" ] || {
    echo "the daily send cap was not reached (last status $cap_status)" >&2
    exit 1
  }
  write_response error_email_unavailable email_unavailable \
    '{"method":"POST","path":"/auth/register"}' \
    "$cap_status" "$TMP_DIR/email_unavailable.out"
}

if [ "$GATE" = "code-step" ]; then
  capture_code_step
else
  TOKEN="$(jq -r '.data.access_token' "$TMP_DIR/login.out")"
  [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || {
    echo 'login returned no access token' >&2
    exit 1
  }
fi

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

echo "captured $(find "$OUT_DIR" -maxdepth 1 -name '*.json' | wc -l) fixtures into $OUT_DIR (gate: $GATE)"
