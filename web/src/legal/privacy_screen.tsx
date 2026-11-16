import type { ReactNode } from 'react';
import { parseMarkdown, tokenizeInline } from './privacy_markdown';
import type { InlineToken, MarkdownBlock } from './privacy_markdown';
import { policyLocale, policyMarkdown } from './privacy_policy';
import type { PolicyLocale } from './privacy_policy';
import './privacy.css';

interface PrivacyScreenProps {
  readonly locale?: PolicyLocale;
}

function renderToken(token: InlineToken, key: number): ReactNode {
  if (token.kind === 'bold') {
    return <strong key={key}>{token.text}</strong>;
  }
  if (token.kind === 'link') {
    return (
      <a key={key} href={token.href} rel="noopener noreferrer">
        {token.href}
      </a>
    );
  }
  return <span key={key}>{token.text}</span>;
}

function renderInline(text: string): ReactNode[] {
  return tokenizeInline(text).map(renderToken);
}

function renderBlock(block: MarkdownBlock, index: number): ReactNode {
  if (block.kind === 'heading') {
    const Heading = block.level === 1 ? 'h1' : 'h2';
    return <Heading key={index}>{renderInline(block.text)}</Heading>;
  }
  if (block.kind === 'list') {
    return (
      <ul key={index}>
        {block.items.map((item, itemIndex) => (
          <li key={itemIndex}>{renderInline(item)}</li>
        ))}
      </ul>
    );
  }
  return <p key={index}>{renderInline(block.text)}</p>;
}

function currentLocale(): PolicyLocale {
  return policyLocale(globalThis.navigator?.languages ?? []);
}

export function PrivacyScreen({ locale }: PrivacyScreenProps = {}) {
  const resolved = locale ?? currentLocale();
  const blocks = parseMarkdown(policyMarkdown(resolved));

  return (
    <main className="lg-privacy" lang={resolved}>
      <article className="lg-privacy__body">{blocks.map(renderBlock)}</article>
    </main>
  );
}
