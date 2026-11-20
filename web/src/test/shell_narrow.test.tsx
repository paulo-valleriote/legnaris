import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import { renderApp } from './render';
import { setViewportWidth } from './match_media';

const here = path.dirname(fileURLToPath(import.meta.url));
const NARROW_QUERY = '@media (max-width: 39.99rem)';

function readCss(relative: string): string {
  return readFileSync(path.resolve(here, relative), 'utf8');
}

function mediaBlock(css: string, query: string): string {
  const start = css.indexOf(query);
  if (start === -1) {
    return '';
  }
  const open = css.indexOf('{', start);
  let depth = 0;
  for (let index = open; index < css.length; index += 1) {
    if (css[index] === '{') {
      depth += 1;
    } else if (css[index] === '}') {
      depth -= 1;
      if (depth === 0) {
        return css.slice(open + 1, index);
      }
    }
  }
  return '';
}

function rule(block: string, selector: string): string {
  const escaped = selector.replace(/[.[\]()*]/g, '\\$&');
  return block.match(new RegExp(`(?:^|\\n)\\s*${escaped}\\s*\\{([^}]*)\\}`))?.[1] ?? '';
}

describe('narrow shell: the navigation becomes a compact top bar', () => {
  const shellCss = readCss('../shell/shell.css');
  const narrow = mediaBlock(shellCss, NARROW_QUERY);

  it('Error: a missing narrow block would leave the 520 px desktop grid on a phone', () => {
    expect(narrow).not.toBe('');
  });

  it('Edge: the breakpoint is a relative unit just under 40rem, so 40rem itself stays desktop', () => {
    expect(shellCss).toContain(NARROW_QUERY);
    expect(shellCss).not.toMatch(/@media \(max-width: \d+px\)/);
  });

  it('Edge: the grid collapses to one shrinkable column with the bar above the content', () => {
    const grid = rule(narrow, '.lg-shell');

    expect(grid).toContain('grid-template-columns: minmax(0, 1fr)');
    expect(grid).toContain('grid-template-rows: auto minmax(0, 1fr)');
    expect(grid).not.toContain('--lg-size-content-min');
    expect(grid).not.toContain('--lg-size-nav-min');
  });

  it('Edge: the navigation is one bar on 16 px gutters with the menu trigger at the trailing edge', () => {
    const nav = rule(narrow, '.lg-nav');

    expect(nav).toContain('var(--lg-space-16)');
    expect(nav).toContain('position: relative');
    expect(nav).not.toContain('flex-flow: row wrap');
  });

  it('Edge: the navigation destinations sit behind the menu trigger, not in the bar', () => {
    expect(rule(narrow, '.lg-nav__menu')).toContain('position: absolute');
    expect(rule(narrow, '.lg-nav__menu-toggle')).toContain('margin-left: auto');
  });

  it('Edge: every navigation control keeps the control height', () => {
    expect(rule(shellCss, '.lg-nav__link')).toContain('min-height: var(--lg-size-control)');
    expect(rule(shellCss, '.lg-nav__action')).toContain('min-height: var(--lg-size-control)');
  });

  it('Edge: the content takes the full width on 16 px gutters', () => {
    const content = rule(narrow, '.lg-content');

    expect(content).toContain('var(--lg-space-16)');
    expect(content).not.toContain('var(--lg-space-48)');
  });

  it('Edge: the top bar hides only the brand copy, so every menu entry keeps its label', () => {
    expect(rule(narrow, '.lg-nav__brand-text')).toContain('display: none');
    expect(narrow).not.toMatch(
      /\.lg-nav__group-label,[^}]*\.lg-nav__icon,[^}]*\.lg-nav__chevron[^}]*display: none/,
    );
  });

  it('Edge: the desktop grid keeps the 200-240 px navigation and the 320 px content minimum', () => {
    const grid = rule(shellCss, '.lg-shell');

    expect(grid).toContain('minmax(var(--lg-size-nav-min), var(--lg-size-nav-max))');
    expect(grid).toContain('minmax(var(--lg-size-content-min), 1fr)');
  });

  it('Edge: the navigation controls are the control height at every width and pointer', () => {
    expect(rule(shellCss, '.lg-nav__link')).toContain('min-height: var(--lg-size-control)');
    expect(rule(shellCss, '.lg-nav__action')).toContain('min-height: var(--lg-size-control)');
    expect(shellCss).not.toContain('pointer: coarse');
  });

  it.each([320, 390, 639])(
    'Happy path: at %ipx the navigation precedes the content and both regions remain',
    (width) => {
      setViewportWidth(width);
      renderApp('/');

      const nav = screen.getByRole('navigation', { name: 'Primary' });
      const main = screen.getByRole('main');

      expect(nav.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(screen.queryByRole('complementary')).not.toBeInTheDocument();
      expect(within(nav).getByRole('button', { name: 'Menu' })).toBeInTheDocument();
    },
  );
});

describe('narrow sheet: gutters, footer and the live region fit a phone', () => {
  const sheetCss = readCss('../sheet/sheet.css');
  const narrow = mediaBlock(sheetCss, NARROW_QUERY);

  it('Error: a missing narrow block would keep the 24 px gutters and footer padding', () => {
    expect(narrow).not.toBe('');
  });

  it('Edge: the editor children and the sticky footer take 16 px gutters', () => {
    expect(narrow).toMatch(
      /\.lg-sheet--editor\s*>[^{]*\{[^}]*padding-inline: var\(--lg-space-16\)/,
    );
    expect(rule(narrow, '.lg-sheet__footer')).toContain('padding-inline: var(--lg-space-16)');
  });

  it('Edge: the visually hidden live region is never sized by the editor child rule', () => {
    const selectors = sheetCss.match(/\.lg-sheet--editor\s*>[^{]*\{/g) ?? [];

    expect(selectors.length).toBeGreaterThan(0);
    for (const selector of selectors) {
      expect(selector).toContain(':not(.lg-visually-hidden)');
    }
  });

  it('Edge: the sticky footer stays pinned to the content end', () => {
    const footer = rule(sheetCss, '.lg-sheet__footer');

    expect(footer).toContain('position: sticky');
    expect(footer).toContain('bottom: 0');
    expect(footer).toContain('flex-wrap: wrap');
  });
});

describe('toggle hit area', () => {
  const engineCss = readCss('../engine/engine.css');

  it('Edge: the native switch input is at least the control height, drawn inside a transparent border', () => {
    const toggle = rule(engineCss, '.lg-field__toggle');

    expect(toggle).toContain('height: var(--lg-size-control)');
    expect(toggle).toContain(
      'border: calc((var(--lg-size-control) - var(--lg-space-32)) / 2) solid transparent',
    );
    expect(toggle).toContain('background-clip: padding-box');
    expect(toggle).toContain('box-sizing: border-box');
  });

  it('Edge: the label keeps a control-sized hit area around the switch', () => {
    const label = rule(engineCss, '.lg-field--toggle');

    expect(label).toContain('min-width: var(--lg-size-control)');
    expect(label).toContain('min-height: var(--lg-size-control)');
  });
});
