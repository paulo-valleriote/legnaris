#!/usr/bin/env python3
"""GitHub PR review helper for the pr-review-resolver skill.

Subcommands:
  fetch <pr-url> [--author LOGIN]
      Print JSON with PR metadata plus the latest review comment authored by
      the given login (default: github-actions[bot]). A comment only counts
      as a review if it contains at least one markdown heading line ("## ...").

  post <pr-url> --body-file FILE
      Post a comment on the PR and print its id/url as JSON.

Authentication: the OPENCODE_PR token is read from the environment or from
the repository-root .env file. The token value is never printed.

Exit codes: 0 = success, 2 = usage/auth/API error (message on stderr).
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request
from typing import Any

API_ROOT = "https://api.github.com"
DEFAULT_AUTHOR = "github-actions[bot]"
PR_URL_RE = re.compile(r"^https://github\.com/([^/]+)/([^/]+)/pull/(\d+)")
HEADING_RE = re.compile(r"(?m)^##\s+\S")


def die(message: str) -> None:
    print(f"error: {message}", file=sys.stderr)
    sys.exit(2)


def repo_root() -> str:
    try:
        out = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True, text=True, check=True,
        )
    except (OSError, subprocess.CalledProcessError):
        die("not inside a git repository (git rev-parse --show-toplevel failed)")
    return out.stdout.strip()


def load_token() -> str:
    token = os.environ.get("OPENCODE_PR", "").strip()
    if token:
        return token
    env_path = os.path.join(repo_root(), ".env")
    try:
        with open(env_path, "r", encoding="utf-8") as handle:
            for line in handle:
                line = line.strip()
                if line.startswith("OPENCODE_PR="):
                    token = line.split("=", 1)[1].strip().strip('"').strip("'")
                    if token:
                        return token
    except OSError:
        pass
    die(
        "OPENCODE_PR token not found: set it in the environment or add "
        "OPENCODE_PR=<token> to the .env file at the repository root"
    )
    return ""  # unreachable


def parse_pr_url(url: str) -> tuple[str, str, int]:
    match = PR_URL_RE.match(url.strip())
    if not match:
        die(f"not a GitHub PR url: {url!r} (expected https://github.com/OWNER/REPO/pull/N)")
    owner, repo, number = match.group(1), match.group(2), int(match.group(3))
    return owner, repo, number


def api_request(method: str, path: str, token: str, payload: dict | None = None) -> Any:
    url = f"{API_ROOT}{path}"
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    request = urllib.request.Request(url, data=data, method=method)
    request.add_header("Authorization", f"Bearer {token}")
    request.add_header("Accept", "application/vnd.github+json")
    request.add_header("X-GitHub-Api-Version", "2022-11-28")
    request.add_header("User-Agent", "pr-review-resolver")
    if data is not None:
        request.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read().decode("utf-8")
    except urllib.error.HTTPError as exc:  # noqa: PERF203 - mapped to friendly errors
        detail = ""
        try:
            detail = json.loads(exc.read().decode("utf-8")).get("message", "")
        except Exception:
            pass
        hints = {
            401: "token is invalid or expired",
            403: "token lacks permission or rate limit exceeded",
            404: "not found (wrong url, private repo, or token lacks 'repo' scope)",
        }
        hint = hints.get(exc.code, "")
        die(f"GitHub API {method} {path} failed: HTTP {exc.code} {detail} {hint}".rstrip())
    except urllib.error.URLError as exc:
        die(f"network error calling GitHub API: {exc.reason}")
    return json.loads(body) if body else None


def fetch_issue_comments(owner: str, repo: str, number: int, token: str) -> list[dict]:
    comments: list[dict] = []
    page = 1
    while True:
        batch = api_request(
            "GET",
            f"/repos/{owner}/{repo}/issues/{number}/comments?per_page=100&page={page}",
            token,
        )
        comments.extend(batch)
        if len(batch) < 100:
            return comments
        page += 1


def cmd_fetch(args: argparse.Namespace) -> None:
    owner, repo, number = parse_pr_url(args.url)
    token = load_token()

    pr = api_request("GET", f"/repos/{owner}/{repo}/pulls/{number}", token)
    comments = fetch_issue_comments(owner, repo, number, token)

    bot_comments = [c for c in comments if (c.get("user") or {}).get("login") == args.author]
    reviews = [c for c in bot_comments if HEADING_RE.search(c.get("body") or "")]
    review = max(reviews, key=lambda c: c.get("created_at") or "") if reviews else None

    head = pr.get("head") or {}
    head_repo = head.get("repo") or {}
    result = {
        "owner": owner,
        "repo": repo,
        "pr_number": number,
        "pr_url": pr.get("html_url"),
        "pr_title": pr.get("title"),
        "pr_state": pr.get("state"),
        "base_ref": (pr.get("base") or {}).get("ref"),
        "head_ref": head.get("ref"),
        "head_sha": head.get("sha"),
        "head_repo": head_repo.get("full_name"),
        "head_is_fork": bool(head_repo.get("fork")),
        "author_searched": args.author,
        "bot_comment_count": len(bot_comments),
        "review_comment_count": len(reviews),
        "review": None if review is None else {
            "comment_id": review.get("id"),
            "comment_url": review.get("html_url"),
            "created_at": review.get("created_at"),
            "updated_at": review.get("updated_at"),
            "body": review.get("body") or "",
        },
    }
    print(json.dumps(result, indent=2, ensure_ascii=False))


def cmd_post(args: argparse.Namespace) -> None:
    owner, repo, number = parse_pr_url(args.url)
    token = load_token()
    try:
        with open(args.body_file, "r", encoding="utf-8") as handle:
            body = handle.read().strip()
    except OSError as exc:
        die(f"cannot read --body-file: {exc}")
    if not body:
        die("--body-file is empty; refusing to post an empty comment")
    comment = api_request(
        "POST", f"/repos/{owner}/{repo}/issues/{number}/comments", token, {"body": body}
    )
    print(json.dumps({"comment_id": comment.get("id"), "comment_url": comment.get("html_url")}, indent=2))


def main() -> None:
    parser = argparse.ArgumentParser(description="Fetch or reply to AI review comments on a GitHub PR")
    sub = parser.add_subparsers(dest="command", required=True)

    fetch_parser = sub.add_parser("fetch", help="fetch PR metadata and the latest AI review comment")
    fetch_parser.add_argument("url", help="PR url: https://github.com/OWNER/REPO/pull/N")
    fetch_parser.add_argument("--author", default=DEFAULT_AUTHOR, help=f"review author login (default: {DEFAULT_AUTHOR})")
    fetch_parser.set_defaults(func=cmd_fetch)

    post_parser = sub.add_parser("post", help="post a reply comment on the PR")
    post_parser.add_argument("url", help="PR url: https://github.com/OWNER/REPO/pull/N")
    post_parser.add_argument("--body-file", required=True, help="file with the markdown comment body")
    post_parser.set_defaults(func=cmd_post)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
