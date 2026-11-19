import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import { message } from '../l10n/strings';
import { renderApp } from './render';

const here = path.dirname(fileURLToPath(import.meta.url));
const srcDir = path.resolve(here, '..');

const REMOVED_PANEL_NAMES = ['right_panel', '.lg-panel', '--lg-size-panel-', 'SHELL_NARROW_QUERY'];

const shellCss = readFileSync(path.resolve(here, '../shell/shell.css'), 'utf8');

function sourceFiles(dir: string): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'test') {
      continue;
    }
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      found.push(...sourceFiles(full));
    } else if (/\.(ts|tsx|css)$/.test(entry.name)) {
      found.push(full);
    }
  }
  return found;
}

describe('application shell', () => {
  it('Happy path: renders the navigation then the content, with no third region', () => {
    renderApp('/');

    const nav = screen.getByRole('navigation', { name: 'Primary' });
    const main = screen.getByRole('main');
    const follows = nav.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING;

    expect(follows).toBeTruthy();
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument();
  });

  it('Edge: the signed-in app is always dark, so the navigation carries no theme control', () => {
    renderApp('/');

    const nav = screen.getByRole('navigation', { name: 'Primary' });

    expect(within(nav).queryByRole('button', { name: /theme/i })).not.toBeInTheDocument();
    expect(nav.querySelector('.lg-nav__footer')).toBeInTheDocument();
  });

  it('Happy path: the content region shows the template catalog', async () => {
    renderApp('/');

    const main = screen.getByRole('main');
    expect(within(main).getByTestId('catalog')).toBeInTheDocument();
    await waitFor(() =>
      expect(within(main).getByText(message('catalog.empty'))).toBeInTheDocument(),
    );
  });

  it('Edge: the content region renders no placeholder rows or demo data', async () => {
    renderApp('/');

    await waitFor(() => expect(screen.getByText(message('catalog.empty'))).toBeInTheDocument());

    expect(screen.queryAllByRole('row')).toHaveLength(0);
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
    expect(screen.queryAllByRole('table')).toHaveLength(0);
  });

  it('Edge: no source references the removed contextual panel', () => {
    const files = sourceFiles(srcDir);
    expect(files.length).toBeGreaterThan(0);

    for (const file of files) {
      const text = readFileSync(file, 'utf8');
      for (const name of REMOVED_PANEL_NAMES) {
        expect(text.includes(name), `${file} must not reference ${name}`).toBe(false);
      }
    }
  });

  it('Edge: the brand carries no decorative accent hairline', () => {
    expect(shellCss).not.toContain('.lg-nav__brand::after');
  });

  it('Edge: the shell adds no second reduced-motion rule', () => {
    expect(shellCss).not.toContain('prefers-reduced-motion');
  });
});
