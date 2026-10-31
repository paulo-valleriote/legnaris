# Load test results — LGN-33

## DAG recalculation benchmark

Run on 2026-10-31, `make bench-dag`, i5-10400F (12 threads), Go test harness.
Worst case: a 200-field template (100 editable + a chain of 100 derived fields)
where one edit recomputes the whole chain.

| Case | Result |
|------|--------|
| 200 fields, full cascade, sequential | ~0.12 ms/op |
| 200 fields, full cascade, parallel | ~0.055 ms/op |
| `fantasy_v1`, edit `stats.strength` | ~0.069 ms/op |
| p95 over 2000 samples (200 fields) | far below the 50 ms budget (enforced by `TestRecalculateChain200_P95UnderBudget`) |

**Verdict: PASS** — roughly 400x headroom over the p95 < 50 ms target.

## k6 PATCH @ 100 req/s

**Not yet run.** The environment this was authored in had no Docker, k6 or
MongoDB available, so the scenario is untested end to end. Run it following
`loadtest/README.md` and fill the table below; the issue stays open until then.

| Criterion | Target | Measured |
|-----------|--------|----------|
| Sustained PATCH rate | 100 req/s | _pending_ |
| PATCH p95 | < 200 ms | _pending_ |
| Error rate | < 0.1 % | _pending_ |
| Dropped iterations | 0 | _pending_ |
