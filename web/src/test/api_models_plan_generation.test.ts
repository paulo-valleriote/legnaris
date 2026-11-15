import { describe, expect, it } from 'vitest';
import { DefensiveParseFailure } from '../api/errors';
import { parsePlanUsage } from '../api/models/plan';

const freePlan = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
};

describe('parsePlanUsage generation fields (LGN-196)', () => {
  it('Happy path: reads the trial remaining, consent requirement and active generation', () => {
    const plan = parsePlanUsage({
      ...freePlan,
      trial: { used: 1, remaining: 2 },
      llm_consent_required: true,
      last_free_generation: true,
      active_generation_id: 'gen-1',
    });

    expect(plan.trialRemaining).toBe(2);
    expect(plan.lastFreeGeneration).toBe(true);
    expect(plan.llmConsentRequired).toBe(true);
    expect(plan.activeGenerationId).toBe('gen-1');
  });

  it('Edge: an absent generation block reads as no trial, no requirement and no active generation', () => {
    const plan = parsePlanUsage(freePlan);

    expect(plan.trialRemaining).toBeUndefined();
    expect(plan.llmConsentRequired).toBe(false);
    expect(plan.lastFreeGeneration).toBe(false);
    expect(plan.activeGenerationId).toBeNull();
  });

  it('Edge: a non-boolean last free generation flag is drift', () => {
    expect(() => parsePlanUsage({ ...freePlan, last_free_generation: 1 })).toThrow(
      DefensiveParseFailure,
    );
  });

  it('Edge: a null active generation is no active generation', () => {
    const plan = parsePlanUsage({ ...freePlan, active_generation_id: null });

    expect(plan.activeGenerationId).toBeNull();
  });

  it('Edge: a non-number trial remaining is drift, never coerced', () => {
    expect(() => parsePlanUsage({ ...freePlan, trial: { used: 1, remaining: '2' } })).toThrow(
      DefensiveParseFailure,
    );
  });

  it('Edge: a non-boolean consent requirement is drift', () => {
    expect(() => parsePlanUsage({ ...freePlan, llm_consent_required: 'yes' })).toThrow(
      DefensiveParseFailure,
    );
  });
});
