import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { renderApp } from './render';
import { setViewportWidth } from './match_media';

const here = path.dirname(fileURLToPath(import.meta.url));
const shellCss = readFileSync(path.resolve(here, '../shell/shell.css'), 'utf8');

function narrowBlock(): string {
  const start = shellCss.indexOf('@media (max-width: 39.99rem)');
  const open = shellCss.indexOf('{', start);
  let depth = 0;
  for (let index = open; index < shellCss.length; index += 1) {
    if (shellCss[index] === '{') {
      depth += 1;
    } else if (shellCss[index] === '}') {
      depth -= 1;
      if (depth === 0) {
        return shellCss.slice(open + 1, index);
      }
    }
  }
  return '';
}

function cssRule(block: string, selector: string): string {
  const escaped = selector.replace(/[.[\]()*]/g, '\\$&');
  return block.match(new RegExp(`(?:^|\\n)\\s*${escaped}\\s*\\{([^}]*)\\}`))?.[1] ?? '';
}

async function openMenuAtPhone() {
  setViewportWidth(390);
  const user = userEvent.setup();
  renderApp('/');
  const trigger = screen.getByRole('button', { name: message('nav.menu') });
  await user.click(trigger);
  const panelId = trigger.getAttribute('aria-controls') ?? '';
  const panel = document.getElementById(panelId);
  return { user, trigger, panel, panelId };
}

describe('phone navigation menu', () => {
  it('Edge: Escape closes the menu and returns focus to the menu button', async () => {
    const { user, trigger, panel } = await openMenuAtPhone();

    await user.keyboard('{Escape}');

    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(panel).toHaveAttribute('hidden');
    expect(trigger).toHaveFocus();
  });

  it('Edge: an outside tap closes the menu and returns focus to the menu button', async () => {
    const { user, trigger, panel } = await openMenuAtPhone();

    await user.click(document.body);

    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(panel).toHaveAttribute('hidden');
    expect(trigger).toHaveFocus();
  });

  it('Edge: navigating from the menu closes it', async () => {
    const { user, trigger, panel } = await openMenuAtPhone();

    await user.click(
      within(panel as HTMLElement).getByRole('link', { name: message('nav.catalog') }),
    );

    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(panel).toHaveAttribute('hidden');
  });

  it('Edge: resizing from phone to desktop with the menu open closes it and restores the sidebar', async () => {
    await openMenuAtPhone();

    act(() => setViewportWidth(1280));

    expect(screen.queryByRole('button', { name: message('nav.menu') })).not.toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: message('nav.primaryLabel') });
    expect(within(nav).getByRole('link', { name: message('nav.catalog') })).toBeVisible();
    expect(within(nav).getByRole('button', { name: message('nav.logout') })).toBeVisible();
  });

  it('Error: a menu without a labelled trigger or a controlled panel is unusable', async () => {
    setViewportWidth(390);
    const user = userEvent.setup();
    renderApp('/');

    const trigger = screen.getByRole('button', { name: message('nav.menu') });

    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    const panelId = trigger.getAttribute('aria-controls');
    expect(panelId).not.toBeNull();
    expect(document.getElementById(panelId ?? '')).toHaveAttribute('hidden');

    await user.click(trigger);

    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(document.getElementById(panelId ?? '')).not.toHaveAttribute('hidden');
  });

  it('Happy path: the menu carries every destination and marks the current page', async () => {
    const { trigger, panel } = await openMenuAtPhone();

    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    const menu = within(panel as HTMLElement);
    for (const key of [
      'nav.catalog',
      'nav.ownSheet',
      'nav.plans',
      'nav.campaigns',
      'nav.account',
    ] as const) {
      expect(menu.getByRole('link', { name: message(key) })).toBeVisible();
    }
    expect(menu.getByRole('button', { name: message('nav.logout') })).toBeVisible();
    expect(menu.getByRole('link', { name: message('nav.catalog') })).toHaveAttribute(
      'aria-current',
      'page',
    );
  });

  it('Happy path: opening the menu moves focus into it', async () => {
    const { panel } = await openMenuAtPhone();

    expect(
      within(panel as HTMLElement).getByRole('link', { name: message('nav.catalog') }),
    ).toHaveFocus();
  });

  it('Happy path: the desktop sidebar keeps its destinations and draws no menu button', () => {
    renderApp('/');

    const nav = screen.getByRole('navigation', { name: message('nav.primaryLabel') });

    expect(
      within(nav).queryByRole('button', { name: message('nav.menu') }),
    ).not.toBeInTheDocument();
    expect(within(nav).getByRole('link', { name: message('nav.catalog') })).toBeVisible();
    expect(within(nav).getByRole('button', { name: message('nav.logout') })).toBeVisible();
  });

  it('Edge: the phone trigger is a 44 px control and the panel overlays without moving content', () => {
    const narrow = narrowBlock();
    const trigger = cssRule(narrow, '.lg-nav__menu-toggle');
    const panel = cssRule(narrow, '.lg-nav__menu');

    expect(trigger).toContain('width: var(--lg-size-control)');
    expect(trigger).toContain('height: var(--lg-size-control)');
    expect(panel).toContain('position: absolute');
    expect(cssRule(narrow, '.lg-nav__menu[hidden]')).toContain('display: none');
  });
});
