import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';
import { BASE_URL, authHeaders, ensureAccount, liveToken, waitForAPI } from './lib/accounts.js';

const VARIANT = __ENV.VARIANT || 'distinct';
const CONCURRENCY = Number(__ENV.CONCURRENCY || 15);
const ITERATIONS = Number(__ENV.ITERATIONS || 100);
const DUPLICATE_RATE = Number(__ENV.DUPLICATE_RATE || 0.3);
const MAX_RETRIES = Number(__ENV.MAX_RETRIES || 50);
const BACKOFF_MIN_MS = Number(__ENV.BACKOFF_MIN_MS || 20);
const BACKOFF_MAX_MS = Number(__ENV.BACKOFF_MAX_MS || 80);
const ACCOUNT_BASE = Number(__ENV.CONCURRENCY_ACCOUNT_BASE || 1000000);

const PATCH_OP = 'PATCH /sheets/:id';
const SAME_FIELD = 'stats.strength';
const ABILITIES = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'];
const DISTINCT_FIELDS = [
  ...ABILITIES.map((a) => `stats.${a}`),
  'combat.baseArmorClass',
  'combat.initiative',
  'combat.speed',
  'combat.hitPointsMax',
  'combat.hitPoints',
  'combat.hitPointsTemp',
  'identity.level',
  'identity.xp',
  'identity.proficiencyBonus',
];

const lostUpdates = new Counter('legnaris_lost_updates');
const inconsistentDerived = new Counter('legnaris_inconsistent_derived');
const conflicts = new Counter('legnaris_conflicts');
const unexpectedStatus = new Counter('legnaris_unexpected_status');

http.setResponseCallback(http.expectedStatuses(200, 409));

if (VARIANT !== 'distinct' && VARIANT !== 'same') {
  throw new Error(`VARIANT must be "distinct" or "same", got "${VARIANT}"`);
}
if (VARIANT === 'distinct' && CONCURRENCY > DISTINCT_FIELDS.length) {
  throw new Error(`distinct needs one field per VU: CONCURRENCY <= ${DISTINCT_FIELDS.length}`);
}

export const options = {
  setupTimeout: __ENV.SETUP_TIMEOUT || '10m',
  tags: { testid: 'concurrency' },
  scenarios: {
    storm: {
      executor: 'per-vu-iterations',
      vus: CONCURRENCY,
      iterations: ITERATIONS,
      maxDuration: __ENV.MAX_DURATION || '5m',
    },
  },
  thresholds: {
    legnaris_lost_updates: ['count==0'],
    legnaris_inconsistent_derived: ['count==0'],
    legnaris_unexpected_status: ['count==0'],
    [`http_req_failed{name:${PATCH_OP}}`]: ['rate<0.001'],
    [`http_req_duration{name:${PATCH_OP}}`]: ['p(95)<200'],
  },
  summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

/**
 * valueFor is unique per (writer, iteration), so a final value identifies
 * exactly which write produced it.
 */
function valueFor(writer, iteration) {
  return writer * 1000 + iteration + 1;
}

function fieldFor(writer) {
  return VARIANT === 'same' ? SAME_FIELD : DISTINCT_FIELDS[writer];
}

function dig(data, path) {
  return path.split('.').reduce((node, key) => (node === undefined || node === null ? undefined : node[key]), data);
}

/**
 * Two accounts, one per variant: the sheet under test is never shared with the
 * ramp or with the other variant, so its history is only this run's writes.
 */
export function setup() {
  waitForAPI();
  const session = ensureAccount(ACCOUNT_BASE + (VARIANT === 'same' ? 1 : 0));
  return { session };
}

/**
 * Each VU writes its own sequence one request at a time (a client that waits
 * for the answer), and some iterations are sent twice in parallel, the way a
 * retry after a timeout or an event storm duplicates a write.
 */
export default function ({ session }) {
  const writer = exec.vu.idInTest - 1;
  const url = `${BASE_URL}/sheets/${session.sheetId}`;
  const body = JSON.stringify({ data: { [fieldFor(writer)]: valueFor(writer, exec.vu.iterationInScenario) } });
  const params = { headers: authHeaders(liveToken(session)), tags: { name: PATCH_OP, variant: VARIANT } };

  const responses =
    Math.random() < DUPLICATE_RATE
      ? http.batch([['PATCH', url, body, params], ['PATCH', url, body, params]])
      : [http.patch(url, body, params)];

  for (const res of responses) {
    const final = retryOnConflict(res, url, body, params);
    if (!check(final, { 'status is 200': (r) => r.status === 200 })) {
      unexpectedStatus.add(1, { status: String(final.status) });
    }
  }
}

/**
 * retryOnConflict resends a PATCH the API refused with 409 (a lost version
 * race), the way a client is expected to. The wait is a short jittered backoff
 * instead of the 1 s Retry-After hint, so a 15-writer storm on one sheet
 * finishes in seconds; the resend is idempotent because it carries the same
 * absolute values. Each 409 is counted, and giving up after MAX_RETRIES counts
 * as an unexpected status.
 */
function retryOnConflict(first, url, body, params) {
  let res = first;
  for (let attempt = 0; res.status === 409 && attempt < MAX_RETRIES; attempt++) {
    conflicts.add(1);
    sleep((BACKOFF_MIN_MS + Math.random() * (BACKOFF_MAX_MS - BACKOFF_MIN_MS)) / 1000);
    res = http.patch(url, body, params);
  }
  return res;
}

function lastWriteOf(writer) {
  return valueFor(writer, ITERATIONS - 1);
}

function writtenValuesOf(field) {
  const accepted = new Set();
  for (let writer = 0; writer < CONCURRENCY; writer++) {
    if (fieldFor(writer) !== field) {
      continue;
    }
    for (let iteration = 0; iteration < ITERATIONS; iteration++) {
      accepted.add(valueFor(writer, iteration));
    }
  }
  return accepted;
}

function verifyWrites(data) {
  const problems = [];
  if (VARIANT === 'same') {
    const actual = dig(data, SAME_FIELD);
    if (!writtenValuesOf(SAME_FIELD).has(actual)) {
      problems.push(`${SAME_FIELD}=${actual} was never written`);
    }
    return problems;
  }
  for (let writer = 0; writer < CONCURRENCY; writer++) {
    const field = fieldFor(writer);
    const actual = dig(data, field);
    if (actual !== lastWriteOf(writer)) {
      problems.push(`${field}: expected last write ${lastWriteOf(writer)}, stored ${actual}`);
    }
  }
  return problems;
}

/**
 * verifyDerived re-computes every modifier and the armor class from the stored
 * inputs. A derived value that does not match its inputs is a cascade computed
 * from a stale read and persisted after a newer write: a lost update.
 */
function verifyDerived(data) {
  const problems = [];
  const base = dig(data, 'rules.modifierBase');
  const step = dig(data, 'rules.modifierStep');
  const modifier = {};
  for (const ability of ABILITIES) {
    modifier[ability] = Math.floor((dig(data, `stats.${ability}`) - base) / step);
    const stored = dig(data, `modifiers.${ability}Mod`);
    if (stored !== modifier[ability]) {
      problems.push(`modifiers.${ability}Mod=${stored}, inputs give ${modifier[ability]}`);
    }
  }
  const armorClass =
    dig(data, 'combat.baseArmorClass') +
    modifier.dexterity * dig(data, 'config.acUsesDex') +
    modifier.constitution * dig(data, 'config.acUsesCon');
  const storedArmorClass = dig(data, 'derived.armorClass');
  if (storedArmorClass !== armorClass) {
    problems.push(`derived.armorClass=${storedArmorClass}, inputs give ${armorClass}`);
  }
  return problems;
}

export function teardown({ session }) {
  const res = http.get(`${BASE_URL}/sheets/${session.sheetId}`, {
    headers: authHeaders(liveToken(session)),
    tags: { name: 'verify GET /sheets/:id' },
  });
  if (res.status !== 200) {
    lostUpdates.add(1);
    console.error(`verification read answered ${res.status}: ${res.body}`);
    return;
  }
  const data = res.json('data.data');
  const writeProblems = verifyWrites(data);
  const derivedProblems = verifyDerived(data);
  lostUpdates.add(writeProblems.length);
  inconsistentDerived.add(derivedProblems.length);
  for (const problem of [...writeProblems, ...derivedProblems]) {
    console.error(`[${VARIANT}] ${problem}`);
  }
  console.log(`[${VARIANT}] verified ${CONCURRENCY} writers: ${writeProblems.length} lost writes, ${derivedProblems.length} inconsistent derived values`);
}
