import exec from 'k6/execution';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { ensureAccount, waitForAPI } from './lib/accounts.js';
import { pickOperation, requestFor } from './lib/mix.js';

const RATE = Number(__ENV.SOAK_RATE || 100);
const DURATION_SECONDS = Number(__ENV.SOAK_SECONDS || 1800);
const USERS = Number(__ENV.USERS || 200);
const MAX_VUS = Number(__ENV.MAX_VUS || 500);
const EDGE_SHARE = 0.2;

const unexpectedStatus = new Counter('legnaris_unexpected_status');

/**
 * Soak holds one moderate rate for a long time. The pass/fail lines are the
 * same budget as the ramp, set per phase: comparing `phase:start` with
 * `phase:end` in the summary shows latency drift, which a run-wide aggregate
 * would average away. Failures are scoped to the phases too: the run-wide rate
 * also counts the 401s of a first-run login in setup().
 */
export const options = {
  setupTimeout: __ENV.SETUP_TIMEOUT || '10m',
  tags: { testid: 'soak' },
  scenarios: {
    soak: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: `${DURATION_SECONDS}s`,
      preAllocatedVUs: Math.min(MAX_VUS, 100),
      maxVUs: MAX_VUS,
    },
  },
  thresholds: {
    'http_req_duration{phase:start}': ['p(95)<200'],
    'http_req_duration{phase:middle}': ['p(95)<200'],
    'http_req_duration{phase:end}': ['p(95)<200'],
    'http_req_failed{phase:start}': ['rate<0.001'],
    'http_req_failed{phase:middle}': ['rate<0.001'],
    'http_req_failed{phase:end}': ['rate<0.001'],
    dropped_iterations: ['count==0'],
    legnaris_unexpected_status: ['count==0'],
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

function currentPhase() {
  const share = (Date.now() - exec.scenario.startTime) / (DURATION_SECONDS * 1000);
  if (share < EDGE_SHARE) {
    return 'start';
  }
  return share > 1 - EDGE_SHARE ? 'end' : 'middle';
}

export default function ({ sessions }) {
  const session = sessions[exec.scenario.iterationInTest % sessions.length];
  const operation = pickOperation();
  const res = requestFor(operation, session, { phase: currentPhase() });
  if (!check(res, { 'status is 200': (r) => r.status === 200 })) {
    unexpectedStatus.add(1, { name: operation });
  }
}
