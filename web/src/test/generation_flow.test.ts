import { describe, expect, it } from 'vitest';
import type { Generation, GenerationStatus } from '../api/models/generation';
import {
  CONCEPT_MAX_LENGTH,
  CONCEPT_MIN_LENGTH,
  WAIT_LIMIT_MS,
  conceptLength,
  conceptState,
  decideWait,
  trialAllowance,
} from '../generate/generation_flow';

function generationWith(status: GenerationStatus, errorCode?: string): Generation {
  return {
    id: 'gen-1',
    status,
    kind: 'character',
    templateId: 'fantasy_v1',
    values: undefined,
    errorCode,
  };
}

describe('conceptState (LGN-196 concept bounds 10-1000)', () => {
  const tests = [
    { name: 'Edge: empty text is too short', text: '', state: 'tooShort' },
    { name: 'Edge: 9 characters is too short', text: '123456789', state: 'tooShort' },
    { name: 'Edge: 10 characters is the lower bound', text: '1234567890', state: 'ok' },
    {
      name: 'Happy path: a typical concept is ok',
      text: 'A wandering ranger of the north',
      state: 'ok',
    },
    { name: 'Edge: 1000 characters is the upper bound', text: 'a'.repeat(1000), state: 'ok' },
    { name: 'Edge: 1001 characters is too long', text: 'a'.repeat(1001), state: 'tooLong' },
    { name: 'Edge: whitespace does not count as a concept', text: '          ', state: 'tooShort' },
  ] as const;

  for (const tc of tests) {
    it(tc.name, () => {
      expect(conceptState(tc.text)).toBe(tc.state);
    });
  }

  it('Edge: the length counts code points, so an emoji is one character', () => {
    expect(conceptLength('🧙'.repeat(10))).toBe(10);
    expect(conceptState('🧙'.repeat(10))).toBe('ok');
  });

  it('Edge: the bounds are exactly the published constants', () => {
    expect(CONCEPT_MIN_LENGTH).toBe(10);
    expect(CONCEPT_MAX_LENGTH).toBe(1000);
  });
});

describe('decideWait (LGN-196 poll every 5 s, give up at 120 s)', () => {
  const tests = [
    {
      name: 'Happy path: a queued generation keeps waiting',
      status: 'queued',
      elapsed: 0,
      outcome: 'keepWaiting',
    },
    {
      name: 'Happy path: a running generation keeps waiting',
      status: 'running',
      elapsed: 5_000,
      outcome: 'keepWaiting',
    },
    {
      name: 'Happy path: a succeeded generation is ready to preview',
      status: 'succeeded',
      elapsed: 10_000,
      outcome: 'ready',
    },
    {
      name: 'Edge: a succeeded generation is ready even past the wait limit',
      status: 'succeeded',
      elapsed: WAIT_LIMIT_MS + 1,
      outcome: 'ready',
    },
    {
      name: 'Edge: an accepted generation is already a Sheet',
      status: 'accepted',
      elapsed: 0,
      outcome: 'alreadyAccepted',
    },
    {
      name: 'Edge: one millisecond before the limit still waits',
      status: 'running',
      elapsed: WAIT_LIMIT_MS - 1,
      outcome: 'keepWaiting',
    },
    {
      name: 'Edge: exactly at the limit gives up',
      status: 'running',
      elapsed: WAIT_LIMIT_MS,
      outcome: 'timedOut',
    },
    {
      name: 'Error: a failed generation is a failure',
      status: 'failed',
      elapsed: 1_000,
      outcome: 'failed',
    },
  ] as const;

  for (const tc of tests) {
    it(tc.name, () => {
      expect(decideWait(generationWith(tc.status), tc.elapsed)).toBe(tc.outcome);
    });
  }

  it('Edge: no generation read yet keeps waiting until the limit', () => {
    expect(decideWait(undefined, 0)).toBe('keepWaiting');
    expect(decideWait(undefined, WAIT_LIMIT_MS)).toBe('timedOut');
  });

  it('Edge: the failure code travels with the failed outcome for the copy', () => {
    expect(decideWait(generationWith('failed', 'invalid_output'), 0)).toBe('failed');
  });
});

describe('trialAllowance (LGN-196 quota line)', () => {
  it('Happy path: a Free plan with a trial reads the remaining allowance', () => {
    expect(trialAllowance({ trialRemaining: 2 })).toEqual({ kind: 'trial', remaining: 2 });
  });

  it('Edge: zero remaining is an exhausted trial', () => {
    expect(trialAllowance({ trialRemaining: 0 })).toEqual({ kind: 'trial', remaining: 0 });
  });

  it('Edge: no trial in the payload reads as unknown, never as zero', () => {
    expect(trialAllowance({ trialRemaining: undefined })).toEqual({ kind: 'unknown' });
  });
});
