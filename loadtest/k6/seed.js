import { ensureAccount, waitForAPI } from './lib/accounts.js';

const USERS = Number(__ENV.USERS || 1000);
const PARALLELISM = Number(__ENV.SEED_PARALLELISM || 20);

export const options = {
  scenarios: {
    seed: {
      executor: 'shared-iterations',
      vus: Math.min(PARALLELISM, USERS),
      iterations: USERS,
      maxDuration: __ENV.SEED_MAX_DURATION || '15m',
    },
  },
};

export function setup() {
  waitForAPI();
}

/**
 * Seeds USERS accounts with one sheet each (the API allows one per user).
 * Idempotent: accounts that already exist are only logged into, so the run can
 * be repeated or extended by raising USERS.
 */
export default function () {
  const session = ensureAccount(__ITER);
  if (!session.sheetId) {
    throw new Error(`account ${__ITER} has no sheet`);
  }
}
