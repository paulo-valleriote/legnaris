import enSource from './privacy-policy.en.md?raw';
import ptBrSource from './privacy-policy.pt-BR.md?raw';

/** The languages the policy is published in; the app has no other locale. */
export type PolicyLocale = 'en' | 'pt-BR';

const SOURCES: Readonly<Record<PolicyLocale, string>> = {
  en: enSource,
  'pt-BR': ptBrSource,
};

/**
 * policyLocale picks Portuguese only when the first preferred language is Portuguese,
 * and English otherwise, including for an empty preference list.
 */
export function policyLocale(languages: readonly string[]): PolicyLocale {
  const first = languages.find((language) => language.trim() !== '') ?? '';
  return /^pt(-|$)/i.test(first.trim()) ? 'pt-BR' : 'en';
}

/**
 * stripLeadingComment removes the authoring header that opens the policy source, so
 * the reader sees the document and not the editorial notes above it.
 */
export function stripLeadingComment(source: string): string {
  return source.replace(/^\s*<!--[\s\S]*?-->/, '');
}

/** policyMarkdown returns the policy text for a locale with its leading comment removed. */
export function policyMarkdown(locale: PolicyLocale): string {
  return stripLeadingComment(SOURCES[locale]);
}
