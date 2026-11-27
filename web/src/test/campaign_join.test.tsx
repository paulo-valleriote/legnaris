import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { parseCampaign } from '../api/models/campaign';
import { DECLARED_ROUTES } from '../app/routes';
import { message, messageWith } from '../l10n/strings';
import { campaignView } from './campaign_helpers';
import { errorOutcome, FREE_PLAN_USAGE, successOutcome } from './fake_transport';
import { renderApp } from './render';

const CODE = 'Abcdefghijklmnopqrstuvwx';
const CAMPAIGN_WIRE = campaignView({ viewer_role: 'player', viewer_owner: false });
const CAMPAIGN = parseCampaign(CAMPAIGN_WIRE);
const CAMPAIGN_PATH = `/campaigns/${CAMPAIGN.id}`;

function joinClient(...joinOutcomes: HttpResponse[]) {
  let index = 0;
  const requests: HttpRequest[] = [];
  const transport: Transport = (request) => {
    requests.push(request);
    const path = new URL(request.url).pathname;
    if (request.method === 'POST' && path === '/campaigns/join') {
      const response = joinOutcomes[Math.min(index, joinOutcomes.length - 1)];
      index += 1;
      return Promise.resolve(response as HttpResponse);
    }
    if (path === CAMPAIGN_PATH) {
      return Promise.resolve(successOutcome(CAMPAIGN_WIRE));
    }
    if (path === '/me/plan') {
      return Promise.resolve(successOutcome(FREE_PLAN_USAGE));
    }
    return Promise.resolve(successOutcome([]));
  };
  renderApp(`/join/${CODE}`, {
    client: createApiClient({ baseUrl: 'http://api.test', transport }),
  });
  return requests;
}

describe('join route (LGN-200)', () => {
  it('Happy path: the join route is declared as a session route with a deep link', () => {
    const route = DECLARED_ROUTES.find((declared) => declared.path === '/join/:code');

    expect(route).toMatchObject({ audience: 'session', deepLink: true, notFound: false });
  });

  it('Happy path: a join opens the campaign directly and announces the arrival', async () => {
    const requests = joinClient(successOutcome(CAMPAIGN_WIRE));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    const notice = await screen.findByTestId('campaign-joined-notice');
    expect(notice).toHaveTextContent(messageWith('campaign.joinedNotice', { name: CAMPAIGN.name }));
    expect(screen.getByTestId('location-pathname')).toHaveTextContent(CAMPAIGN_PATH);
    expect(notice).toHaveFocus();
    const join = requests.find(
      (request) => request.method === 'POST' && request.url.endsWith('/campaigns/join'),
    );
    expect(JSON.parse(join?.body ?? '')).toEqual({ code: CODE });
  });

  it('Happy path: an existing member also lands on the campaign', async () => {
    joinClient(successOutcome(CAMPAIGN_WIRE));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByTestId('campaign-joined-notice')).toHaveTextContent(
      messageWith('campaign.joinedNotice', { name: CAMPAIGN.name }),
    );
    expect(screen.getByTestId('location-pathname')).toHaveTextContent(CAMPAIGN_PATH);
  });

  it('Edge: a password-protected campaign asks for the password and then opens the campaign', async () => {
    joinClient(errorOutcome('PASSWORD_REQUIRED', 'password', 403), successOutcome(CAMPAIGN_WIRE));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));
    const password = await screen.findByLabelText(message('join.passwordLabel'));
    expect(password).toHaveFocus();

    await user.type(password, 'correct horse');
    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByTestId('campaign-joined-notice')).toBeInTheDocument();
    expect(screen.getByTestId('location-pathname')).toHaveTextContent(CAMPAIGN_PATH);
  });

  it('Error: an invalid invite stays on the join screen with the copy', async () => {
    joinClient(errorOutcome('INVALID_INVITE', 'invalid', 404));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.invalidInvite'));
    await waitFor(() =>
      expect(screen.getByTestId('location-pathname')).toHaveTextContent(`/join/${CODE}`),
    );
    expect(screen.queryByTestId('campaign-joined-notice')).not.toBeInTheDocument();
  });

  it('Edge: an unknown code shows the invalid-invite copy and never navigates', async () => {
    joinClient(errorOutcome('INVALID_INVITE', 'invalid', 404));
    const user = userEvent.setup();

    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.invalidInvite'));
    expect(screen.getByTestId('location-pathname')).toHaveTextContent(`/join/${CODE}`);
  });
});
