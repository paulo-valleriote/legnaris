import http from 'k6/http';
import exec from 'k6/execution';
import { check, fail } from 'k6';
import { Counter } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const RATE = Number(__ENV.RATE || 100);
const DURATION = __ENV.DURATION || '2m';
const USERS = Number(__ENV.USERS || 50);
const MAX_VUS = Number(__ENV.MAX_VUS || 200);
const RUN_ID = __ENV.RUN_ID || `${Date.now()}`;
const PASSWORD = 'LoadTest-Password-1!';
const JSON_HEADERS = { 'Content-Type': 'application/json' };

const appErrors = new Counter('legnaris_patch_unexpected_status');

export const options = {
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

function post(path, body, token) {
  const headers = token ? { ...JSON_HEADERS, Authorization: `Bearer ${token}` } : JSON_HEADERS;
  return http.post(`${BASE_URL}${path}`, JSON.stringify(body), { headers, tags: { name: `setup ${path}` } });
}

/**
 * Every user owns exactly one sheet, so the load spreads over USERS accounts
 * each patching its own sheet, the way real traffic does.
 */
export function setup() {
  const sessions = [];
  for (let i = 0; i < USERS; i++) {
    const email = `lt-${RUN_ID}-${i}@loadtest.local`;
    const registered = post('/auth/register', { email, username: `lt${RUN_ID}x${i}`, password: PASSWORD });
    if (registered.status !== 201) {
      fail(`register ${i} answered ${registered.status}: ${registered.body}`);
    }
    const token = registered.json('data.access_token');
    const created = post('/sheets', { template_id: 'fantasy_v1', sheet_name: `load ${i}` }, token);
    if (created.status !== 201) {
      fail(`create sheet ${i} answered ${created.status}: ${created.body}`);
    }
    sessions.push({ token, sheetId: created.json('data.id') });
  }
  return { sessions };
}

export default function ({ sessions }) {
  const session = sessions[exec.scenario.iterationInTest % sessions.length];
  const body = { data: { 'stats.strength': 1 + Math.floor(Math.random() * 20) } };
  const res = http.patch(`${BASE_URL}/sheets/${session.sheetId}`, JSON.stringify(body), {
    headers: { ...JSON_HEADERS, Authorization: `Bearer ${session.token}` },
    tags: { name: 'PATCH /sheets/:id' },
  });
  const ok = check(res, { 'status is 200': (r) => r.status === 200 });
  if (!ok) {
    appErrors.add(1);
  }
}

export function teardown({ sessions }) {
  for (const session of sessions) {
    http.del(`${BASE_URL}/sheets/${session.sheetId}`, null, {
      headers: { Authorization: `Bearer ${session.token}` },
      tags: { name: 'teardown DELETE /sheets/:id' },
    });
  }
}
