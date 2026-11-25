import { describe, expect, it } from 'vitest';
import {
  FIFTH_TEMPLATE_ID,
  readStartIntent,
  signUpCtaPath,
  startTemplateId,
} from '../landing/start_intent';

describe('home start intent', () => {
  it('Happy path: the fifth CTA keeps the sign-up path and carries the intent', () => {
    expect(signUpCtaPath('fifth')).toBe('/sign-up?start=fifth');
  });

  it('Happy path: the fifth intent names the 5E Character template', () => {
    expect(startTemplateId('fifth')).toBe(FIFTH_TEMPLATE_ID);
    expect(startTemplateId('fifth')).toBe('fifth_character_v1');
  });

  it('Happy path: fantasy stays accepted as an alias of the fifth intent', () => {
    expect(signUpCtaPath('fantasy')).toBe('/sign-up?start=fantasy');
    expect(startTemplateId('fantasy')).toBe(FIFTH_TEMPLATE_ID);
  });

  it('Happy path: a carried intent is read back from the query', () => {
    expect(readStartIntent(new URLSearchParams('start=fifth'))).toBe('fifth');
    expect(readStartIntent(new URLSearchParams('?start=fifth'))).toBe('fifth');
    expect(readStartIntent(new URLSearchParams('start=fantasy'))).toBe('fantasy');
  });

  it('Edge: a missing, empty or unknown intent reads as none', () => {
    expect(readStartIntent(new URLSearchParams())).toBeNull();
    expect(readStartIntent(new URLSearchParams('start='))).toBeNull();
    expect(readStartIntent(new URLSearchParams('start=druid'))).toBeNull();
    expect(readStartIntent(new URLSearchParams('start=FIFTH'))).toBeNull();
    expect(readStartIntent(new URLSearchParams('start=FANTASY'))).toBeNull();
  });

  it('Edge: a repeated intent uses the first value and an unrelated parameter is ignored', () => {
    expect(readStartIntent(new URLSearchParams('next=%2Fsheets%2F1&start=druid&start=fifth'))).toBe(
      null,
    );
    expect(readStartIntent(new URLSearchParams('next=%2Fsheets%2F1&start=fifth'))).toBe('fifth');
    expect(readStartIntent(new URLSearchParams('next=%2Fsheets%2F1&start=fantasy'))).toBe(
      'fantasy',
    );
  });
});
