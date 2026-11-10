#!/usr/bin/env bash
# Verifies that the app, web and rabbitmq images published for a commit exist in
# GHCR, so the VPS is never asked to pull an image the Images workflow did not
# build and scan. Packages are public, so no registry login is needed.
set -euo pipefail

SHA="${1:-}"
if [[ ! "$SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "usage: check-images.sh <40-hex sha>" >&2
  exit 64
fi

OWNER="${IMAGE_OWNER:-paulo-valleriote}"
missing=()
for name in app web rabbitmq; do
  image="ghcr.io/${OWNER}/legnaris-${name}:sha-${SHA}"
  docker manifest inspect "$image" >/dev/null 2>&1 || missing+=("$image")
done

if (( ${#missing[@]} > 0 )); then
  printf 'missing image: %s\n' "${missing[@]}" >&2
  exit 1
fi
echo "all images present for $SHA"
