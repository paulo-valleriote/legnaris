import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message, messageWith } from '../l10n/strings';
import {
  fail,
  baseRoutes,
  campaignSummary,
  campaignView,
  CAMPAIGN_ID,
  ok,
  scripted,
  clientFor,
} from './campaign_helpers';
import { renderApp } from './render';

describe('campaigns list and create (LGN-218)', () => {
  it('Happy path: lists the caller campaigns and shows owned Campaigns against the plan limit', async () => {
    const script = scripted([
      ...baseRoutes({ used: 1, limit: 5 }),
      { method: 'GET', path: '/campaigns', respond: () => ok([campaignSummary()]) },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    expect(await screen.findByRole('link', { name: 'Ember Coast' })).toHaveAttribute(
      'href',
      `/campaigns/${CAMPAIGN_ID}`,
    );
    expect(
      screen.getByText(messageWith('campaigns.ownedCount', { used: 1, limit: 5 })),
    ).toBeInTheDocument();
  });

  it('Edge: an empty list shows the empty copy, not a blank screen', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    expect(await screen.findByText(message('campaigns.empty'))).toBeInTheDocument();
  });

  it('Error: a failed list shows the failure copy with a retry that refetches', async () => {
    let calls = 0;
    const script = scripted([
      ...baseRoutes(),
      {
        method: 'GET',
        path: '/campaigns',
        respond: () => {
          calls += 1;
          return calls === 1 ? fail('INTERNAL_ERROR', 500) : ok([campaignSummary()]);
        },
      },
    ]);
    const user = userEvent.setup();
    renderApp('/campaigns', { client: clientFor(script.transport) });

    const alert = await screen.findByRole('alert');
    expect(
      within(alert).getByText(message('campaigns.loadFailed'), { exact: false }),
    ).toBeInTheDocument();
    await user.click(within(alert).getByRole('button', { name: message('campaigns.retry') }));

    expect(await screen.findByRole('link', { name: 'Ember Coast' })).toBeInTheDocument();
  });

  it('Edge: at the owned Campaign limit the create form is disabled and says why', async () => {
    const script = scripted([
      ...baseRoutes({ used: 1, limit: 1 }),
      { method: 'GET', path: '/campaigns', respond: () => ok([campaignSummary()]) },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    expect(await screen.findByText(message('campaigns.limitReached'))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('campaigns.create') })).toBeDisabled();
  });

  it('Edge: a plan without owned_campaigns hides the count rather than inventing one', async () => {
    const script = scripted([
      ...baseRoutes(null),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    expect(screen.queryByText(/Campaigns you own/)).not.toBeInTheDocument();
  });

  it('Edge: a blank name is refused on the client and sends no request', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    const user = userEvent.setup();
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    await user.type(screen.getByLabelText(message('campaigns.nameLabel')), '   ');
    await user.click(screen.getByRole('button', { name: message('campaigns.create') }));

    expect(await screen.findByText(message('campaigns.nameRequired'))).toBeInTheDocument();
    expect(script.calls('POST', '/campaigns')).toHaveLength(0);
  });

  it('Error: PLAN_LIMIT_REACHED on create shows the plan copy and keeps the name', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
      { method: 'POST', path: '/campaigns', respond: () => fail('PLAN_LIMIT_REACHED', 403) },
    ]);
    const user = userEvent.setup();
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    await user.type(screen.getByLabelText(message('campaigns.nameLabel')), 'Ember Coast');
    await user.click(screen.getByRole('button', { name: message('campaigns.create') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.planLimitReached'),
    );
    expect(screen.getByLabelText(message('campaigns.nameLabel'))).toHaveValue('Ember Coast');
  });

  it('Happy path: creating a campaign posts name and template and opens its page', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
      { method: 'POST', path: '/campaigns', respond: () => ok(campaignView(), 201) },
      { method: 'GET', path: `/campaigns/${CAMPAIGN_ID}`, respond: () => ok(campaignView()) },
    ]);
    const user = userEvent.setup();
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    await user.type(screen.getByLabelText(message('campaigns.nameLabel')), 'Ember Coast');
    await user.click(screen.getByRole('button', { name: message('campaigns.create') }));

    await waitFor(() => expect(script.calls('POST', '/campaigns')).toHaveLength(1));
    expect(JSON.parse(script.calls('POST', '/campaigns')[0]?.request.body ?? '')).toEqual({
      name: 'Ember Coast',
      template_id: 'fantasy_v1',
    });
    expect(
      await screen.findByRole('heading', { name: 'Ember Coast', level: 1 }),
    ).toBeInTheDocument();
  });
});
