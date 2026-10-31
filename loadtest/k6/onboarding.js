import exec from 'k6/execution';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { PASSWORD, authHeaders, postJSON, waitForAPI } from './lib/accounts.js';

const RATE = Number(__ENV.ONBOARDING_RATE || 5);
const DURATION = __ENV.DURATION || '1m';
const MAX_VUS = Number(__ENV.MAX_VUS || 200);
const RUN_ID = __ENV.RUN_ID || `${Date.now()}`;

const REGISTER_OP = 'POST /auth/register';
const CREATE_OP = 'POST /sheets';

const unexpectedStatus = new Counter('legnaris_unexpected_status');

export const options = {
  tags: { testid: 'onboarding' },
  scenarios: {
    onboarding: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: Math.min(MAX_VUS, 50),
      maxVUs: MAX_VUS,
    },
  },
  thresholds: {
    [`http_req_duration{name:${REGISTER_OP}}`]: ['p(95)<1500'],
    [`http_req_duration{name:${CREATE_OP}}`]: ['p(95)<200'],
    [`http_req_failed{name:${REGISTER_OP}}`]: ['rate<0.01'],
    [`http_req_failed{name:${CREATE_OP}}`]: ['rate<0.01'],
    dropped_iterations: ['count==0'],
    legnaris_unexpected_status: ['count==0'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export function setup() {
  waitForAPI();
}

/**
 * Onboarding is the one flow that is not steady-state: every iteration creates
 * a brand-new account (password hashing) and its only sheet. It is measured on
 * its own, at a low rate, because it weighs very differently from a PATCH.
 * Run it only on a throwaway environment: accounts are not removed.
 */
export default function () {
  const n = exec.scenario.iterationInTest;
  const registered = postJSON(
    '/auth/register',
    { email: `lt-onb-${RUN_ID}-${n}@loadtest.local`, username: `ltonb${RUN_ID}x${n}`, password: PASSWORD },
    null,
    REGISTER_OP,
  );
  if (!check(registered, { 'register is 201': (r) => r.status === 201 })) {
    unexpectedStatus.add(1, { op: REGISTER_OP, status: String(registered.status) });
    return;
  }

  const created = postJSON(
    '/sheets',
    { template_id: 'fantasy_v1', sheet_name: `onboarding ${n}` },
    registered.json('data.access_token'),
    CREATE_OP,
  );
  if (!check(created, { 'create sheet is 201': (r) => r.status === 201 })) {
    unexpectedStatus.add(1, { op: CREATE_OP, status: String(created.status) });
  }
}
