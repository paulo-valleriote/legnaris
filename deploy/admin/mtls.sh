#!/usr/bin/env bash
# Client certificates for the admin host (LGN-262). Traefik verifies them
# against the committed public CA in deploy/traefik/admin-client-ca.pem; the CA
# key never leaves ADMIN_CA_DIR on the operator's machine.
#
#   mtls.sh init-ca <ca-pem-out>          create the CA once
#   mtls.sh issue <name> [days]           issue a client cert (default 90 days)
#   mtls.sh revoke <name> <ca-pem-out>    rotate the CA and reissue the others
#   mtls.sh list                          show active and revoked clients
#
# Traefik has no CRL support, so revoking rotates the CA: commit and deploy the
# new <ca-pem-out>, then hand the reissued bundles to the remaining operators.
set -euo pipefail

DIR="${ADMIN_CA_DIR:-$HOME/.config/legnaris/admin-ca}"
CA_DAYS=1825
MAX_DAYS=365

die() { echo "$1" >&2; exit "${2:-1}"; }

valid_name() { [[ "$1" =~ ^[a-z][a-z0-9-]{0,31}$ ]]; }

new_ca() {
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
    -keyout "$DIR/ca.key" -out "$DIR/ca.pem" -days "$CA_DAYS" \
    -subj "/CN=legnaris-admin-ca-$(date -u +%Y%m%d%H%M%S)" \
    -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
    -addext "keyUsage=critical,keyCertSign,cRLSign" 2>/dev/null
  chmod 600 "$DIR/ca.key"
}

sign() {
  local client="$1" days="$2"
  local ext
  ext="$(mktemp)"
  printf 'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\n' >"$ext"
  openssl x509 -req -in "$client/csr.pem" -CA "$DIR/ca.pem" -CAkey "$DIR/ca.key" \
    -CAcreateserial -out "$client/cert.pem" -days "$days" -extfile "$ext" 2>/dev/null
  rm -f "$ext"
  printf '%s\n' "$days" >"$client/days"
}

publish_ca() {
  install -m 644 "$DIR/ca.pem" "$1"
  echo "CA certificate written to $1; commit it and merge to deploy"
}

require_ca() { [[ -f "$DIR/ca.key" && -f "$DIR/ca.pem" ]] || die "no CA in $DIR; run init-ca first" 65; }

cmd_init_ca() {
  local out="${1:-}"
  [[ -n "$out" ]] || die "usage: mtls.sh init-ca <ca-pem-out>" 64
  [[ ! -e "$DIR/ca.key" ]] || die "a CA already exists in $DIR" 73
  install -m 700 -d "$DIR" "$DIR/clients" "$DIR/revoked"
  new_ca
  publish_ca "$out"
}

cmd_issue() {
  local name="${1:-}" days="${2:-90}"
  valid_name "$name" || die "name must match ^[a-z][a-z0-9-]{0,31}\$" 64
  if [[ ! "$days" =~ ^[1-9][0-9]*$ ]] || (( days > MAX_DAYS )); then
    die "days must be 1..$MAX_DAYS" 64
  fi
  require_ca
  local client="$DIR/clients/$name"
  [[ ! -e "$client" ]] || die "client $name already exists" 73
  install -m 700 -d "$client"
  openssl req -new -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
    -keyout "$client/key.pem" -out "$client/csr.pem" -subj "/CN=$name" 2>/dev/null
  chmod 600 "$client/key.pem"
  sign "$client" "$days"
  echo "issued $client/cert.pem and $client/key.pem"
}

cmd_revoke() {
  local name="${1:-}" out="${2:-}"
  if ! valid_name "$name" || [[ -z "$out" ]]; then
    die "usage: mtls.sh revoke <name> <ca-pem-out>" 64
  fi
  require_ca
  [[ -d "$DIR/clients/$name" ]] || die "no active client $name" 66
  rm -rf "$DIR/revoked/$name"
  mv "$DIR/clients/$name" "$DIR/revoked/$name"
  new_ca
  local client
  for client in "$DIR"/clients/*/; do
    [[ -d "$client" ]] || continue
    sign "${client%/}" "$(cat "${client%/}/days")"
    echo "reissued ${client%/}/cert.pem"
  done
  publish_ca "$out"
}

cmd_list() {
  local state client
  for state in clients revoked; do
    for client in "$DIR/$state"/*/; do
      [[ -d "$client" ]] && echo "$state $(basename "$client")"
    done
  done
  return 0
}

case "${1:-}" in
  init-ca) cmd_init_ca "${@:2}" ;;
  issue) cmd_issue "${@:2}" ;;
  revoke) cmd_revoke "${@:2}" ;;
  list) cmd_list ;;
  *) die "usage: mtls.sh init-ca|issue|revoke|list" 64 ;;
esac
