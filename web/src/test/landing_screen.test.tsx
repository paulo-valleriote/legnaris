import { act } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message, messages } from '../l10n/strings';
import { DEMO_PATH, PRIVACY_PATH, SRD_PATH } from '../app/routes';
import { signUpCtaPath } from '../landing/start_intent';
import { renderApp } from './render';

async function visit(path: string) {
  const view = renderApp(path, { session: 'unauthenticated' });
  await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());
  return view;
}

const FIFTH_SIGN_UP_PATH = signUpCtaPath('fifth');

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
      '/sign-up?start=fifth',
    );
    expect(within(hero).getByRole('link', { name: message('auth.submitLogin') })).toHaveAttribute(
      'href',
      '/sign-in',
    );
  });

  it('Happy path: the 5E compatible badge sits above the unchanged hero title', async () => {
    await visit('/');

    const badge = screen.getByText(message('landing.badge'));
    const title = screen.getByRole('heading', { level: 1, name: message('landing.heroTitle') });

    expect(message('landing.heroTitle')).toBe(
      'The living character sheet for players who have outgrown paper.',
    );
    expect(message('landing.badge')).toBe('5E compatible');
    expect(badge).toBeVisible();
    expect(badge.compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('Happy path: the page presents the 5E positioning verbatim', async () => {
    await visit('/');

    expect(message('landing.heroLead')).toBe(
      'Replace the printed fifth-edition sheet and the PDF: enter a value once and every modifier, save, skill and spell number recalculates in place while you play.',
    );
    expect(message('landing.momentsLead')).toBe(
      'Take damage, spend a slot, take a long rest. The same sheet recalculates every number in place, mid-session.',
    );
    expect(message('landing.step1Body')).toBe(
      'Pick 5E Character for your hero, or 5E Stat Block for the creatures you run.',
    );
    expect(message('landing.faq1Answer')).toBe(
      'Fifth edition. Legnaris is 5E compatible: the 5E Character and 5E Stat Block templates follow the open SRD 5.2.1 rules and do the math for you. Other game systems join only when their rules are openly licensed.',
    );
    expect(screen.getByText(message('landing.heroLead'))).toBeVisible();
    expect(screen.getByText(message('landing.step1Body'))).toBeVisible();
  });

  it('Happy path: the systems answer links the open SRD page', async () => {
    const user = userEvent.setup();
    await visit('/');

    const question = screen.getByText(message('landing.faq1Question'));
    const item = question.closest('details');
    expect(item).not.toBeNull();

    await user.click(question);

    expect(item).toHaveTextContent(message('landing.faq1Answer'));
    expect(within(item!).getByRole('link', { name: message('landing.faq1Link') })).toHaveAttribute(
      'href',
      SRD_PATH,
    );
  });

  it('Edge: no publisher trademark appears in the landing copy', () => {
    const copy = Object.entries(messages)
      .filter(([key]) => key.startsWith('landing.'))
      .map(([, text]) => text)
      .join('\n');

    expect(copy).not.toMatch(/Dungeons? & Dragons|D&D|Wizards/i);
    expect(copy).not.toContain('no publisher content is included');
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
    ).toEqual(['/', '/sign-in', DEMO_PATH, '/sign-up?start=fifth']);
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

  it('Happy path: the footer links to the SRD attribution', async () => {
    await visit('/');

    const footer = screen.getByRole('contentinfo');
    expect(
      within(footer).getByRole('link', { name: message('landing.footer.srd') }),
    ).toHaveAttribute('href', SRD_PATH);
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
    ).toHaveAttribute('href', FIFTH_SIGN_UP_PATH);
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
    expect(within(sticky).getByRole('link', { name: message('landing.create') })).toHaveAttribute(
      'href',
      FIFTH_SIGN_UP_PATH,
    );
  });

  it('Edge: every conversion CTA on the page carries the fifth start intent', async () => {
    await visit('/');

    expect(
      within(screen.getByRole('banner')).getByRole('link', { name: message('landing.create') }),
    ).toHaveAttribute('href', FIFTH_SIGN_UP_PATH);
    expect(
      within(screen.getByRole('contentinfo')).getByRole('link', {
        name: message('landing.create'),
      }),
    ).toHaveAttribute('href', FIFTH_SIGN_UP_PATH);
    expect(
      within(screen.getByTestId('landing-sticky-cta')).getByRole('link', {
        name: message('landing.create'),
      }),
    ).toHaveAttribute('href', FIFTH_SIGN_UP_PATH);
  });
});

describe('landing mobile conversion bar', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stubObserver() {
    const callbacks: IntersectionObserverCallback[] = [];
    class StubObserver {
      constructor(callback: IntersectionObserverCallback) {
        callbacks.push(callback);
      }
      observe() {}
      unobserve() {}
      disconnect() {}
      takeRecords() {
        return [];
      }
    }
    vi.stubGlobal('IntersectionObserver', StubObserver);
    return (isIntersecting: boolean) =>
      act(() => {
        for (const callback of callbacks) {
          callback([{ isIntersecting } as IntersectionObserverEntry], {} as IntersectionObserver);
        }
      });
  }

  it('Edge: the bar stays inert while the hero actions are on screen', async () => {
    const report = stubObserver();
    await visit('/');

    report(true);

    const sticky = screen.getByTestId('landing-sticky-cta');
    expect(sticky).not.toHaveClass('lg-landing__sticky--shown');
    expect(sticky).toHaveAttribute('inert');
  });

  it('Happy path: the bar appears once the hero actions scroll away', async () => {
    const report = stubObserver();
    await visit('/');

    report(false);

    const sticky = screen.getByTestId('landing-sticky-cta');
    expect(sticky).toHaveClass('lg-landing__sticky--shown');
    expect(sticky).not.toHaveAttribute('inert');
  });

  it('Happy path: the hero leads with the live sheet, sign-up second', async () => {
    await visit('/');

    const hero = screen.getByRole('banner');
    expect(within(hero).getByRole('link', { name: message('landing.tryDemo') })).toHaveClass(
      'lg-landing__cta--primary',
    );
    expect(within(hero).getByRole('link', { name: message('landing.create') })).toHaveClass(
      'lg-landing__cta--outline',
    );
    expect(await within(hero).findByTestId('landing-hero-sheet')).toBeInTheDocument();
  });
});
