#!/usr/bin/env python3
"""Lint rule 2 (promoted from the AI review on PR #58): cited requirement IDs exist.

Every requirement ID cited in code (`RND-21`, `PRV-13`, `DRB-01`, ...) must be
defined in the spec corpus, otherwise the citation cannot be traced back to a
requirement:

    AD-*   -> .specs/STATE.md        (architecture decision records)
    other  -> .specs/**/spec.md      (requirement definitions)
    LGN-*  -> skipped                (Jira card keys, not spec requirement IDs)

`ISO-8601`-style standard identifiers are skipped explicitly: they name a date
format, not a requirement. Both `*.dart` and `*.go` sources are scanned.

Usage:
    python3 scripts/check_requirement_ids.py [path ...]   # default: repo root

Exit codes: 0 clean, 1 violations, 2 usage error.
"""

import re
import sys
from pathlib import Path

ID = re.compile(r"\b([A-Z][A-Z0-9]*)-(\d+)\b")

SKIP_PREFIXES = {"LGN"}    # Jira card keys — tracked in Jira, not in spec.md
SKIP_IDS = {"ISO-8601"}    # date-format standard, not a requirement
CODE_SUFFIXES = {".dart", ".go"}
SKIP_DIRS = {".git", ".dart_tool", "build", "node_modules", ".idea"}


def collect_ids(paths, pattern=ID):
    """Every ID-shaped token appearing in the given markdown files."""
    ids = set()
    for p in paths:
        if not p.is_file():
            continue
        ids.update(m.group(0) for m in pattern.finditer(p.read_text(encoding="utf-8")))
    return ids


def find_violations(text, spec_ids, state_ids):
    """Return [(line, id, source)] for IDs cited in text that no spec defines."""
    violations = []
    for lineno, line in enumerate(text.splitlines(), 1):
        for m in ID.finditer(line):
            full, prefix = m.group(0), m.group(1)
            if full in SKIP_IDS or prefix in SKIP_PREFIXES:
                continue
            if prefix == "AD":
                if full not in state_ids:
                    violations.append((lineno, full, ".specs/STATE.md"))
            elif full not in spec_ids:
                violations.append((lineno, full, ".specs/**/spec.md"))
    return violations


def code_files(root, paths=None):
    roots = [Path(p) for p in paths] if paths else [root]
    for base in roots:
        candidates = sorted(base.rglob("*")) if base.is_dir() else [base]
        for p in candidates:
            if p.suffix not in CODE_SUFFIXES or not p.is_file():
                continue
            if any(part in SKIP_DIRS for part in p.parts):
                continue
            yield p


def main(argv=None):
    argv = sys.argv[1:] if argv is None else argv
    root = Path.cwd()
    spec_ids = collect_ids(sorted(root.glob(".specs/**/spec.md")))
    state_ids = collect_ids([root / ".specs" / "STATE.md"])
    if not spec_ids:
        print("check_requirement_ids: no .specs/**/spec.md found (run from the repo root)", file=sys.stderr)
        return 2

    found = 0
    for path in code_files(root, argv or None):
        try:
            text = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        for lineno, req_id, source in find_violations(text, spec_ids, state_ids):
            print(f"{path}:{lineno}: {req_id} is cited but not defined in {source}")
            found += 1
    if found:
        print(f"\n{found} dangling requirement ID(s); define the ID in the spec or fix the citation", file=sys.stderr)
        return 1
    print(f"check_requirement_ids: clean ({len(spec_ids)} spec IDs, {len(state_ids)} decision IDs)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
