import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { MIGHT_MAX, MIGHT_MIN } from '../landing/cascade';
import { renderApp } from './render';

/** Mounts the app as a visitor and waits for the cold-start restore to settle signed out. */
async function visit(path: string) {
  const view = renderApp(path, { session: 'unauthenticated' });
  await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());
  return view;
}

describe('landing screen', () => {
  it('Happy path: a visitor on the root sees the landing, not the sign-in form or the catalog', async () => {
    await visit('/');

    expect(screen.getByTestId('landing')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      message('landing.heroTitle'),
    );
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
    expect(screen.queryByLabelText(message('auth.passwordLabel'))).not.toBeInTheDocument();
  });

  it('Happy path: the hero actions lead to the registration and sign-in paths', async () => {
    await visit('/');

    const hero = screen.getByRole('banner');
    expect(within(hero).getByRole('link', { name: message('landing.create') })).toHaveAttribute(
      'href',
      '/sign-up',
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
    ).toEqual(['/', '/sign-in', '/sign-up']);
  });

  it('Edge: the footer carries only the closing call to action and the legal line, no second photograph', async () => {
    await visit('/');

    const footer = screen.getByRole('contentinfo');
    expect(footer.querySelector('img')).toBeNull();
    expect(footer.lastElementChild).toHaveClass('lg-landing__legal');
    expect(screen.getByTestId('landing').querySelectorAll('img')).toHaveLength(1);
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

    await user.click(question);

    expect(item).toHaveAttribute('open');
    expect(screen.getByText(message('landing.faq1Answer'))).toBeVisible();
    expect(screen.getAllByRole('group').length).toBeGreaterThanOrEqual(1);
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

  it('Happy path: raising the base score recalculates every derived value', async () => {
    const user = userEvent.setup();
    await visit('/');

    await user.click(demo().getByRole('button', { name: message('landing.demoBaseUp') }));
    await user.click(demo().getByRole('button', { name: message('landing.demoBaseUp') }));

    const values = demo()
      .getAllByRole('status')
      .map((node) => node.textContent);
    expect(values).toEqual(['16', '+3', '+6', '112']);
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
    expect(demo().getAllByRole('status')[0]).toHaveTextContent(String(MIGHT_MIN));
    expect(demo().getAllByRole('status')[1]).toHaveTextContent('−5');
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
    expect(demo().getAllByRole('status')[0]).toHaveTextContent(String(MIGHT_MAX));
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
