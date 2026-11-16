import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { parseMarkdown } from '../legal/privacy_markdown';
import { policyMarkdown } from '../legal/privacy_policy';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';

const PLAN = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
  llm_consent_at: null,
  llm_consent_version: null,
};

function accountClient(plan: ReturnType<typeof successOutcome> | ReturnType<typeof errorOutcome>) {
  const fake = createFakeTransport(successOutcome([]), plan);
  return createApiClient({ baseUrl: 'http://api.test', transport: fake.transport });
}

function policyTitle(): string {
  const [first] = parseMarkdown(policyMarkdown('en'));
  if (first?.kind !== 'heading') {
    throw new Error('the English policy must open with a heading');
  }
  return first.text;
}

function openButton() {
  return screen.getByRole('button', { name: message('account.privacyOpen') });
}

describe('account privacy dialog (LGN-234)', () => {
  it('Happy path: the account screen opens the policy in a modal without leaving the route', async () => {
    const user = userEvent.setup();
    renderApp('/account', { client: accountClient(successOutcome(PLAN)) });

    expect(screen.queryByRole('link', { name: /privacy/i })).not.toBeInTheDocument();
    await user.click(openButton());

    const dialog = screen.getByRole('dialog', { name: policyTitle() });
    expect(dialog).toHaveAttribute('aria-modal', 'true');
    expect(within(dialog).getAllByRole('heading', { level: 3 }).length).toBeGreaterThan(0);
    expect(within(dialog).queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    expect(screen.getByTestId('privacy-dialog-body')).toHaveFocus();
    expect(screen.getByRole('heading', { level: 1, name: 'Your account' })).toBeInTheDocument();
  });

  it('Edge: Escape closes the modal and returns focus to the opener', async () => {
    const user = userEvent.setup();
    renderApp('/account', { client: accountClient(successOutcome(PLAN)) });

    await user.click(openButton());
    await user.keyboard('{Escape}');

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(openButton()).toHaveFocus();
  });

  it('Edge: the close button and a scrim click both dismiss the modal', async () => {
    const user = userEvent.setup();
    renderApp('/account', { client: accountClient(successOutcome(PLAN)) });

    await user.click(openButton());
    await user.click(screen.getByRole('button', { name: message('privacy.close') }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    await user.click(openButton());
    await user.click(screen.getByTestId('privacy-dialog-overlay'));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('Edge: a click inside the policy keeps the modal open', async () => {
    const user = userEvent.setup();
    renderApp('/account', { client: accountClient(successOutcome(PLAN)) });

    await user.click(openButton());
    await user.click(screen.getByTestId('privacy-dialog-body'));

    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('Edge: Tab stays trapped inside the modal', async () => {
    const user = userEvent.setup();
    renderApp('/account', { client: accountClient(successOutcome(PLAN)) });

    await user.click(openButton());
    const dialog = screen.getByRole('dialog');
    for (let step = 0; step < 6; step += 1) {
      await user.tab();
      expect(dialog).toContainElement(document.activeElement as HTMLElement);
    }
  });

  it('Edge: the policy stays reachable when the plan fails to load', async () => {
    const user = userEvent.setup();
    renderApp('/account', {
      client: accountClient(errorOutcome('INTERNAL', 'The server had a problem.', 500)),
    });

    expect(await screen.findByTestId('account-failure')).toBeInTheDocument();
    await user.click(openButton());

    expect(screen.getByRole('dialog', { name: policyTitle() })).toBeInTheDocument();
  });
});
