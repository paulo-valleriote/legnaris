#!/usr/bin/env bash
#
# Diagnose and repair container egress in the Docker-in-Docker dev container.
#
# Why this exists: when the Cloudflare Tunnel hostnames (app./api./ops.) stop
# answering in the browser while the stack itself is healthy, the cause here is
# usually the host, not the compose files. A daemon restart can switch the
# iptables backend (nft <-> legacy) and leave the abandoned backend's rules in
# the kernel: that stale chain keeps `FORWARD policy DROP` and only knows the
# default bridge, so every packet leaving a user-defined bridge
# (legnaris-prod_edge / legnaris-prod_internal) is dropped. The Docker embedded
# resolver (127.0.0.11) still answers container names but forwards external
# lookups as the container IP, so cloudflared never completes edge discovery
# (`lookup _v2-origintunneld._tcp.argotunnel.com ... i/o timeout`), the API and
# web client lose their origin, and `app` crash-loops on its Mongo lookup.
#
# The check is functional on purpose: it probes real egress from a running
# container, because the backend mix is invisible from the Docker CLI.
#
# Usage (root is required: the legacy tables and the per-backend scanners are
# both privileged):
#   sudo scripts/dind_network.sh check
#   sudo scripts/dind_network.sh fix
#
# Exit codes: 0 healthy / nothing to repair, 1 broken (check) or repair failed
# (fix), 2 usage or precondition error.

set -euo pipefail

# Both commands run against the local Cloudflare Tunnel stack, so they resolve
# the repository root from this file instead of the caller's directory.
readonly REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly PROBE_HOST="example.com"
readonly BACKUP_FILE="${TMPDIR:-/tmp}/legnaris-iptables-legacy-backup.v4"

cd "$REPO_ROOT"

compose() {
    docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.local.yml "$@"
}

# resolve_bin prints the first existing path among the candidates. The backends
# ship one binary each, and those names — not the `iptables` alternative, which
# may point at either — are what tells them apart.
resolve_bin() {
    local candidate
    for candidate in "$@"; do
        if [ -x "$candidate" ]; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

# legacy_tables prints the legacy (xtables) tables currently loaded. Reading
# /proc/net/ip_tables_names never modprobes ip_tables, unlike `iptables-legacy -L`.
legacy_tables() {
    local names
    [ -r /proc/net/ip_tables_names ] || return 0
    names="$(< /proc/net/ip_tables_names)"
    printf '%s' "$names"
}

# has_docker_chains reports whether an xtables backend holds Docker's chains.
has_docker_chains() {
    local rules
    rules="$("$1" -S 2>/dev/null || true)"
    [[ "$rules" == *"-N DOCKER"* ]]
}

# live_backend prints which backend holds Docker's active chains.
live_backend() {
    if has_docker_chains "$NFT_BIN"; then
        echo "nft"
    elif [ -n "$(legacy_tables)" ] && has_docker_chains "$LEGACY_BIN"; then
        echo "legacy"
    else
        echo "none"
    fi
}

# instance_of prints the container id of a compose service, or nothing when the
# stack is down.
instance_of() {
    compose ps -q "$1" 2>/dev/null | head -1 || true
}

# egress_ok asks the embedded resolver for an external name from inside a
# container; a resolved `Name:` line proves the whole egress path (embedded DNS
# -> upstream -> FORWARD -> MASQUERADE) works.
egress_ok() {
    local out
    out="$(docker exec "$1" nslookup "$PROBE_HOST" 2>&1 || true)"
    [[ "$out" == *$'\nName:'* || "$out" == Name:* ]]
}

require_stack() {
    local container
    # traefik is attached to both bridges and ships with busybox, so the probe
    # needs no extra image; it is also always up when the stack is.
    container="$(instance_of traefik)"
    if [ -z "$container" ]; then
        echo "the stack is not running: start it with 'make deploy-local' first" >&2
        exit 2
    fi
    echo "$container"
}

require_root() {
    if [ "$(id -u)" -ne 0 ]; then
        echo "this command needs root: run 'sudo make net-${1}'" >&2
        exit 2
    fi
}

print_backends() {
    local tables
    echo "iptables alternative ..... $(readlink -f "$(command -v iptables || echo /usr/sbin/iptables)")"
    echo "docker's live backend .... $(live_backend)"
    tables="$(legacy_tables | tr '\n' ' ')"
    echo "legacy tables loaded ..... ${tables:-<none>}"
    if [ -n "$tables" ]; then
        echo "legacy FORWARD chain ..... $("$LEGACY_BIN" -L FORWARD -n 2>/dev/null | head -1 || true)"
    fi
}

# tunnel_hint reports, informationally, when cloudflared last registered an edge
# connection: it is the user-visible symptom, and it recovers on its own once
# egress is back (the container restarts until the tunnel comes up).
tunnel_hint() {
    local container line
    container="$(instance_of cloudflared)"
    if [ -z "$container" ]; then
        echo "cloudflared .............. not running"
        return 0
    fi
    line="$(docker logs --tail 500 "$container" 2>&1 | grep 'Registered tunnel connection' | tail -1 || true)"
    if [ -n "$line" ]; then
        echo "cloudflared .............. edge connection registered at ${line%% *}"
    else
        echo "cloudflared .............. no edge connection registered recently"
    fi
}

cmd_check() {
    local container
    require_root check
    container="$(require_stack)"

    echo "== host iptables =="
    print_backends
    echo
    echo "== container egress (probe from traefik) =="
    docker exec "$container" nslookup "$PROBE_HOST" 2>&1 || true
    echo
    if egress_ok "$container"; then
        tunnel_hint
        echo "OK: containers reach external DNS and the internet."
        exit 0
    fi
    echo "FAIL: containers cannot resolve external names." >&2
    echo "That is what keeps the tunnel down; repair it with: sudo make net-fix" >&2
    exit 1
}

cmd_fix() {
    local container backend table
    require_root fix
    container="$(require_stack)"

    if [ -z "$(legacy_tables)" ]; then
        echo "no legacy tables are loaded: nothing to repair."
        egress_ok "$container" && echo "OK: container egress already works." && exit 0
        echo "FAIL: container egress is broken and the legacy backend is not the cause." >&2
        exit 1
    fi

    backend="$(live_backend)"
    if [ "$backend" = "legacy" ]; then
        # Flushing here would delete Docker's own live rules.
        echo "docker's live rules already live in the legacy backend, so those tables are not stale."
        echo "Refusing to touch them: the cause is elsewhere." >&2
        exit 1
    fi
    if ! has_docker_chains "$LEGACY_BIN"; then
        echo "the legacy tables hold no Docker chains: nothing to repair."
        egress_ok "$container" && echo "OK: container egress already works." && exit 0
        echo "FAIL: container egress is broken and the legacy backend is not the cause." >&2
        exit 1
    fi

    echo "backing up the stale legacy rules to $BACKUP_FILE"
    "$LEGACY_SAVE" > "$BACKUP_FILE"
    "$LEGACY_SAVE" -t nat >> "$BACKUP_FILE"

    echo "removing the stale legacy rules (docker's live rules are in the ${backend} backend)"
    for table in filter nat mangle; do
        "$LEGACY_BIN" -F -t "$table"
        "$LEGACY_BIN" -X -t "$table"
    done
    "$LEGACY_BIN" -P FORWARD ACCEPT

    echo
    if egress_ok "$container"; then
        echo "OK: container egress restored."
        tunnel_hint
        echo "cloudflared and app recover on their own; follow along with 'docker compose ps'."
        exit 0
    fi
    echo "FAIL: container egress is still broken after the repair;" >&2
    echo "restore the previous rules with: $LEGACY_RESTORE < $BACKUP_FILE" >&2
    exit 1
}

NFT_BIN="$(resolve_bin /usr/sbin/iptables-nft /sbin/iptables-nft /usr/bin/iptables-nft)" || NFT_BIN=""
LEGACY_BIN="$(resolve_bin /usr/sbin/iptables-legacy /sbin/iptables-legacy /usr/bin/iptables-legacy)" || LEGACY_BIN=""
LEGACY_SAVE="$(resolve_bin /usr/sbin/iptables-legacy-save /sbin/iptables-legacy-save /usr/bin/iptables-legacy-save)" || LEGACY_SAVE=""
LEGACY_RESTORE="$(resolve_bin /usr/sbin/iptables-legacy-restore /sbin/iptables-legacy-restore /usr/bin/iptables-legacy-restore)" || LEGACY_RESTORE="iptables-legacy-restore"

if [ -z "$NFT_BIN" ] || [ -z "$LEGACY_BIN" ] || [ -z "$LEGACY_SAVE" ]; then
    echo "this host has no xtables backends to compare (expected on the dev container, never in production)" >&2
    exit 2
fi

case "${1:-}" in
    check) cmd_check ;;
    fix) cmd_fix ;;
    *) echo "usage: $0 {check|fix}" >&2; exit 2 ;;
esac
