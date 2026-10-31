import exec from 'k6/execution';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import { ensureAccount, login, waitForAPI } from './lib/accounts.js';
import { pickOperation, requestFor } from './lib/mix.js';
import { currentStep, rampStages } from './lib/ramp.js';

const USERS = Number(__ENV.USERS || 200);
const MAX_VUS = Number(__ENV.MAX_VUS || 500);
const BACKGROUND_RATE = Number(__ENV.BACKGROUND_RATE || 200);
const STEPS = (__ENV.LOGIN_STAGES || '5,10,20,40,80').split(',').map(Number);
const STEP_SECONDS = Number(__ENV.STEP_SECONDS || 60);
const RAMP_SECONDS = Number(__ENV.RAMP_SECONDS || 10);
const LOGIN_OP = 'POST /auth/login';

const unexpectedStatus = new Counter('legnaris_unexpected_status');

/**
 * Login is deliberately expensive (bcrypt, cost 12: ~230 ms of one core per
 * attempt), so it competes with every other request for CPU. This test holds a
 * steady background of sheet traffic and ramps the login rate on top of it: the
 * question is not how many logins per second the API can do, but at which login
 * rate the sheet operations start to slow down. Every request carries the
 * login-rate step, so each step has its own background p95.
 */
export const options = {
  setupTimeout: __ENV.SETUP_TIMEOUT || '10m',
  tags: { testid: 'auth-storm' },
  scenarios: {
    background: {
      executor: 'constant-arrival-rate',
      exec: 'background',
      rate: BACKGROUND_RATE,
      timeUnit: '1s',
      duration: `${STEPS.length * STEP_SECONDS}s`,
      preAllocatedVUs: Math.min(MAX_VUS, 50),
      maxVUs: MAX_VUS,
    },
    logins: {
      executor: 'ramping-arrival-rate',
      exec: 'loginStorm',
      startRate: STEPS[0],
      timeUnit: '1s',
      preAllocatedVUs: Math.min(MAX_VUS, 100),
      maxVUs: MAX_VUS,
      stages: rampStages(STEPS, STEP_SECONDS, RAMP_SECONDS),
    },
  },
  thresholds: Object.fromEntries(
    STEPS.flatMap((rate) => [
      [`http_req_duration{step:${rate},group:background}`, ['p(95)<200']],
      [`http_req_duration{step:${rate},name:${LOGIN_OP}}`, ['p(95)<1000']],
      [`http_req_failed{step:${rate},name:${LOGIN_OP}}`, ['rate<0.01']],
    ]),
  ),
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

export function background({ sessions }) {
  const session = sessions[exec.scenario.iterationInTest % sessions.length];
  const operation = pickOperation();
  const res = requestFor(operation, session, { group: 'background', step: currentStep(STEPS, STEP_SECONDS) });
  if (!check(res, { 'background status is 200': (r) => r.status === 200 })) {
    unexpectedStatus.add(1, { name: operation, status: String(res.status) });
  }
}

export function loginStorm({ sessions }) {
  const index = sessions[exec.scenario.iterationInTest % sessions.length].index;
  const session = login(index, LOGIN_OP, { step: currentStep(STEPS, STEP_SECONDS) });
  if (!check(session, { 'login succeeds': (s) => s !== null })) {
    unexpectedStatus.add(1, { name: LOGIN_OP });
  }
}
