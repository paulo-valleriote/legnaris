#!/usr/bin/env bash
# bootstrap.sh [--dry-run] [--key-login-verified]
#
# Brings the already-running VPS to the hardened baseline. Each step reads the
# current state first and acts only when it differs, so a second run changes
# nothing. It never recreates the deploy user, the clone, volumes or containers.
#
# Required: DEPLOY_PUBLIC_KEY, one ssh-ed25519 or ssh-rsa public key line. It is
# installed behind the forced command in deploy/vps/ssh-entry.sh.
#
# Password and root SSH login are disabled only with --key-login-verified, which
# asserts that key login was proven from a second session. The daemon config is
# validated before reload; the operator's existing SSH session stays open while
# the reload happens, so a bad config can still be undone from it.
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

DRY_RUN=0
KEY_LOGIN_VERIFIED=0
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    --key-login-verified) KEY_LOGIN_VERIFIED=1 ;;
    *)
      echo "usage: bootstrap.sh [--dry-run] [--key-login-verified]" >&2
      exit 64
      ;;
  esac
done

APP_DIR="${LEGNARIS_APP_DIR:-/opt/legnaris}"
REPO_URL="${LEGNARIS_REPO_URL:-https://github.com/paulo-valleriote/legnaris.git}"
AUTH_KEYS="${LEGNARIS_AUTHORIZED_KEYS:-/home/deploy/.ssh/authorized_keys}"
# The drop-in is named 00- because sshd keeps the first value it reads for each
# keyword and reads the drop-ins in lexical order; 50-cloud-init.conf would win
# over a 99- file. The effective settings are verified with sshd -T anyway.
SSHD_DROPIN="${LEGNARIS_SSHD_DROPIN:-/etc/ssh/sshd_config.d/00-legnaris.conf}"
AUTO_UPGRADES="${LEGNARIS_AUTO_UPGRADES:-/etc/apt/apt.conf.d/20auto-upgrades}"
FIREWALL_PORTS=(22 80 443)

KEY_PATTERN='^ssh-(ed25519|rsa) [A-Za-z0-9+/=]+( [[:print:]]+)?$'
KEY="${DEPLOY_PUBLIC_KEY-}"
if [[ ! "$KEY" =~ $KEY_PATTERN ]]; then
  echo "DEPLOY_PUBLIC_KEY must be a single ssh public key line" >&2
  exit 64
fi

HARDENED_SSHD=$'PasswordAuthentication no\nPermitRootLogin no'
AUTO_UPGRADES_CONTENT=$'APT::Periodic::Update-Package-Lists "1";\nAPT::Periodic::Unattended-Upgrade "1";'

say() {
  echo "$*"
}

# act runs a mutating command, or only reports it in dry-run mode.
act() {
  if (( DRY_RUN )); then
    echo "[dry-run] $*"
  else
    "$@"
  fi
}

# write_file replaces a whole file with stdin, through a temporary file so a
# failed write never leaves a partial file behind.
write_file() {
  local path="$1" mode="$2" content tmp
  content="$(cat)"
  if (( DRY_RUN )); then
    echo "[dry-run] write $path (mode $mode)"
    return 0
  fi
  mkdir -p "$(dirname "$path")"
  tmp="$(mktemp "$(dirname "$path")/.bootstrap.XXXXXX")"
  printf '%s\n' "$content" >"$tmp"
  chmod "$mode" "$tmp"
  mv -f "$tmp" "$path"
}

# own_for_deploy hands a path to the deploy user. Bootstrap runs as root, so the
# chown is not gated on EUID; a non-root run fails loudly instead of skipping.
own_for_deploy() {
  act chown -R deploy:deploy "$1"
}

require_ufw() {
  if ! command -v ufw >/dev/null 2>&1; then
    echo "ufw is not installed; install it (apt-get install -y ufw) before bootstrap" >&2
    exit 1
  fi
}

ensure_deploy_user() {
  if id deploy >/dev/null 2>&1; then
    say "ok: user deploy exists"
  else
    act useradd --create-home --shell /bin/bash deploy
  fi
  if id -nG deploy 2>/dev/null | tr ' ' '\n' | grep -qx docker; then
    say "ok: deploy is in the docker group"
  else
    act usermod -aG docker deploy
  fi
}

ensure_authorized_key() {
  local restricted existing kept
  restricted="restrict,command=\"$APP_DIR/deploy/vps/ssh-entry.sh\" $KEY"
  existing="$(cat "$AUTH_KEYS" 2>/dev/null || true)"
  if grep -qxF "$restricted" <<<"$existing"; then
    say "ok: restricted CI key installed"
    return 0
  fi
  kept="$(grep -vF "$KEY" <<<"$existing" | sed '/^$/d' || true)"
  write_file "$AUTH_KEYS" 600 <<<"$(printf '%s\n%s' "$kept" "$restricted" | sed '/^$/d')"
  own_for_deploy "$(dirname "$AUTH_KEYS")"
}

ensure_firewall() {
  local status port
  status="$(ufw status 2>/dev/null || true)"
  for port in "${FIREWALL_PORTS[@]}"; do
    if grep -qE "^${port}/tcp[[:space:]]+ALLOW" <<<"$status"; then
      say "ok: ${port}/tcp allowed"
    else
      act ufw allow "${port}/tcp"
    fi
  done
  if grep -q '^Status: active' <<<"$status"; then
    say "ok: firewall active"
  else
    act ufw --force enable
  fi
}

# verify_effective_sshd asks sshd for the settings it will actually use, after
# every drop-in is merged. A file that sorts earlier can override the drop-in.
verify_effective_sshd() {
  local effective
  effective="$(sshd -T 2>/dev/null || true)"
  if grep -qx 'passwordauthentication no' <<<"$effective" &&
    grep -qx 'permitrootlogin no' <<<"$effective"; then
    return 0
  fi
  echo "effective sshd settings do not disable password and root login; another config file overrides the drop-in" >&2
  return 1
}

# restore_dropin puts back the previous drop-in content, or removes the file
# when there was none.
restore_dropin() {
  local previous="$1"
  if [[ -n "$previous" ]]; then
    printf '%s\n' "$previous" >"$SSHD_DROPIN"
  else
    rm -f "$SSHD_DROPIN"
  fi
}

# apply_sshd writes the drop-in, validates the whole daemon config, checks the
# effective settings and only then reloads. Any failed check restores the
# previous drop-in and fails the run.
apply_sshd() {
  local previous
  previous="$(cat "$SSHD_DROPIN" 2>/dev/null || true)"
  if (( DRY_RUN )); then
    echo "[dry-run] write $SSHD_DROPIN, validate with sshd -t and sshd -T, reload ssh"
    return 0
  fi
  mkdir -p "$(dirname "$SSHD_DROPIN")"
  printf '%s\n' "$HARDENED_SSHD" >"$SSHD_DROPIN"
  if ! sshd -t; then
    restore_dropin "$previous"
    echo "sshd config is invalid; the drop-in was reverted" >&2
    return 1
  fi
  if ! verify_effective_sshd; then
    restore_dropin "$previous"
    echo "the drop-in was reverted" >&2
    return 1
  fi
  systemctl reload ssh
}

ensure_ssh_hardening() {
  local current
  current="$(cat "$SSHD_DROPIN" 2>/dev/null || true)"
  if [[ "$current" == "$HARDENED_SSHD" ]]; then
    verify_effective_sshd || return 1
    say "ok: ssh hardening in place"
  elif (( KEY_LOGIN_VERIFIED )); then
    apply_sshd
  else
    say "skip: password login stays enabled until rerun with --key-login-verified"
  fi
}

ensure_unattended_upgrades() {
  if ! dpkg -s unattended-upgrades >/dev/null 2>&1; then
    act apt-get install -y -q unattended-upgrades
  fi
  if [[ "$(cat "$AUTO_UPGRADES" 2>/dev/null || true)" == "$AUTO_UPGRADES_CONTENT" ]]; then
    say "ok: automatic security updates enabled"
  else
    write_file "$AUTO_UPGRADES" 644 <<<"$AUTO_UPGRADES_CONTENT"
  fi
}

ensure_clone() {
  if [[ -d "$APP_DIR/.git" ]]; then
    say "ok: checkout exists at $APP_DIR"
  else
    act git clone "$REPO_URL" "$APP_DIR"
    own_for_deploy "$APP_DIR"
  fi
}

require_ufw
ensure_deploy_user
ensure_authorized_key
ensure_firewall
ensure_ssh_hardening
ensure_unattended_upgrades
ensure_clone

if (( DRY_RUN )); then
  say "dry run complete: nothing was changed"
else
  say "bootstrap complete"
fi
