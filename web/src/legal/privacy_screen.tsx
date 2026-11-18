import type { ReactNode } from 'react';
import { message } from '../l10n/strings';
import { PageHeader } from '../ui/page_header';
import { tokenizeInline } from './privacy_markdown';
import type { InlineToken, MarkdownBlock } from './privacy_markdown';
import { policyDocument, policyLocale } from './privacy_policy';
import type { PolicyLocale } from './privacy_policy';
import './privacy.css';

type HeadingTag = 'h1' | 'h2' | 'h3';

interface PrivacyScreenProps {
  readonly locale?: PolicyLocale;
}

interface PrivacyArticleProps {
  readonly blocks: readonly MarkdownBlock[];
  readonly headingLevelOffset?: 0 | 1;
  readonly className?: string;
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

function headingTag(level: 1 | 2, offset: 0 | 1): HeadingTag {
  const shifted = level + offset;
  if (shifted === 1) {
    return 'h1';
  }
  return shifted === 2 ? 'h2' : 'h3';
}

function renderBlock(block: MarkdownBlock, index: number, offset: 0 | 1): ReactNode {
  if (block.kind === 'heading') {
    const Heading = headingTag(block.level, offset);
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

export function currentPolicyLocale(): PolicyLocale {
  return policyLocale(globalThis.navigator?.languages ?? []);
}

export function PrivacyArticle({ blocks, headingLevelOffset = 0, className }: PrivacyArticleProps) {
  const classes = className === undefined ? 'lg-privacy__body' : `lg-privacy__body ${className}`;
  return (
    <article className={classes}>
      {blocks.map((block, index) => renderBlock(block, index, headingLevelOffset))}
    </article>
  );
}

export function PrivacyScreen({ locale }: PrivacyScreenProps = {}) {
  const resolved = locale ?? currentPolicyLocale();
  const { title, body } = policyDocument(resolved);

  return (
    <main className="lg-privacy" lang={resolved}>
      <div className="lg-privacy__inner">
        <PageHeader title={title} backTo="/" backLabel={message('privacy.back')} />
        <PrivacyArticle blocks={body} />
      </div>
    </main>
  );
}
