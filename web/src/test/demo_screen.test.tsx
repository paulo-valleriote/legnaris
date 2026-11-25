import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DECLARED_ROUTES, DEMO_PATH, LANDING_PATH, SRD_PATH } from '../app/routes';
import { message } from '../l10n/strings';
import { signUpCtaPath } from '../landing/start_intent';
import { renderApp } from './render';
import { rollName } from './dice_names';

const here = path.dirname(fileURLToPath(import.meta.url));
const FIFTH_SIGN_UP_PATH = signUpCtaPath('fifth');
const fetchSpy = vi.fn(() => Promise.reject(new Error('the demo page must stay offline')));

beforeEach(() => {
  vi.stubGlobal('fetch', fetchSpy);
});

afterEach(() => {
  vi.unstubAllGlobals();
  fetchSpy.mockClear();
});

async function visitDemo(session: 'authenticated' | 'unauthenticated' = 'unauthenticated') {
  const view = renderApp(DEMO_PATH, { session });
  await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());
  await waitFor(() => expect(screen.getByTestId('landing-demo-sheet')).toBeInTheDocument());
  return view;
}

describe('demo page', () => {
  it('Happy path: the demo route is declared public with a deep link', () => {
    const route = DECLARED_ROUTES.find((declared) => declared.path === DEMO_PATH);

    expect(route).toMatchObject({ audience: 'public', deepLink: true, notFound: false });
  });

  it('Happy path: a visitor reaches the live sheet on its own page', async () => {
    await visitDemo();

    expect(
      screen.getByRole('heading', { level: 1, name: message('demo.title') }),
    ).toBeInTheDocument();
    expect(screen.getByText(message('demo.lead'))).toBeInTheDocument();
    expect(screen.getByTestId('landing-demo-sheet')).toBeInTheDocument();
    expect(screen.queryByTestId('landing')).not.toBeInTheDocument();
  });

  it('Happy path: the sample character sits below the page heading', async () => {
    await visitDemo();

    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.getByRole('heading', { level: 2, name: 'Ysolde Ashgrave' })).toBeInTheDocument();
  });

  it('Happy path: Back to home returns to the landing', async () => {
    await visitDemo();

    expect(screen.getByRole('link', { name: message('demo.backHome') })).toHaveAttribute(
      'href',
      LANDING_PATH,
    );
  });

  it('Happy path: opening the demo page makes no network request', async () => {
    await visitDemo();

    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: a signed-in visitor also reaches the public demo page', async () => {
    await visitDemo('authenticated');

    expect(screen.getByTestId('landing-demo-sheet')).toBeInTheDocument();
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
  });

  it.each([
    {
      name: 'Edge: before any interaction the end CTA invites a passive reader',
      roll: false,
      key: 'demo.endIdle',
    },
    {
      name: 'Happy path: after a roll the end CTA acknowledges the play',
      roll: true,
      key: 'landing.demoCtaText',
    },
  ] as const)('$name', async ({ roll, key }) => {
    const user = userEvent.setup();
    await visitDemo();
    if (roll) {
      await user.click(screen.getByRole('button', { name: rollName('Wisdom modifier') }));
    }

    const cta = screen.getByTestId('landing-demo-cta');
    expect(cta).toHaveTextContent(message(key));
    expect(cta.classList.contains('lg-demo__end--played')).toBe(roll);
    expect(
      within(cta).getByRole('link', { name: message('landing.demoCtaAction') }),
    ).toHaveAttribute('href', FIFTH_SIGN_UP_PATH);
  });

  it('Happy path: the header offers sign-up next to the way back home', async () => {
    await visitDemo();

    const actions = screen.getByRole('navigation', { name: message('demo.actionsLabel') });
    expect(within(actions).getByRole('link', { name: message('landing.create') })).toHaveAttribute(
      'href',
      FIFTH_SIGN_UP_PATH,
    );
  });

  it('Happy path: rolling a die reveals the sign-up CTA and keeps the demo state', async () => {
    const user = userEvent.setup();
    await visitDemo();
    const modifier = screen.getByRole('button', { name: rollName('Wisdom modifier') });

    expect(modifier).toHaveTextContent('+3');
    await user.click(modifier);

    const cta = screen.getByTestId('landing-demo-cta');
    expect(
      within(cta).getByRole('link', { name: message('landing.demoCtaAction') }),
    ).toHaveAttribute('href', FIFTH_SIGN_UP_PATH);
    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Edge: the mobile sticky CTA stays hidden and inert before any interaction', async () => {
    await visitDemo();

    const sticky = screen.getByTestId('landing-sticky-cta');
    expect(sticky).not.toHaveClass('lg-landing__sticky--shown');
    expect(sticky).toHaveAttribute('inert');
  });

  it('Happy path: after a roll the mobile sticky CTA appears with the fifth start intent', async () => {
    const user = userEvent.setup();
    await visitDemo();
    await user.click(screen.getByRole('button', { name: rollName('Wisdom modifier') }));

    expect(screen.getByTestId('landing-sticky-cta')).toHaveClass('lg-landing__sticky--shown');
    expect(
      within(screen.getByTestId('landing-sticky-cta')).getByRole('link', {
        name: message('landing.create'),
      }),
    ).toHaveAttribute('href', FIFTH_SIGN_UP_PATH);
  });

  it('Happy path: the demo footer links to the SRD attribution', async () => {
    await visitDemo();

    expect(
      within(screen.getByRole('contentinfo')).getByRole('link', {
        name: message('landing.footer.srd'),
      }),
    ).toHaveAttribute('href', SRD_PATH);
  });

  it('Edge: the demo page is its own lazy chunk over the lazily loaded sheet', () => {
    const router = readFileSync(path.resolve(here, '../app/router.tsx'), 'utf8');
    const demo = readFileSync(path.resolve(here, '../landing/demo_screen.tsx'), 'utf8');
    const landing = readFileSync(path.resolve(here, '../landing/landing_screen.tsx'), 'utf8');

    expect(router).toContain("lazy(() => import('../landing/demo_screen'))");
    expect(demo).toContain("lazy(() => import('./fifth_demo_sheet'))");
    expect(demo).not.toContain("from './fifth_demo_sheet'");
    expect(landing).not.toContain('fifth_demo_sheet');
  });
});
