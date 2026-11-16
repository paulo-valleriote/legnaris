import enSource from './privacy-policy.en.md?raw';
import ptBrSource from './privacy-policy.pt-BR.md?raw';

export type PolicyLocale = 'en' | 'pt-BR';

const SOURCES: Readonly<Record<PolicyLocale, string>> = {
  en: enSource,
  'pt-BR': ptBrSource,
};

export function policyLocale(languages: readonly string[]): PolicyLocale {
  const first = languages.find((language) => language.trim() !== '') ?? '';
  return /^pt(-|$)/i.test(first.trim()) ? 'pt-BR' : 'en';
}

export function stripLeadingComment(source: string): string {
  return source.replace(/^\s*<!--[\s\S]*?-->/, '');
}

export function policyMarkdown(locale: PolicyLocale): string {
  return stripLeadingComment(SOURCES[locale]);
}
