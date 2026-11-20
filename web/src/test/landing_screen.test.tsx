import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { DEMO_PATH, PRIVACY_PATH } from '../app/routes';
import { signUpCtaPath } from '../landing/start_intent';
import { renderApp } from './render';

async function visit(path: string) {
  const view = renderApp(path, { session: 'unauthenticated' });
  await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());
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

  it('Edge: the hero carries no in-page navigation, only the brand and its actions', async () => {
    await visit('/');

    const hero = screen.getByRole('banner');
    expect(within(hero).queryByRole('navigation')).not.toBeInTheDocument();
    expect(
      within(hero)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href')),
    ).toEqual(['/', '/sign-in', '/sign-up?start=fantasy', DEMO_PATH]);
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

  it('Happy path: the game moments replace the principles and keep the questions and the legal line', async () => {
    await visit('/');

    expect(
      screen.getByRole('heading', { level: 2, name: message('landing.momentsTitle') }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { level: 2, name: message('landing.faqTitle') }),
    ).toBeInTheDocument();
    expect(screen.getByText(message('landing.faq1Question'))).toBeInTheDocument();
    expect(
      within(screen.getByRole('contentinfo')).getByRole('link', {
        name: message('landing.footer.privacy'),
      }),
    ).toHaveAttribute('href', PRIVACY_PATH);
  });

  it('Edge: the home mounts no ad slot even when a slot is configured', async () => {
    renderApp('/', {
      session: 'unauthenticated',
      adsConfig: { client: 'ca-pub-1234567890', slots: { catalog: '1111111111' } },
    });
    await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());

    expect(document.querySelector('.lg-ad-slot')).toBeNull();
    expect(screen.queryByText(message('ads.label'))).not.toBeInTheDocument();
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

describe('landing conversion', () => {
  it('Happy path: the hero keeps the static copy and its two actions', async () => {
    await visit('/');

    expect(
      screen.getByRole('heading', { level: 1, name: message('landing.heroTitle') }),
    ).toBeVisible();
    expect(screen.getByText(message('landing.heroLead'))).toBeVisible();
    expect(
      within(screen.getByRole('banner')).getByRole('link', { name: message('landing.create') }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
    expect(
      within(screen.getByRole('banner')).getByRole('link', { name: message('landing.tryDemo') }),
    ).toHaveAttribute('href', DEMO_PATH);
  });

  it('Edge: the home never mounts the demo sheet or its contextual CTA', async () => {
    await visit('/');

    expect(screen.queryByTestId('landing-demo-sheet')).not.toBeInTheDocument();
    expect(screen.queryByTestId('landing-demo-cta')).not.toBeInTheDocument();
    expect(screen.queryByTestId('landing-demo-loading')).not.toBeInTheDocument();
  });

  it('Edge: the mobile sticky CTA is a sign-up action present for narrow viewports', async () => {
    await visit('/');

    const sticky = screen.getByTestId('landing-sticky-cta');
    expect(
      within(sticky).getByRole('link', { name: message('landing.stickyCta') }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
  });

  it('Edge: every conversion CTA on the page carries the Fantasy start intent', async () => {
    await visit('/');

    expect(
      within(screen.getByRole('banner')).getByRole('link', { name: message('landing.create') }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
    expect(
      within(screen.getByRole('contentinfo')).getByRole('link', {
        name: message('landing.create'),
      }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
    expect(
      within(screen.getByTestId('landing-sticky-cta')).getByRole('link', {
        name: message('landing.stickyCta'),
      }),
    ).toHaveAttribute('href', FANTASY_SIGN_UP_PATH);
  });
});
