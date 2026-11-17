import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
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

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.passwordRequired'),
    );
    const password = screen.getByLabelText(message('join.passwordLabel'));
    expect(password).toHaveAttribute('type', 'password');
    expect(password).toHaveAttribute('autocomplete', 'off');

    const submit = screen.getByRole('button', { name: message('join.submit') });
    expect(submit).toBeDisabled();

    await user.type(password, 'correct horse');
    await user.click(submit);

    expect(await screen.findByRole('heading', { name: message('join.joinedTitle') })).toBeVisible();
    expect(JSON.parse(fake.lastRequest?.body ?? '')).toEqual({
      code: CODE,
      password: 'correct horse',
    });
  });

  it('Edge: a wrong password shows the same invalid-invite copy as an unknown code', async () => {
    renderJoin(
      errorOutcome('PASSWORD_REQUIRED', 'password', 403),
      errorOutcome('INVALID_INVITE', 'invalid', 404),
    );
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));
    await user.type(await screen.findByLabelText(message('join.passwordLabel')), 'wrong horse');
    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.invalidInvite'));
    expect(
      screen.queryByRole('heading', { name: message('join.joinedTitle') }),
    ).not.toBeInTheDocument();
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
