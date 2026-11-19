#!/bin/sh
set -eu

root="${CLAUDE_PROJECT_DIR:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)}"
cd "$root" || exit 1
exec go run ./backend/cmd/markdown-check -hook
