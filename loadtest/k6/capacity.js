import exec from 'k6/execution';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { ensureAccount, waitForAPI } from './lib/accounts.js';
import { pickOperation, requestFor } from './lib/mix.js';
import { currentStep, rampStages, stepThresholds } from './lib/ramp.js';

const USERS = Number(__ENV.USERS || 200);
const MAX_VUS = Number(__ENV.MAX_VUS || 1000);
const STEPS = (__ENV.STAGES || '100,200,400,800,1600').split(',').map(Number);
const STEP_SECONDS = Number(__ENV.STEP_SECONDS || 60);
const RAMP_SECONDS = Number(__ENV.RAMP_SECONDS || 10);
const P95_BUDGET_MS = Number(__ENV.P95_BUDGET_MS || 200);

const unexpectedStatus = new Counter('legnaris_unexpected_status');

/**
 * Capacity: climb until a step breaches the budget, then stop. The last step
 * that survived is the measured capacity (see lib/ramp.js).
 */
export const options = {
  setupTimeout: __ENV.SETUP_TIMEOUT || '10m',
  tags: { testid: 'capacity' },
  scenarios: {
    mix: {
      executor: 'ramping-arrival-rate',
      startRate: STEPS[0],
      timeUnit: '1s',
      preAllocatedVUs: Math.min(MAX_VUS, 200),
      maxVUs: MAX_VUS,
      stages: rampStages(STEPS, STEP_SECONDS, RAMP_SECONDS),
    },
  },
  thresholds: {
    dropped_iterations: ['count==0'],
    legnaris_unexpected_status: ['count==0'],
    ...stepThresholds(STEPS, { p95BudgetMs: P95_BUDGET_MS, abort: true }),
  },
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
  const ok = check(res, { 'status is 200': (r) => r.status === 200 });
  if (!ok) {
    unexpectedStatus.add(1, { name: operation });
  }
}
