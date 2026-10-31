import exec from 'k6/execution';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { ensureAccount, waitForAPI } from './lib/accounts.js';
import { CASCADE_OP, GET_OP, PATCH_OP, pickOperation, requestFor } from './lib/mix.js';

const USERS = Number(__ENV.USERS || 200);
const MAX_VUS = Number(__ENV.MAX_VUS || 1000);
const STEPS = (__ENV.STAGES || '100,200,400,800,1600').split(',').map(Number);
const STEP_SECONDS = Number(__ENV.STEP_SECONDS || 60);
const RAMP_SECONDS = Number(__ENV.RAMP_SECONDS || 10);
const P95_BUDGET_MS = Number(__ENV.P95_BUDGET_MS || 200);

const unexpectedStatus = new Counter('legnaris_unexpected_status');

function stages() {
  const out = [];
  for (const rate of STEPS) {
    out.push({ duration: `${RAMP_SECONDS}s`, target: rate });
    out.push({ duration: `${STEP_SECONDS - RAMP_SECONDS}s`, target: rate });
  }
  return out;
}

/**
 * thresholds gives every step its own pass/fail line. A breach of the budget
 * (p95 above P95_BUDGET_MS, 200ms by default, or more than 1% failures) aborts the run, so the last step
 * that did not breach is the measured capacity. The per-operation lines report
 * without aborting. Thresholds are per step because the run-wide aggregate
 * would hide a collapse at the top step behind the healthy ones.
 */
function thresholds() {
  const out = {
    dropped_iterations: ['count==0'],
    legnaris_unexpected_status: ['count==0'],
  };
  for (const rate of STEPS) {
    const step = `step:${rate}`;
    out[`http_req_duration{${step}}`] = [
      { threshold: `p(95)<${P95_BUDGET_MS}`, abortOnFail: true, delayAbortEval: '10s' },
      'p(99)<500',
    ];
    out[`http_req_failed{${step}}`] = [{ threshold: 'rate<0.01', abortOnFail: true, delayAbortEval: '10s' }];
    out[`http_req_duration{${step},name:${GET_OP}}`] = ['p(95)<100'];
    out[`http_req_duration{${step},name:${PATCH_OP}}`] = ['p(95)<200'];
    out[`http_req_duration{${step},name:${CASCADE_OP}}`] = ['p(95)<200'];
    out[`http_req_failed{${step},name:${GET_OP}}`] = ['rate<0.001'];
    out[`http_req_failed{${step},name:${PATCH_OP}}`] = ['rate<0.001'];
    out[`http_req_failed{${step},name:${CASCADE_OP}}`] = ['rate<0.001'];
  }
  return out;
}

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
      stages: stages(),
    },
  },
  thresholds: thresholds(),
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

function currentStep() {
  const elapsed = Date.now() - exec.scenario.startTime;
  const index = Math.min(STEPS.length - 1, Math.floor(elapsed / (STEP_SECONDS * 1000)));
  return String(STEPS[index]);
}

export default function ({ sessions }) {
  const session = sessions[exec.scenario.iterationInTest % sessions.length];
  const operation = pickOperation();
  const res = requestFor(operation, session, { step: currentStep() });
  const ok = check(res, { 'status is 200': (r) => r.status === 200 });
  if (!ok) {
    unexpectedStatus.add(1, { name: operation });
  }
}
