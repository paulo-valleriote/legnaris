#!/usr/bin/env python3
"""Lint rule 3 (promoted from the AI review on PR #58): validation citations resolve.

Every `file:line` citation in `.specs/**/validation.md` must name a file that
exists and a line that file actually has. Closed features recorded evidence
that existed at the time, so the check is scoped to what a change is
responsible for:

  * a `validation.md` the change touches is checked in full (variant A) —
    this is what catches citations broken by the same change;
  * citations from every OTHER `validation.md` are checked when they point
    into a file the change touches (variant B), including attribution by
    basename when the change deleted that file.

`--all` checks every citation in every validation.md regardless of the diff.

Shorthand citations (`:12`) continue the most recently cited file in reading
order. Line ranges (`821-823`) and lists (`73,97`) are expanded; an en dash
is accepted as a range separator.

Usage:
    python3 scripts/check_validation_citations.py [--base REF] [--all] [--root DIR]

Default base is `origin/main`. Exit codes: 0 clean, 1 violations, 2 usage error.
"""

import argparse
import re
import subprocess
import sys
from pathlib import Path

SKIP_DIRS = {".git", ".dart_tool", "build", "node_modules", ".idea"}

SPAN = re.compile(r"`([^`\n]+)`")
FULL = re.compile(r"^(?P<file>[^\s`()]+?\.[A-Za-z0-9_]+):(?P<lines>[0-9][0-9, \-–]*)$")
SHORT = re.compile(r"^:(?P<lines>[0-9][0-9, \-–]*)$")
# A bare file mention (`exclusive_control_test.dart`) re-anchors the shorthand
# that follows it, even without a line claim of its own.
CODE_EXT = r"(?:dart|go|md|yml|yaml|json|py|sh|arb|mod|sum|txt|html|css|ts|js)"
BARE = re.compile(r"^(?P<file>(?:[^\s()`]+/)*[^\s()`]+\." + CODE_EXT + r")$")
RANGE = re.compile(r"^(\d+)-(\d+)$")


def build_index(root):
    """Map every basename in the repo to its relative paths."""
    index = {}
    for p in sorted(root.rglob("*")):
        if not p.is_file() or any(part in SKIP_DIRS for part in p.parts):
            continue
        index.setdefault(p.name, []).append(str(p.relative_to(root)))
    return index


def resolve(ref, doc_rel, root, index):
    """Resolve a citation reference to a repo-relative path."""
    if (root / ref).is_file():
        return ("ok", str(Path(ref)))
    doc_dir = str(Path(doc_rel).parent)
    local = str(Path(doc_dir) / ref)
    if (root / local).is_file():
        return ("ok", local)
    hits = index.get(Path(ref).name, [])
    same_dir = [h for h in hits if str(Path(h).parent) == doc_dir]
    if len(same_dir) == 1:
        return ("ok", same_dir[0])
    if len(hits) == 1:
        return ("ok", hits[0])
    if len(hits) > 1:
        return ("ambiguous", sorted(hits))
    return ("missing", None)


def expand_lines(spec):
    """Expand '821-823' / '73,97' / '10–12' into line numbers."""
    out = []
    for part in spec.split(","):
        part = part.strip().replace("–", "-")
        if not part:
            continue
        m = RANGE.match(part)
        if m:
            out.extend(range(int(m.group(1)), int(m.group(2)) + 1))
        elif part.isdigit():
            out.append(int(part))
    return out


def iter_citations(lines):
    """Yield (lineno, span, kind, ref, line_spec) in reading order.

    kind is "full" (`file.go:12`), "short" (`:12`) or "bare" (`file.go`).
    """
    for lineno, line in enumerate(lines, 1):
        for span in SPAN.findall(line):
            span = span.strip()
            m = FULL.match(span)
            if m:
                yield lineno, span, "full", m.group("file"), m.group("lines")
                continue
            m = SHORT.match(span)
            if m:
                yield lineno, span, "short", None, m.group("lines")
                continue
            m = BARE.match(span)
            if m:
                yield lineno, span, "bare", m.group("file"), None


def check_document(doc_rel, root, index, changed=None):
    """Check one validation.md. changed=None checks every citation; otherwise
    only citations this change owns (variants A/B described in the module
    docstring)."""
    full = changed is None or doc_rel in changed
    changed_basenames = {Path(c).name for c in changed} if changed else set()
    violations = []
    last_ref = None
    text = (root / doc_rel).read_text(encoding="utf-8")
    for lineno, span, kind, ref, line_spec in iter_citations(text.splitlines()):
        if kind == "bare":
            last_ref = ref
            continue
        if kind == "short":
            ref = last_ref
            if ref is None:
                if full:
                    violations.append((doc_rel, lineno, span, "shorthand without a preceding file citation"))
                continue
        else:
            last_ref = ref

        kind, target = resolve(ref, doc_rel, root, index)
        if kind == "ok":
            if not full and target not in changed:
                continue
            n = len((root / target).read_text(encoding="utf-8", errors="replace").splitlines())
            for ln in expand_lines(line_spec):
                if ln > n:
                    violations.append((doc_rel, lineno, span, f"line {ln} exceeds {target} ({n} lines)"))
        elif kind == "missing":
            if full or Path(ref).name in changed_basenames:
                violations.append((doc_rel, lineno, span, f"file not found: {ref}"))
        elif full:
            violations.append((doc_rel, lineno, span, "ambiguous basename: " + ", ".join(target)))
    return violations


def changed_files(root, base):
    """Files the current branch changes relative to merge-base(base, HEAD)."""
    try:
        proc = subprocess.run(
            ["git", "diff", "--name-only", f"{base}...HEAD"],
            cwd=root, capture_output=True, text=True, check=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError) as exc:
        detail = getattr(exc, "stderr", str(exc)).strip()
        print(f"check_validation_citations: cannot diff against '{base}': {detail}", file=sys.stderr)
        print("pass --base <ref> explicitly, or --all to check every citation", file=sys.stderr)
        return None
    return {line for line in proc.stdout.splitlines() if line}


def run(root, changed=None):
    root = Path(root)
    index = build_index(root)
    violations = []
    docs = sorted(root.glob(".specs/**/validation.md"))
    for doc in docs:
        doc_rel = str(doc.relative_to(root))
        violations.extend(check_document(doc_rel, root, index, changed))
    return violations


def main(argv=None):
    parser = argparse.ArgumentParser(prog="check_validation_citations")
    parser.add_argument("--base", default=None, help="diff base ref (default: origin/main)")
    parser.add_argument("--all", action="store_true", help="check every citation, not just the diff scope")
    parser.add_argument("--root", default=".", help="repository root")
    args = parser.parse_args(argv)

    changed = None
    if not args.all:
        changed = changed_files(Path(args.root).resolve(), args.base or "origin/main")
        if changed is None:
            return 2

    violations = run(Path(args.root).resolve(), changed)
    scope = "all citations" if changed is None else f"{len(changed)} changed file(s)"
    for doc, lineno, span, reason in violations:
        print(f"{doc}:{lineno}: `{span}` -> {reason}")
    if violations:
        print(f"\n{len(violations)} dead citation(s) in scope ({scope})", file=sys.stderr)
        return 1
    print(f"check_validation_citations: clean ({scope})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
