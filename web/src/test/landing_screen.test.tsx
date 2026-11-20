import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message, messageWith } from '../l10n/strings';
import { PRIVACY_PATH } from '../app/routes';
import { signUpCtaPath } from '../landing/start_intent';
import { MIGHT_MAX, MIGHT_MIN } from '../landing/cascade';
import { renderApp } from './render';
import { rollName } from './dice_names';

const here = path.dirname(fileURLToPath(import.meta.url));

async function visit(path: string) {
  const view = renderApp(path, { session: 'unauthenticated' });
  await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());
  return view;
}

async function visitDemo() {
  const view = await visit('/');
  await waitFor(() => expect(screen.getByTestId('landing-demo-sheet')).toBeInTheDocument());
  return view;
}

const FANTASY_SIGN_UP_PATH = signUpCtaPath('fantasy');

describe('landing screen', () => {
  it('Happy path: a visitor on the root sees the landing, not the sign-in form or the catalog', async () => {
    await visit('/');

    expect(screen.getByTestId('landing')).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { level: 1, name: message('landing.heroTitle') }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
    expect(screen.queryByLabelText(message('auth.passwordLabel'))).not.toBeInTheDocument();
  });

  it('Happy path: the hero actions lead to the registration and sign-in paths', async () => {
    await visit('/');

    const hero = screen.getByRole('banner');
    expect(within(hero).getByRole('link', { name: message('landing.create') })).toHaveAttribute(
      'href',
      '/sign-up?start=fantasy',
    );
    expect(within(hero).getByRole('link', { name: message('auth.submitLogin') })).toHaveAttribute(
      'href',
      '/sign-in',
    );
  });

  it('Happy path: the page carries the landmarks', async () => {
    await visit('/');

    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
  });

  it('Edge: the hero carries no in-page navigation, only the brand and the two actions', async () => {
    await visit('/');

    const hero = screen.getByRole('banner');
    expect(within(hero).queryByRole('navigation')).not.toBeInTheDocument();
    expect(
      within(hero)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href')),
    ).toEqual(['/', '/sign-in', '/sign-up?start=fantasy']);
  });

  it('Edge: the footer carries only the closing call to action and the legal line, no second photograph', async () => {
    await visit('/');

    const footer = screen.getByRole('contentinfo');
    expect(footer.querySelector('img')).toBeNull();
    expect(footer.lastElementChild).toHaveClass('lg-landing__legal');
    expect(screen.getByTestId('landing').querySelectorAll('img')).toHaveLength(1);
  });

  it('Happy path: the footer links to the public privacy policy', async () => {
    await visit('/');

    const footer = screen.getByRole('contentinfo');
    expect(
      within(footer).getByRole('link', { name: message('landing.footer.privacy') }),
    ).toHaveAttribute('href', PRIVACY_PATH);
  });

  it('Happy path: the browser chrome takes the stage colour while mounted and is restored after', async () => {
    document.documentElement.style.setProperty('--lg-color-stage', '#061516');
    const view = await visit('/');

    const metas = () => document.head.querySelectorAll('meta[name="theme-color"]');
    expect(metas()[0]).toHaveAttribute('content', '#061516');
    expect(metas()[0]).not.toHaveAttribute('media');

    view.unmount();

    expect(metas()).toHaveLength(0);
    document.documentElement.style.removeProperty('--lg-color-stage');
  });

  it('Edge: with no resolvable stage colour no theme-color entry is added', async () => {
    document.documentElement.style.removeProperty('--lg-color-stage');
    await visit('/');

    expect(document.head.querySelector('meta[name="theme-color"]')).toBeNull();
  });

  it('Edge: the hero photograph is decorative and has no accessible name', async () => {
    await visit('/');

    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(screen.getByTestId('landing').querySelector('img')).toHaveAttribute('alt', '');
  });

  it('Happy path: every question is a disclosure that opens onto its answer', async () => {
    const user = userEvent.setup();
    await visit('/');

    const question = screen.getByText(message('landing.faq1Question'));
    const item = question.closest('details');
    expect(item).not.toBeNull();
    expect(item).not.toHaveAttribute('open');
    expect(item!.querySelector('.lg-landing__faq-chevron path')).toHaveAttribute(
      'd',
      'm6 9 6 6 6-6',
    );

    await user.click(question);

    expect(item).toHaveAttribute('open');
    expect(screen.getByText(message('landing.faq1Answer'))).toBeVisible();
    expect(screen.getAllByRole('group').length).toBeGreaterThanOrEqual(1);
  });

  it('Happy path: the demo stepper uses the shared minus and plus glyphs', async () => {
    await visit('/');

    const down = screen.getByRole('button', { name: message('landing.demoBaseDown') });
    const up = screen.getByRole('button', { name: message('landing.demoBaseUp') });

    expect(down.querySelector('svg path')).toHaveAttribute('d', 'M5 12h14');
    expect(up.querySelector('svg path')).toHaveAttribute('d', 'M12 5v14');
  });

  it('Happy path: the sign-up path opens the registration mode', async () => {
    await visit('/sign-up');

    expect(screen.getByLabelText(message('auth.usernameLabel'))).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: message('auth.submitRegister') }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId('landing')).not.toBeInTheDocument();
  });

  it('Happy path: the sign-in path opens the credential form in sign-in mode', async () => {
    await visit('/sign-in');

    expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument();
    expect(screen.queryByLabelText(message('auth.usernameLabel'))).not.toBeInTheDocument();
  });

  it('Edge: an unknown path without a session lands on the sign-in form, not the landing', async () => {
    await visit('/no-such-page');

    expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument();
    expect(screen.queryByTestId('landing')).not.toBeInTheDocument();
  });

  it('Edge: a signed-in person on an auth path is handed to the catalog', () => {
    renderApp('/sign-in', { session: 'authenticated' });

    expect(screen.getByTestId('catalog')).toBeInTheDocument();
    expect(screen.queryByTestId('landing')).not.toBeInTheDocument();
  });

  it('Edge: a signed-in person on the root never sees the landing', () => {
    renderApp('/', { session: 'authenticated' });

    expect(screen.getByTestId('catalog')).toBeInTheDocument();
    expect(screen.queryByTestId('landing')).not.toBeInTheDocument();
  });
});

describe('landing recalculation example', () => {
  function demo() {
    return within(screen.getByRole('figure', { name: message('landing.demoLabel') }));
  }

  function demoOutputs(): HTMLOutputElement[] {
    return [
      ...screen
        .getByRole('figure', { name: message('landing.demoLabel') })
        .querySelectorAll('output'),
    ];
  }

  function demoValues(): (string | null)[] {
    return demoOutputs().map((node) => node.textContent);
  }

  it('Happy path: raising the base score recalculates every derived value', async () => {
    const user = userEvent.setup();
    await visit('/');

    await user.click(demo().getByRole('button', { name: message('landing.demoBaseUp') }));
    await user.click(demo().getByRole('button', { name: message('landing.demoBaseUp') }));

    expect(demoValues()).toEqual(['16', '+3', '+6', '112']);
  });

  it('Edge: one polite live region announces the change and the values are not live', async () => {
    const user = userEvent.setup();
    await visit('/');
    const live = demo().getByTestId('demo-live');

    expect(live).toHaveAttribute('aria-live', 'polite');
    expect(live).toHaveTextContent(
      messageWith('landing.demoChange', {
        might: 14,
        modifier: '+2',
        climbing: '+5',
        carry: 98,
      }),
    );

    for (const output of demoOutputs()) {
      expect(output).toHaveAttribute('aria-live', 'off');
    }

    await user.click(demo().getByRole('button', { name: message('landing.demoBaseUp') }));

    expect(live).toHaveTextContent(
      messageWith('landing.demoChange', {
        might: 15,
        modifier: '+2',
        climbing: '+5',
        carry: 105,
      }),
    );
  });

  it('Edge: the lower bound disables the decrease control and keeps the value', async () => {
    const user = userEvent.setup();
    await visit('/');
    const down = demo().getByRole('button', { name: message('landing.demoBaseDown') });

    for (let index = 0; index < MIGHT_MAX; index += 1) {
      if (!down.hasAttribute('disabled')) {
        await user.click(down);
      }
    }

    expect(down).toBeDisabled();
    expect(demoValues()[0]).toBe(String(MIGHT_MIN));
    expect(demoValues()[1]).toBe('−5');
  });

  it('Edge: the upper bound disables the increase control and keeps the value', async () => {
    const user = userEvent.setup();
    await visit('/');
    const up = demo().getByRole('button', { name: message('landing.demoBaseUp') });

    for (let index = 0; index < MIGHT_MAX; index += 1) {
      if (!up.hasAttribute('disabled')) {
        await user.click(up);
      }
    }

    expect(up).toBeDisabled();
    expect(demoValues()[0]).toBe(String(MIGHT_MAX));
  });

  it('Edge: reaching the upper bound swaps the caption for a remark and leaving restores it', async () => {
    const user = userEvent.setup();
    await visit('/');
    const up = demo().getByRole('button', { name: message('landing.demoBaseUp') });

    for (let index = 0; index < MIGHT_MAX; index += 1) {
      if (!up.hasAttribute('disabled')) {
        await user.click(up);
      }
    }
    expect(demo().getByText(message('landing.demoNoteMax'))).toBeInTheDocument();
    expect(demo().queryByText(message('landing.demoNote'))).not.toBeInTheDocument();

    await user.click(demo().getByRole('button', { name: message('landing.demoBaseDown') }));
    expect(demo().getByText(message('landing.demoNote'))).toBeInTheDocument();
  });

  it('Edge: reaching the lower bound swaps the caption for a remark', async () => {
    const user = userEvent.setup();
    await visit('/');
    const down = demo().getByRole('button', { name: message('landing.demoBaseDown') });

    for (let index = 0; index < MIGHT_MAX; index += 1) {
      if (!down.hasAttribute('disabled')) {
        await user.click(down);
      }
    }

    expect(demo().getByText(message('landing.demoNoteMin'))).toBeInTheDocument();
  });

  it('Edge: only the rows whose value changed replay their heat line (14 to 15 moves just the carry limit)', async () => {
    const user = userEvent.setup();
    await visit('/');
    const heat = () => Array.from(document.querySelectorAll('.lg-landing__demo-heat'));
    const before = heat();

    await user.click(demo().getByRole('button', { name: message('landing.demoBaseUp') }));

    const after = heat();
    expect(after).toHaveLength(3);
    expect(after[0]).toBe(before[0]);
    expect(after[1]).toBe(before[1]);
    expect(after[2]).not.toBe(before[2]);
  });

  it('Edge: the example says its values belong to no game', async () => {
    await visit('/');

    expect(demo().getByText(message('landing.demoNote'))).toBeInTheDocument();
  });
});

describe('landing hero conversion', () => {
  it('Happy path: the hero copy and action render around the lazily loaded demo sheet', async () => {
    await visitDemo();

    expect(
      screen.getByRole('heading', { level: 1, name: message('landing.heroTitle') }),
    ).toBeVisible();
    expect(screen.getByText(message('landing.heroLead'))).toBeVisible();
    expect(
      within(screen.getByRole('banner')).getByRole('link', { name: message('landing.create') }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
    expect(screen.getByTestId('landing-demo-sheet')).toBeInTheDocument();
    expect(screen.queryByTestId('landing-demo-loading')).not.toBeInTheDocument();
  });

  it('Edge: the demo loads behind a reserved fallback after the static copy', () => {
    const text = readFileSync(path.resolve(here, '../landing/landing_screen.tsx'), 'utf8');

    expect(text).toContain("lazy(() => import('./demo_sheet'))");
    expect(text).toMatch(/<Suspense[\s\S]*fallback=/);
    expect(text.indexOf('landing.heroTitle')).toBeLessThan(text.indexOf('<Suspense'));
  });

  it('Edge: the demo names its character below the page heading, never as a second H1', async () => {
    await visitDemo();

    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.getByRole('heading', { level: 2, name: 'Rowan Vale' })).toBeInTheDocument();
  });

  it('Edge: the contextual CTA is absent until the visitor interacts with the demo', async () => {
    await visitDemo();

    expect(screen.queryByTestId('landing-demo-cta')).not.toBeInTheDocument();
  });

  it('Happy path: editing an ability reveals the sign-up CTA and keeps the demo state', async () => {
    const user = userEvent.setup();
    await visitDemo();
    const medallion = screen.getByTestId('sheet-ability-strength');
    const modifier = within(medallion).getByRole('button', { name: rollName('Might modifier') });

    expect(modifier).toHaveTextContent('+3');
    await user.click(within(medallion).getByRole('button', { name: 'Might' }));
    const input = within(medallion).getByRole('textbox', { name: 'Might' });
    await user.clear(input);
    await user.type(input, '18{Enter}');

    const cta = screen.getByTestId('landing-demo-cta');
    expect(
      within(cta).getByRole('link', { name: message('landing.demoCtaAction') }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
    expect(modifier).toHaveTextContent('+4');
  });

  it('Happy path: changing hit points reveals the CTA and announces the pending change', async () => {
    const user = userEvent.setup();
    await visitDemo();
    const card = screen.getByTestId('sheet-hp-card');
    const input = within(card).getByLabelText(message('sheet.play.amount'));

    await user.clear(input);
    await user.type(input, '5');
    await user.click(within(card).getByRole('button', { name: message('sheet.play.damage') }));

    expect(screen.getByTestId('landing-demo-cta')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-card')).toHaveAttribute('aria-live', 'polite');
  });

  it('Happy path: a keyboard roll opens the announced dice result and reveals the CTA', async () => {
    const user = userEvent.setup();
    await visitDemo();
    const roll = screen.getByRole('button', { name: rollName('Might modifier') });

    roll.focus();
    await user.keyboard('{Enter}');

    const panel = document.querySelector('.lg-dice-panel');
    expect(panel).not.toBeNull();
    const live = panel!.querySelector('[role="status"]');
    expect(live).toHaveAttribute('aria-live', 'polite');
    expect(live).toHaveTextContent('Might modifier');
    expect(screen.getByTestId('landing-demo-cta')).toBeInTheDocument();
  });

  it('Edge: the mobile sticky CTA is a sign-up action present for narrow viewports', async () => {
    await visit('/');

    const sticky = screen.getByTestId('landing-sticky-cta');
    expect(
      within(sticky).getByRole('link', { name: message('landing.stickyCta') }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
  });

  it('Edge: every conversion CTA on the page carries the Fantasy start intent', async () => {
    const user = userEvent.setup();
    await visitDemo();

    await user.click(screen.getByRole('button', { name: rollName('Might modifier') }));

    expect(
      within(screen.getByRole('banner')).getByRole('link', { name: message('landing.create') }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
    expect(
      within(screen.getByRole('contentinfo')).getByRole('link', {
        name: message('landing.create'),
      }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
    expect(
      within(screen.getByTestId('landing-demo-cta')).getByRole('link', {
        name: message('landing.demoCtaAction'),
      }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
    expect(
      within(screen.getByTestId('landing-sticky-cta')).getByRole('link', {
        name: message('landing.stickyCta'),
      }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
  });
});
