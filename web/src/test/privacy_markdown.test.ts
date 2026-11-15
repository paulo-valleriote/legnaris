import { describe, expect, it } from 'vitest';
import { parseMarkdown, tokenizeInline } from '../legal/privacy_markdown';

describe('privacy Markdown reader (LGN-234)', () => {
  it('Happy path: headings, paragraphs and bullets become ordered blocks', () => {
    const blocks = parseMarkdown('# Title\n\nFirst line\ncontinued.\n\n## Part\n\n- one\n- two\n');

    expect(blocks).toEqual([
      { kind: 'heading', level: 1, text: 'Title' },
      { kind: 'paragraph', text: 'First line continued.' },
      { kind: 'heading', level: 2, text: 'Part' },
      { kind: 'list', items: ['one', 'two'] },
    ]);
  });

  it('Edge: empty and whitespace-only sources produce no blocks', () => {
    expect(parseMarkdown('')).toEqual([]);
    expect(parseMarkdown('\n   \n\t\n')).toEqual([]);
  });

  it('Edge: CRLF line endings parse the same as LF', () => {
    expect(parseMarkdown('# A\r\n\r\n- x\r\n')).toEqual([
      { kind: 'heading', level: 1, text: 'A' },
      { kind: 'list', items: ['x'] },
    ]);
  });

  it('Edge: a heading deeper than two levels is kept as paragraph text', () => {
    expect(parseMarkdown('### Deep')).toEqual([{ kind: 'paragraph', text: '### Deep' }]);
  });

  it('Edge: a heading marker without a space is not a heading', () => {
    expect(parseMarkdown('#Title')).toEqual([{ kind: 'paragraph', text: '#Title' }]);
  });

  it('Edge: a paragraph after a list closes the list first', () => {
    expect(parseMarkdown('- a\nafter')).toEqual([
      { kind: 'list', items: ['a'] },
      { kind: 'paragraph', text: 'after' },
    ]);
  });

  it('Edge: a blank line separates two lists', () => {
    expect(parseMarkdown('- a\n\n- b')).toEqual([
      { kind: 'list', items: ['a'] },
      { kind: 'list', items: ['b'] },
    ]);
  });

  it('Happy path: bold and a parenthesised https URL tokenize into bold and link tokens', () => {
    expect(tokenizeInline('Use **this** (https://example.test/x).')).toEqual([
      { kind: 'text', text: 'Use ' },
      { kind: 'bold', text: 'this' },
      { kind: 'text', text: ' ' },
      { kind: 'text', text: '(' },
      { kind: 'link', href: 'https://example.test/x' },
      { kind: 'text', text: ')' },
      { kind: 'text', text: '.' },
    ]);
  });

  it('Edge: an unclosed bold marker stays literal text', () => {
    expect(tokenizeInline('a **b')).toEqual([{ kind: 'text', text: 'a **b' }]);
  });

  it('Edge: an empty bold run stays literal text', () => {
    expect(tokenizeInline('****')).toEqual([{ kind: 'text', text: '****' }]);
  });

  it('Edge: a parenthesised value that is not an http URL stays literal text', () => {
    expect(tokenizeInline('(javascript:alert(1))')).toEqual([
      { kind: 'text', text: '(javascript:alert(1))' },
    ]);
  });

  it('Edge: empty text yields no tokens', () => {
    expect(tokenizeInline('')).toEqual([]);
  });
});
