import { message } from '../l10n/strings';
import { parseMarkdown } from './privacy_markdown';
import type { MarkdownBlock } from './privacy_markdown';
import enSource from './privacy-policy.en.md?raw';
import ptBrSource from './privacy-policy.pt-BR.md?raw';

export type PolicyLocale = 'en' | 'pt-BR';

export interface PolicyDocument {
  readonly title: string;
  readonly body: readonly MarkdownBlock[];
}

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

export function policyDocument(locale: PolicyLocale): PolicyDocument {
  const blocks = parseMarkdown(policyMarkdown(locale));
  const [first, ...rest] = blocks;
  if (first?.kind === 'heading' && first.level === 1) {
    return { title: first.text, body: rest };
  }
  return { title: message('privacy.title'), body: blocks };
}
