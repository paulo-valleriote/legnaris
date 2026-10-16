#!/usr/bin/env python3
"""Lint rule 1 (promoted from the AI review on PR #58): no vacuous assertions.

    expect(true, isTrue)             # compares a constant against a matcher
    expect(0, greaterThan(-1))       # can never fail
    expect('same', equals('same'))   # observes nothing about the code under test

An assertion whose first argument is a compile-time literal is reported: it
passes or fails regardless of the behavior it claims to verify. The scan masks
comments and string literals first, so a commented-out copy or prose inside a
string never triggers, and an interpolated string (dynamic content) is not
treated as a literal.

Usage:
    python3 scripts/check_test_assertions.py [path ...]   # default: desktop/ web/

Exit codes: 0 clean, 1 violations, 2 usage error.
"""

import re
import sys
from pathlib import Path

MARKER_LIT = "@LIT@"      # plain string literal: a constant, so vacuous
MARKER_DYN = "@DYN@"      # interpolated string: has dynamic content, not vacuous

# Every Dart source is scanned; for the TypeScript client only test files carry
# assertions, so the test suffixes are matched instead of every `*.ts`/`*.tsx`.
DART_SUFFIX = ".dart"
CLIENT_TEST_SUFFIXES = (".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx")
SKIP_DIRS = {
    ".git",
    ".dart_tool",
    "build",
    "node_modules",
    ".idea",
    "dist",
    "coverage",
    ".vite",
    "playwright-report",
    "test-results",
}

# First argument of expect() that makes the assertion vacuous, after
# whitespace normalization of the extracted argument.
VACUOUS_ARG = re.compile(
    r"^(?:"
    r"true|false|null"  # bool/null literals
    r"|" + re.escape(MARKER_LIT) + r""  # plain string literal
    r"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?"  # decimal literal
    r"|0x[0-9a-fA-F]+"  # hex literal
    r")$"
)

CALL = re.compile(r"\b(?:expect|expectLater)\s*\(")


def mask(source):
    """Blank comments and replace string literals with markers.

    Newlines are preserved exactly, so line numbers computed on the masked
    text match the original file.
    """
    out = []
    # Stack of frames: ["code", depth] for code (depth None = top level,
    # depth >= 1 = inside ${...}), ["str", quote, raw, interpolated].
    stack = [["code", None]]

    def blank(text):
        out.append("".join(ch if ch == "\n" else " " for ch in text))

    i, n = 0, len(source)
    while i < n:
        frame = stack[-1]
        if frame[0] == "str":
            quote, raw = frame[1], frame[2]
            c = source[i]
            if not raw and c == "\\":
                # Escape: the next character is literal; newlines inside the
                # region are counted when the string closes.
                i += 2
                continue
            if source.startswith(quote, i):
                i += len(quote)
                region = source[frame[4]:i]
                marker = MARKER_DYN if frame[3] else MARKER_LIT
                out.append(marker)
                out.append("\n" * region.count("\n"))
                stack.pop()
                continue
            if not raw and c == "$" and i + 1 < n and (
                source[i + 1] == "{" or source[i + 1].isalpha() or source[i + 1] == "_"
            ):
                frame[3] = True
                if source[i + 1] == "{":
                    stack.append(["code", 1])
                    i += 2
                    continue
                # $identifier: skip the identifier, the string continues
                i += 1
                while i < n and (source[i].isalnum() or source[i] in "_$"):
                    i += 1
                continue
            i += 1
            continue

        # frame is code
        depth = frame[1]
        c = source[i]
        if c == "/" and i + 1 < n and source[i + 1] == "/":
            j = source.find("\n", i)
            j = n if j == -1 else j
            blank(source[i:j])
            i = j
            continue
        if c == "/" and i + 1 < n and source[i + 1] == "*":
            end = source.find("*/", i + 2)
            j = n if end == -1 else end + 2
            blank(source[i:j])
            i = j
            continue
        raw = False
        start = i
        if c in "rR" and i + 1 < n and source[i + 1] in "'\"":
            raw = True
            i += 1
            c = source[i]
        if c in "'\"":
            quote = c * 3 if source.startswith(c * 3, i) else c
            stack.append(["str", quote, raw, False, start])
            i += len(quote)
            continue
        if depth is not None:
            if c == "{":
                frame[1] = depth + 1
            elif c == "}":
                if depth == 1:
                    stack.pop()  # end of ${...}: back to the enclosing string
                    i += 1
                    continue
                frame[1] = depth - 1
        out.append(c)
        i += 1
    return "".join(out)


def _first_arg(masked, open_pos):
    """Return the first argument of the call whose '(' is at open_pos."""
    depth, i = 1, open_pos + 1
    start = i
    while i < len(masked):
        c = masked[i]
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
            if depth == 0:
                return masked[start:i]
        elif c == "," and depth == 1:
            return masked[start:i]
        i += 1
    return masked[start:]


def find_violations(source):
    """Return [(line, text)] for every vacuous assertion in source."""
    masked = mask(source)
    violations = []
    for m in CALL.finditer(masked):
        arg = re.sub(r"\s+", " ", _first_arg(masked, m.end() - 1)).strip()
        if VACUOUS_ARG.fullmatch(arg):
            line = masked.count("\n", 0, m.start()) + 1
            violations.append((line, arg.replace(MARKER_LIT, "'<string literal>'")))
    return violations


def source_files(paths):
    """Dart sources plus client test files — the files each toolchain asserts in."""
    for raw in paths:
        p = Path(raw)
        candidates = sorted(p.rglob("*")) if p.is_dir() else [p]
        for candidate in candidates:
            if not candidate.is_file():
                continue
            if any(part in SKIP_DIRS for part in candidate.parts):
                continue
            if candidate.suffix == DART_SUFFIX or candidate.name.endswith(CLIENT_TEST_SUFFIXES):
                yield candidate


def main(argv=None):
    argv = sys.argv[1:] if argv is None else argv
    roots = argv or ["desktop", "web"]
    found = 0
    for path in source_files(roots):
        try:
            text = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        for line, arg in find_violations(text):
            print(f"{path}:{line}: vacuous assertion, constant first argument: expect({arg}, ...)")
            found += 1
    if found:
        print(f"\n{found} vacuous assertion(s); assert on observed behavior, not on constants", file=sys.stderr)
        return 1
    print(f"check_test_assertions: clean ({', '.join(roots)})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
