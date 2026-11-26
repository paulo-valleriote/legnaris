import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const css = readFileSync(path.resolve(here, '../sheet/preset/compose/compose.css'), 'utf8');

function ruleFor(selector: string, block: string): string {
  const start = block.indexOf(selector);
  expect(start).toBeGreaterThanOrEqual(0);
  return block.slice(start, block.indexOf('}', start));
}

describe('compose inline edit touch targets', () => {
  it.each([
    {
      name: 'Happy path: the temporary hit points value has a control-sized hit area',
      selector: '.lg-compose-hp__temp .lg-compose-hp__number.lg-inline-edit',
    },
    {
      name: 'Happy path: an ability score has a control-sized hit area',
      selector: '.lg-compose-ability .lg-compose-ability__score {',
    },
  ])('$name', ({ selector }) => {
    const rule = ruleFor(
      selector,
      css.slice(css.lastIndexOf('\n.lg-compose-hp__temp .lg-compose-hp__number.lg-inline-edit,')),
    );
    expect(rule).toMatch(/min-inline-size:\s*var\(--lg-size-control\)/);
    expect(rule).toMatch(/min-block-size:\s*var\(--lg-size-control-compact\)/);
  });

  it('Edge: on a coarse pointer both targets grow to the 44px control size', () => {
    const coarse = css.slice(css.lastIndexOf('@media (pointer: coarse)'));
    expect(coarse).toContain('.lg-compose-ability .lg-compose-ability__score');
    expect(coarse).toMatch(/min-inline-size:\s*var\(--lg-size-control\)/);
    expect(coarse).toMatch(/min-block-size:\s*var\(--lg-size-control\)/);
  });
});
