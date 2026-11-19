import { describe, expect, it } from 'vitest';
import { DefensiveParseFailure } from '../api/errors';
import { parsePlanUsage } from '../api/models/plan';

const freePlan = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
};

describe('parsePlanUsage generation fields (LGN-196, LGN-342)', () => {
  it('Happy path: reads the consent requirement and active generation without trial fields', () => {
    const plan = parsePlanUsage({
      ...freePlan,
      llm_consent_required: true,
      active_generation_id: 'gen-1',
    });

    expect(plan.llmConsentRequired).toBe(true);
    expect(plan.activeGenerationId).toBe('gen-1');
  });

  it('Edge: an absent generation block reads as no requirement and no active generation', () => {
    const plan = parsePlanUsage(freePlan);

    expect(plan.llmConsentRequired).toBe(false);
    expect(plan.activeGenerationId).toBeNull();
  });

  it('Edge: a payload without trial fields decodes and exposes none', () => {
    const plan = parsePlanUsage({ ...freePlan, last_free_generation: undefined });

    expect(plan).not.toHaveProperty('trialRemaining');
    expect(plan).not.toHaveProperty('lastFreeGeneration');
  });

  it('Edge: a leftover trial block and last free flag are ignored, never decoded', () => {
    const plan = parsePlanUsage({
      ...freePlan,
      trial: { used: 2, remaining: 1 },
      last_free_generation: true,
    });

    expect(plan).not.toHaveProperty('trialRemaining');
    expect(plan).not.toHaveProperty('lastFreeGeneration');
  });

  it('Edge: a malformed leftover trial block is ignored rather than drift', () => {
    expect(() => parsePlanUsage({ ...freePlan, trial: 'gone' })).not.toThrow();
  });

  it('Edge: a null active generation is no active generation', () => {
    const plan = parsePlanUsage({ ...freePlan, active_generation_id: null });

    expect(plan.activeGenerationId).toBeNull();
  });

  it('Edge: a non-boolean consent requirement is drift', () => {
    expect(() => parsePlanUsage({ ...freePlan, llm_consent_required: 'yes' })).toThrow(
      DefensiveParseFailure,
    );
  });
});
