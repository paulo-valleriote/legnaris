import exec from 'k6/execution';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { ensureAccount, waitForAPI } from './lib/accounts.js';
import { pickOperation, requestFor } from './lib/mix.js';
import { currentStep, rampStages, stepThresholds } from './lib/ramp.js';

const USERS = Number(__ENV.USERS || 500);
const MAX_VUS = Number(__ENV.MAX_VUS || 3000);
const STEPS = (__ENV.STRESS_STAGES || '2000,3000,4500,6000,300').split(',').map(Number);
const STEP_SECONDS = Number(__ENV.STEP_SECONDS || 60);
const RAMP_SECONDS = Number(__ENV.RAMP_SECONDS || 10);
const P95_BUDGET_MS = Number(__ENV.P95_BUDGET_MS || 200);

const unexpectedStatus = new Counter('legnaris_unexpected_status');

/**
 * Stress goes past the limit the capacity run found, to learn how the service
 * behaves when it is overloaded, not just where the limit is: does latency
 * degrade smoothly or fall off a cliff, does it answer 5xx or 429 or just slow
 * down, and does it recover. The last step (300 by default) drops back well
 * under the limit: its p95 and error rate must return to normal, otherwise the
 * overload left damage behind (a queue that never drains, leaked goroutines).
 *
 * Nothing aborts on a breached budget, only on a meltdown (see lib/ramp.js),
 * so a red step is a finding, not a failed run. Dropped iterations are
 * expected here and reported, not thresholded: they say the generator could not
 * keep the rate, which is the overload showing.
 */
export const options = {
  setupTimeout: __ENV.SETUP_TIMEOUT || '10m',
  tags: { testid: 'stress' },
  scenarios: {
    overload: {
      executor: 'ramping-arrival-rate',
      startRate: STEPS[0],
      timeUnit: '1s',
      preAllocatedVUs: Math.min(MAX_VUS, 500),
      maxVUs: MAX_VUS,
      stages: rampStages(STEPS, STEP_SECONDS, RAMP_SECONDS),
    },
  },
  thresholds: stepThresholds(STEPS, { p95BudgetMs: P95_BUDGET_MS, abort: false }),
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export function setup() {
  waitForAPI();
  const sessions = [];
  for (let i = 0; i < USERS; i++) {
    sessions.push(ensureAccount(i));
  }
  return { sessions };
}

export default function ({ sessions }) {
  const session = sessions[exec.scenario.iterationInTest % sessions.length];
  const operation = pickOperation();
  const res = requestFor(operation, session, { step: currentStep(STEPS, STEP_SECONDS) });
  if (!check(res, { 'status is 200': (r) => r.status === 200 })) {
    unexpectedStatus.add(1, { name: operation, status: String(res.status) });
  }
}
