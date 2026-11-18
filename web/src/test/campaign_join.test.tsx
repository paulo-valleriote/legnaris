import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { DECLARED_ROUTES } from '../app/routes';
import { message, messageWith } from '../l10n/strings';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';

const CODE = 'Abcdefghijklmnopqrstuvwx';
const CAMPAIGN = { id: '01900000-0000-7000-8000-0000000000cc', name: 'Curse of Strahd' };

function renderJoin(...outcomes: Parameters<typeof createFakeTransport>) {
  const fake = createFakeTransport(...outcomes);
  renderApp(`/join/${CODE}`, {
    client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
  });
  return fake;
}

describe('join route (LGN-200)', () => {
  it('Happy path: the join route is declared as a session route with a deep link', () => {
    const route = DECLARED_ROUTES.find((declared) => declared.path === '/join/:code');

    expect(route).toMatchObject({ audience: 'session', deepLink: true, notFound: false });
  });

  it('Happy path: a campaign without a password joins with the code only and confirms', async () => {
    const fake = renderJoin(successOutcome(CAMPAIGN));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('heading', { name: message('join.joinedTitle') })).toBeVisible();
    expect(screen.getByText(messageWith('join.joined', { name: CAMPAIGN.name }))).toBeVisible();
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({ code: CODE });
    expect(screen.queryByLabelText(message('join.passwordLabel'))).not.toBeInTheDocument();
  });

  it('Happy path: a password-protected campaign reveals the password prompt and joins with it', async () => {
    const fake = renderJoin(
      errorOutcome('PASSWORD_REQUIRED', 'password', 403),
      successOutcome(CAMPAIGN),
    );
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    const password = await screen.findByLabelText(message('join.passwordLabel'));
    expect(password).toHaveAttribute('type', 'password');
    expect(password).toHaveAttribute('autocomplete', 'off');
    expect(password).toHaveFocus();
    expect(password).toHaveAccessibleDescription(message('join.passwordHint'));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();

    const blocked = screen.getByRole('button', { name: message('join.submit') });
    expect(blocked).toHaveAttribute('aria-disabled', 'true');
    expect(blocked).toHaveAccessibleDescription(message('join.submitNeedsPassword'));

    await user.type(password, 'correct horse');
    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('heading', { name: message('join.joinedTitle') })).toBeVisible();
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({
      code: CODE,
      password: 'correct horse',
    });
  });

  it('Edge: a wrong password shows the invalid-invite copy against the password field', async () => {
    renderJoin(
      errorOutcome('PASSWORD_REQUIRED', 'password', 403),
      errorOutcome('INVALID_INVITE', 'invalid', 404),
    );
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));
    const password = await screen.findByLabelText(message('join.passwordLabel'));
    await user.type(password, 'wrong horse');
    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    await waitFor(() => expect(password).toHaveAttribute('aria-invalid', 'true'));
    expect(password.getAttribute('aria-describedby')).toContain('join-password-error');
    expect(document.getElementById('join-password-error')).toHaveTextContent(
      message('api.error.invalidInvite'),
    );
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { name: message('join.joinedTitle') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: the password field stays absent until the server asks for one', async () => {
    renderJoin(errorOutcome('INVALID_INVITE', 'invalid', 404));
    const user = userEvent.setup();

    expect(screen.queryByLabelText(message('join.passwordLabel'))).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.invalidInvite'));
    expect(screen.queryByLabelText(message('join.passwordLabel'))).not.toBeInTheDocument();
  });

  it('Happy path: the confirmation carries the page title and one way back to the sheets', async () => {
    renderJoin(successOutcome(CAMPAIGN));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('heading', { name: message('join.joinedTitle') })).toBeVisible();
    const back = screen.getByRole('link', { name: message('join.backToSheets') });
    expect(back).toHaveAttribute('href', '/');
    expect(back).toHaveClass('lg-button', 'lg-button--secondary');
  });

  it('Edge: an unknown or rotated code shows the invalid-invite copy', async () => {
    renderJoin(errorOutcome('INVALID_INVITE', 'invalid', 404));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.invalidInvite'));
    expect(screen.queryByLabelText(message('join.passwordLabel'))).not.toBeInTheDocument();
  });

  it('Edge: a rate-limited attempt shows the rate limit copy', async () => {
    renderJoin(errorOutcome('RATE_LIMITED', 'retry later', 429));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.rateLimited'));
  });

  it('Edge: a full campaign shows the campaign-full copy', async () => {
    renderJoin(errorOutcome('CAMPAIGN_FULL', 'full', 409));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.campaignFull'));
  });

  it('Edge: the invite code never appears in the rendered failure copy', async () => {
    renderJoin(errorOutcome('INVALID_INVITE', CODE, 404));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    const alert = await screen.findByRole('alert');
    expect(alert.textContent).not.toContain(CODE);
  });
});
