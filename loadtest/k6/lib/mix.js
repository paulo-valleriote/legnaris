import http from 'k6/http';
import { BASE_URL, authHeaders, liveToken } from './accounts.js';

export const GET_OP = 'GET /sheets/:id';
export const PATCH_OP = 'PATCH /sheets/:id';
export const CASCADE_OP = 'PATCH /sheets/:id cascade';

const MIX_GET = Number(__ENV.MIX_GET || 55);
const MIX_PATCH = Number(__ENV.MIX_PATCH || 30);
const MIX_CASCADE = Number(__ENV.MIX_CASCADE || 15);

/**
 * pickOperation draws one operation by the MIX_* weights: reads dominate, and a
 * share of the writes touch fields that cascade through the whole DAG.
 */
export function pickOperation() {
  const roll = Math.random() * (MIX_GET + MIX_PATCH + MIX_CASCADE);
  if (roll < MIX_GET) {
    return GET_OP;
  }
  if (roll < MIX_GET + MIX_PATCH) {
    return PATCH_OP;
  }
  return CASCADE_OP;
}

function randomScore() {
  return 1 + Math.floor(Math.random() * 20);
}

/**
 * requestFor sends the operation against the session's own sheet. Extra tags
 * (a ramp step, a soak phase) are merged on top of the operation name.
 */
export function requestFor(operation, session, extraTags = {}) {
  const url = `${BASE_URL}/sheets/${session.sheetId}`;
  const headers = authHeaders(liveToken(session));
  const tags = { name: operation, ...extraTags };
  if (operation === GET_OP) {
    return http.get(url, { headers, tags });
  }
  const data =
    operation === PATCH_OP
      ? { 'combat.hitPoints': randomScore() }
      : {
          'stats.strength': randomScore(),
          'stats.dexterity': randomScore(),
          'stats.constitution': randomScore(),
          'identity.proficiencyBonus': 1 + Math.floor(Math.random() * 6),
        };
  return http.patch(url, JSON.stringify({ data }), { headers, tags });
}
