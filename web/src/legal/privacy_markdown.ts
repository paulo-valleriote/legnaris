/**
 * A dependency-free reader for the Markdown subset the privacy policy uses:
 * `#` and `##` headings, paragraphs, `-` bullet lists, `**bold**` and bare
 * `https` URLs in parentheses. It produces plain data; the screen turns that
 * data into elements, so no markup from the source ever reaches the DOM as HTML.
 */

export type MarkdownBlock =
  | { readonly kind: 'heading'; readonly level: 1 | 2; readonly text: string }
  | { readonly kind: 'paragraph'; readonly text: string }
  | { readonly kind: 'list'; readonly items: readonly string[] };

export type InlineToken =
  | { readonly kind: 'text'; readonly text: string }
  | { readonly kind: 'bold'; readonly text: string }
  | { readonly kind: 'link'; readonly href: string };

const HEADING = /^(#{1,2})\s+(.*)$/;
const BULLET = /^-\s+(.*)$/;
const INLINE = /\*\*([^*]+?)\*\*|\((https?:\/\/[^\s()]+)\)/g;

/**
 * parseMarkdown splits the source into blocks. Consecutive text lines join into one
 * paragraph, a blank line ends a block, and a bullet line starts or extends a list.
 */
export function parseMarkdown(source: string): MarkdownBlock[] {
  const blocks: MarkdownBlock[] = [];
  let paragraph: string[] = [];
  let items: string[] = [];

  const flushParagraph = () => {
    if (paragraph.length > 0) {
      blocks.push({ kind: 'paragraph', text: paragraph.join(' ') });
      paragraph = [];
    }
  };
  const flushList = () => {
    if (items.length > 0) {
      blocks.push({ kind: 'list', items });
      items = [];
    }
  };

  for (const raw of source.split(/\r?\n/)) {
    const line = raw.trim();
    const heading = HEADING.exec(line);
    const bullet = BULLET.exec(line);

    if (line === '') {
      flushParagraph();
      flushList();
    } else if (heading) {
      flushParagraph();
      flushList();
      blocks.push({ kind: 'heading', level: heading[1].length as 1 | 2, text: heading[2] });
    } else if (bullet) {
      flushParagraph();
      items.push(bullet[1]);
    } else {
      flushList();
      paragraph.push(line);
    }
  }

  flushParagraph();
  flushList();
  return blocks;
}

/**
 * tokenizeInline splits one line of text into literal text, bold runs and links.
 * An unclosed `**` and a non-http parenthesised value stay literal text.
 */
export function tokenizeInline(text: string): InlineToken[] {
  const tokens: InlineToken[] = [];
  let cursor = 0;

  for (const match of text.matchAll(INLINE)) {
    const start = match.index ?? 0;
    if (start > cursor) {
      tokens.push({ kind: 'text', text: text.slice(cursor, start) });
    }
    if (match[1] !== undefined) {
      tokens.push({ kind: 'bold', text: match[1] });
    } else {
      tokens.push({ kind: 'text', text: '(' });
      tokens.push({ kind: 'link', href: match[2] });
      tokens.push({ kind: 'text', text: ')' });
    }
    cursor = start + match[0].length;
  }

  if (cursor < text.length) {
    tokens.push({ kind: 'text', text: text.slice(cursor) });
  }
  return tokens;
}
