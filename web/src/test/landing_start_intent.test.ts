import { describe, expect, it } from 'vitest';
import {
  FANTASY_TEMPLATE_ID,
  readStartIntent,
  signUpCtaPath,
  startTemplateId,
} from '../landing/start_intent';

describe('home start intent', () => {
  it('Happy path: the fantasy CTA keeps the sign-up path and carries the intent', () => {
    expect(signUpCtaPath('fantasy')).toBe('/sign-up?start=fantasy');
  });

  it('Happy path: the fantasy intent names the fantasy template', () => {
    expect(startTemplateId('fantasy')).toBe(FANTASY_TEMPLATE_ID);
    expect(startTemplateId('fantasy')).toBe('fantasy_v1');
  });

  it('Happy path: a carried intent is read back from the query', () => {
    expect(readStartIntent(new URLSearchParams('start=fantasy'))).toBe('fantasy');
    expect(readStartIntent(new URLSearchParams('?start=fantasy'))).toBe('fantasy');
  });

  it('Edge: a missing, empty or unknown intent reads as none', () => {
    expect(readStartIntent(new URLSearchParams())).toBeNull();
    expect(readStartIntent(new URLSearchParams('start='))).toBeNull();
    expect(readStartIntent(new URLSearchParams('start=druid'))).toBeNull();
    expect(readStartIntent(new URLSearchParams('start=FANTASY'))).toBeNull();
  });

  it('Edge: a repeated intent uses the first value and an unrelated parameter is ignored', () => {
    expect(
      readStartIntent(new URLSearchParams('next=%2Fsheets%2F1&start=druid&start=fantasy')),
    ).toBe(null);
    expect(readStartIntent(new URLSearchParams('next=%2Fsheets%2F1&start=fantasy'))).toBe(
      'fantasy',
    );
  });
});
