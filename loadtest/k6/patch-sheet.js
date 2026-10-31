import http from 'k6/http';
import exec from 'k6/execution';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { BASE_URL, authHeaders, ensureAccount, liveToken, waitForAPI } from './lib/accounts.js';

const RATE = Number(__ENV.RATE || 100);
const DURATION = __ENV.DURATION || '2m';
const USERS = Number(__ENV.USERS || 50);
const MAX_VUS = Number(__ENV.MAX_VUS || 200);

const appErrors = new Counter('legnaris_patch_unexpected_status');

export const options = {
  setupTimeout: __ENV.SETUP_TIMEOUT || '10m',
  tags: { testid: 'patch-sheet' },
  scenarios: {
    patch_sustained: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: Math.min(MAX_VUS, RATE),
      maxVUs: MAX_VUS,
    },
  },
  thresholds: {
    'http_req_duration{name:PATCH /sheets/:id}': ['p(95)<200'],
    'http_req_failed{name:PATCH /sheets/:id}': ['rate<0.001'],
    dropped_iterations: ['count==0'],
    legnaris_patch_unexpected_status: ['count==0'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

/**
 * Uses the seeded accounts (see seed.js): every user owns exactly one sheet
 * (the API allows one per user), so the load spreads over USERS accounts each
 * patching its own sheet, the way real traffic does. Missing accounts are
 * created on the fly, which is slow; seed first for a large USERS.
 */
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
  const body = { data: { 'stats.strength': 1 + Math.floor(Math.random() * 20) } };
  const res = http.patch(`${BASE_URL}/sheets/${session.sheetId}`, JSON.stringify(body), {
    headers: authHeaders(liveToken(session)),
    tags: { name: 'PATCH /sheets/:id' },
  });
  const ok = check(res, { 'status is 200': (r) => r.status === 200 });
  if (!ok) {
    appErrors.add(1);
  }
}
