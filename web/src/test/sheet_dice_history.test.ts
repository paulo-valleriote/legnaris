import { describe, expect, it } from 'vitest';
import { succeed } from '../dice/errors';
import { ROLL_HISTORY_LIMIT, rememberRoll, type RollEntry } from '../sheet/dice_history';

function entry(id: number): RollEntry {
  return { id, label: `Roll ${id}`, outcome: succeed({ total: id, parts: [] }) };
}

describe('rememberRoll', () => {
  it('Edge: the first roll ever becomes the only entry', () => {
    expect(rememberRoll([], entry(1))).toEqual([entry(1)]);
  });

  it('Happy path: rolls stack newest first', () => {
    expect(rememberRoll([entry(2), entry(1)], entry(3))).toEqual([entry(3), entry(2), entry(1)]);
  });

  it('Edge: a fourth roll evicts the oldest and keeps exactly three', () => {
    expect(rememberRoll([entry(3), entry(2), entry(1)], entry(4))).toEqual([
      entry(4),
      entry(3),
      entry(2),
    ]);
  });

  it('Edge: remembering never mutates the history it was given', () => {
    const history = [entry(1)];

    const next = rememberRoll(history, entry(2));

    expect(history).toEqual([entry(1)]);
    expect(next).not.toBe(history);
  });

  it('Edge: the history is capped at three results', () => {
    expect(ROLL_HISTORY_LIMIT).toBe(3);
  });
});
