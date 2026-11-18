import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const read = (relative: string) => readFileSync(path.resolve(here, '..', relative), 'utf8');

function ruleBody(css: string, selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\>]/g, '\\$&');
  const match = new RegExp(`(?:^|\\n)\\s*${escaped}\\s*\\{([^}]*)\\}`).exec(css);
  return match?.[1] ?? '';
}

describe('page rhythm under the page title', () => {
  const tests: { name: string; file: string; selector: string; expected: RegExp }[] = [
    {
      name: 'Happy path: a standalone page header leaves 32 below it',
      file: 'ui/ui.css',
      selector: '.lg-page-header',
      expected: /margin-bottom:\s*var\(--lg-space-32\)/,
    },
    {
      name: 'Edge: the catalog column gap already spaces the header',
      file: 'catalog/catalog.css',
      selector: '.lg-catalog > .lg-page-header',
      expected: /margin-bottom:\s*0/,
    },
    {
      name: 'Edge: the generate column gap already spaces the header',
      file: 'generate/generate.css',
      selector: '.lg-generate > .lg-page-header',
      expected: /margin-bottom:\s*0/,
    },
    {
      name: 'Edge: the sheet column gap of 16 is topped up to 32',
      file: 'sheet/sheet.css',
      selector: '.lg-sheet > .lg-page-header',
      expected: /margin-bottom:\s*var\(--lg-space-16\)/,
    },
    {
      name: 'Happy path: the Your sheet name uses the list item scale',
      file: 'sheet/your_sheet.css',
      selector: '.lg-yoursheet__name',
      expected: /font-size:\s*var\(--lg-type-18\)/,
    },
  ];

  for (const testCase of tests) {
    it(testCase.name, () => {
      expect(ruleBody(read(testCase.file), testCase.selector)).toMatch(testCase.expected);
    });
  }

  it('Edge: a missing selector reads as an empty rule', () => {
    expect(ruleBody('.lg-a { color: red; }', '.lg-b')).toBe('');
  });

  it('Happy path: on narrow screens the nav links flow inline with the brand', () => {
    const shell = read('shell/shell.css');
    const narrow = shell.slice(shell.indexOf('@media (max-width: 39.99rem)'));
    expect(ruleBody(narrow, '.lg-nav__group')).toMatch(/display:\s*contents/);
  });
});
