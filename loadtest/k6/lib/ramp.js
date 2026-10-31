import exec from 'k6/execution';
import { CASCADE_OP, GET_OP, PATCH_OP } from './mix.js';

const ABORT_EVAL = '10s';
const MELTDOWN_FAILED_RATE = 0.5;

/**
 * rampStages climbs through the steps: each one ramps for rampSeconds, then
 * holds its rate, so the hold part measures a stable rate.
 */
export function rampStages(steps, stepSeconds, rampSeconds) {
  const out = [];
  for (const rate of steps) {
    out.push({ duration: `${rampSeconds}s`, target: rate });
    out.push({ duration: `${stepSeconds - rampSeconds}s`, target: rate });
  }
  return out;
}

/**
 * currentStep names the step the run is in by elapsed time, to tag requests so
 * every step gets its own series and thresholds.
 */
export function currentStep(steps, stepSeconds) {
  const elapsed = Date.now() - exec.scenario.startTime;
  const index = Math.min(steps.length - 1, Math.floor(elapsed / (stepSeconds * 1000)));
  return String(steps[index]);
}

/**
 * stepThresholds gives every step its own pass/fail line, because a run-wide
 * aggregate would hide a collapse at the top step behind the healthy ones.
 *
 * With abort, a breach of the budget (p95 above p95BudgetMs or more than 1 %
 * failures) stops the run: the last step that survived is the capacity. Without
 * it the run keeps pushing to see how the service degrades and recovers, and
 * only a meltdown (half the requests failing) stops it.
 */
export function stepThresholds(steps, { p95BudgetMs, abort }) {
  const stop = { abortOnFail: true, delayAbortEval: ABORT_EVAL };
  const out = {};
  for (const rate of steps) {
    const step = `step:${rate}`;
    out[`http_req_duration{${step}}`] = [
      abort ? { threshold: `p(95)<${p95BudgetMs}`, ...stop } : `p(95)<${p95BudgetMs}`,
      'p(99)<500',
    ];
    out[`http_req_failed{${step}}`] = abort
      ? [{ threshold: 'rate<0.01', ...stop }]
      : ['rate<0.01', { threshold: `rate<${MELTDOWN_FAILED_RATE}`, ...stop }];
    out[`http_req_duration{${step},name:${GET_OP}}`] = ['p(95)<100'];
    out[`http_req_duration{${step},name:${PATCH_OP}}`] = ['p(95)<200'];
    out[`http_req_duration{${step},name:${CASCADE_OP}}`] = ['p(95)<200'];
    out[`http_req_failed{${step},name:${GET_OP}}`] = ['rate<0.001'];
    out[`http_req_failed{${step},name:${PATCH_OP}}`] = ['rate<0.001'];
    out[`http_req_failed{${step},name:${CASCADE_OP}}`] = ['rate<0.001'];
  }
  return out;
}
