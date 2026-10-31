import http from 'k6/http';
import { sleep, fail } from 'k6';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
export const PASSWORD = 'LoadTest-Password-1!';
export const ACCOUNT_PREFIX = __ENV.ACCOUNT_PREFIX || 'lt-seed';
const JSON_HEADERS = { 'Content-Type': 'application/json' };
const TOKEN_REFRESH_MARGIN_MS = 30000;

export function emailFor(index) {
  return `${ACCOUNT_PREFIX}-${index}@loadtest.local`;
}

export function authHeaders(token) {
  return { ...JSON_HEADERS, Authorization: `Bearer ${token}` };
}

/**
 * postJSON posts a JSON body, authenticated when a token is given. The tag
 * defaults to a "setup" name so setup traffic stays out of the scenario series.
 */
export function postJSON(path, body, token, name = `setup ${path}`) {
  const headers = token ? authHeaders(token) : JSON_HEADERS;
  return http.post(`${BASE_URL}${path}`, JSON.stringify(body), { headers, tags: { name } });
}

function post(path, body, token) {
  return postJSON(path, body, token);
}

export function waitForAPI() {
  for (let attempt = 0; attempt < 60; attempt++) {
    if (http.get(`${BASE_URL}/health`, { tags: { name: 'setup /health' } }).status === 200) {
      return;
    }
    sleep(1);
  }
  fail(`${BASE_URL} did not become healthy within 60s`);
}

function toSession(index, response) {
  return {
    index,
    token: response.json('data.access_token'),
    expiresAt: Date.now() + Number(response.json('data.expires_in') || 0) * 1000,
  };
}

/**
 * login authenticates the seeded account and returns its session, or null when
 * the account does not exist yet (the API answers 401 for unknown accounts).
 */
export function login(index) {
  const res = post('/auth/login', { email: emailFor(index), password: PASSWORD });
  if (res.status === 401) {
    return null;
  }
  if (res.status !== 200) {
    fail(`login ${index} answered ${res.status}: ${res.body}`);
  }
  return toSession(index, res);
}

function register(index) {
  const res = post('/auth/register', {
    email: emailFor(index),
    username: `${ACCOUNT_PREFIX.replace(/[^a-z0-9]/gi, '')}x${index}`,
    password: PASSWORD,
  });
  if (res.status !== 201) {
    fail(`register ${index} answered ${res.status}: ${res.body}`);
  }
  return toSession(index, res);
}

function ensureSheet(session) {
  const mine = http.get(`${BASE_URL}/sheets`, { headers: authHeaders(session.token), tags: { name: 'setup GET /sheets' } });
  if (mine.status === 200) {
    return mine.json('data.id');
  }
  if (mine.status !== 404) {
    fail(`fetch sheet ${session.index} answered ${mine.status}: ${mine.body}`);
  }
  const created = post('/sheets', { template_id: 'fantasy_v1', sheet_name: `load ${session.index}` }, session.token);
  if (created.status !== 201) {
    fail(`create sheet ${session.index} answered ${created.status}: ${created.body}`);
  }
  return created.json('data.id');
}

/**
 * ensureAccount is idempotent: a first run registers the account and creates
 * its sheet, every later run only logs in. The API allows one sheet per user,
 * so the seeded data set is exactly `index -> (account, sheet)`.
 */
export function ensureAccount(index) {
  const session = login(index) || register(index);
  session.sheetId = ensureSheet(session);
  return session;
}

/**
 * liveToken returns a token that is valid now, signing in again when the
 * cached one is about to expire. A soak outlives the access-token TTL, so a
 * token minted in setup() cannot be trusted for the whole run.
 */
export function liveToken(session) {
  if (Date.now() < session.expiresAt - TOKEN_REFRESH_MARGIN_MS) {
    return session.token;
  }
  const renewed = login(session.index);
  if (renewed === null) {
    fail(`seeded account ${session.index} no longer exists`);
  }
  session.token = renewed.token;
  session.expiresAt = renewed.expiresAt;
  return session.token;
}
