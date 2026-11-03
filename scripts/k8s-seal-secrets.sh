#!/usr/bin/env bash
# Seals the production secrets so they can live in Git. Reads plain env files
# (gitignored, never committed) and writes SealedSecret manifests next to the
# overlay kustomizations, then lists them under `resources`.
#
#   deploy/k8s/overlays/production/secrets.env      -> legnaris-secrets   (default)
#   deploy/k8s/overlays/production/backup-s3.env    -> backup-s3          (default)
#   deploy/k8s/overlays/production-ops/oauth2-proxy.env -> oauth2-proxy   (monitoring)
#   deploy/k8s/overlays/production-ops/grafana-admin.env -> grafana-admin (monitoring)
#
# Needs kubectl pointed at the production cluster and `kubeseal` installed.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
prod="$root/deploy/k8s/overlays/production"
ops="$root/deploy/k8s/overlays/production-ops"

seal() {
  local env_file="$1" name="$2" namespace="$3" out="$4" dir
  [ -f "$env_file" ] || { echo "missing $env_file" >&2; exit 1; }
  dir="$(dirname "$out")"
  kubectl create secret generic "$name" --namespace "$namespace" \
    --from-env-file="$env_file" --dry-run=client -o yaml \
    | kubeseal --format yaml --controller-namespace kube-system > "$out"
  local base
  base="$(basename "$out")"
  grep -qx "  - $base" "$dir/kustomization.yaml" || sed -i "/^resources:/a\\  - $base" "$dir/kustomization.yaml"
  echo "sealed $name -> $out"
}

seal "$prod/secrets.env" legnaris-secrets default "$prod/sealed-secrets.yaml"
seal "$prod/backup-s3.env" backup-s3 default "$prod/sealed-backup-s3.yaml"
seal "$ops/oauth2-proxy.env" oauth2-proxy monitoring "$ops/sealed-oauth2-proxy.yaml"
seal "$ops/grafana-admin.env" grafana-admin monitoring "$ops/sealed-grafana-admin.yaml"
