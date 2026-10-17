# Quickstart: 014-web-polish-decommission (F10)

**Preconditions**: Feature 006 (`cookie auth`) delivered; feature 001 (`check_requirement_ids.py` TS) delivered; `web/` builds.

## Verify each POL gate locally

```bash
# POL-01 (parity) — requires Playwright + reference artifact
npm run test:web -- --grep="parity"

# POL-02 (CSP enforce)
curl -I -s https://app.legnaris.com.br | grep -i "Content-Security-Policy"
# Must include enforce (not only Report-Only), report-uri/report-to present.

# POL-03 (cookie removal / cookie presence)
# Login → inspect response headers for __Host-legnaris_refresh (HttpOnly, Secure, SameSite=Strict)
# Refresh endpoint → must read cookie, must NOT accept X-Refresh-Token

# POL-04 (rate limit)
# Exceed window on /auth/login → expect 429 with Retry-After

# POL-05 (CSP report endpoint)
curl -X POST https://api.legnaris.com.br/csp-report -H "Content-Type: application/csp-report" -d '{"csp-report":{"document-uri":"..."}}' -w "\n%{http_code}\n"
# Expect 204; verify Loki / Prometheus metric.

# POL-06 (security gates)
make sec-scan
# Must pass with zero new findings; no continue-on-error.

# POL-07 / POL-09 (removal + traceability)
grep -r "desktop/" Makefile .github/workflows/ || echo "clean"
make lint-specs
bash scripts/check_requirement_ids.py web/src/

# POL-10 (tests ship)
make fmt && make lint && make vet && make lint-specs && make test-race
grep -c '^\- \[ \]' .specs/features/014-web-polish-decommission/tasks.md || echo "tasks needed for implement"
```

**When to remove desktop**: only after all above green AND Playwright comparison passes.
