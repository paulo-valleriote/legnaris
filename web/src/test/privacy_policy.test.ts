import { describe, expect, it } from 'vitest';
import { policyLocale, policyMarkdown, stripLeadingComment } from '../legal/privacy_policy';

describe('privacy policy locale and source (LGN-234)', () => {
  it('Happy path: a Portuguese first preference selects pt-BR', () => {
    expect(policyLocale(['pt-BR', 'en'])).toBe('pt-BR');
    expect(policyLocale(['pt'])).toBe('pt-BR');
  });

  it('Edge: other, empty and blank preferences fall back to English', () => {
    expect(policyLocale([])).toBe('en');
    expect(policyLocale(['   ', ''])).toBe('en');
    expect(policyLocale(['fr-FR', 'pt-BR'])).toBe('en');
    expect(policyLocale(['ptolemy'])).toBe('en');
  });

  it('Edge: a Portuguese preference after a blank entry is still the first real one', () => {
    expect(policyLocale(['  ', 'pt-PT'])).toBe('pt-BR');
  });

  it('Edge: the leading HTML comment is removed and the body is kept', () => {
    expect(stripLeadingComment('<!-- note -->\n\n# Body')).toBe('\n\n# Body');
    expect(stripLeadingComment('# Body <!-- kept -->')).toBe('# Body <!-- kept -->');
    expect(stripLeadingComment('')).toBe('');
  });

  it('Edge: an unterminated leading comment is left as it is', () => {
    expect(stripLeadingComment('<!-- open')).toBe('<!-- open');
  });

  it.each(['en', 'pt-BR'] as const)(
    'Happy path: the %s policy names every processor, including Stripe for payments',
    (locale) => {
      const body = policyMarkdown(locale);

      for (const processor of ['**Stripe**', '**Resend**', '**OpenRouter**']) {
        expect(body).toContain(processor);
      }
    },
  );

  it.each(['en', 'pt-BR'] as const)(
    'Edge: the %s policy keeps the AI revocation path and the 24-hour retention of the consent disclosure',
    (locale) => {
      const body = policyMarkdown(locale);

      expect(body).toContain('/account');
      expect(body).toContain('DELETE /me/llm-consent');
      expect(body).toContain('24');
    },
  );

  it('Happy path: each locale source opens with its own heading once the comment is gone', () => {
    expect(policyMarkdown('en').trimStart().startsWith('# Privacy policy')).toBe(true);
    expect(policyMarkdown('pt-BR').trimStart().startsWith('# Política de privacidade')).toBe(true);
    expect(policyMarkdown('en')).not.toContain('<!--');
    expect(policyMarkdown('pt-BR')).not.toContain('<!--');
  });
});
