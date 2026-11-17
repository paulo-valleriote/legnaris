import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { CampaignSummary } from '../api/models/campaign';
import { updatedText, orderCampaigns } from '../campaign/campaigns_screen';
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

const OTHER_CAMPAIGN_ID = '0190c5a0-0000-7000-8000-00000000c099';
const THIRD_CAMPAIGN_ID = '0190c5a0-0000-7000-8000-00000000c09a';

const UPDATED = messageWith('campaigns.updated', { date: 'Nov 17, 2026' });

function summary(overrides: Partial<CampaignSummary> = {}): CampaignSummary {
  return {
    id: CAMPAIGN_ID,
    name: 'Ember Coast',
    templateId: 'fantasy_v1',
    version: 3,
    viewerRole: 'gm',
    viewerOwner: true,
    updatedAt: '2026-11-17T10:00:00Z',
    ...overrides,
  };
}

interface RowCase {
  readonly name: string;
  readonly summary: ReturnType<typeof campaignSummary>;
  readonly role: string;
  readonly meta: string;
}

const rowCases: readonly RowCase[] = [
  {
    name: 'Happy path: a row shows the campaign name, role pill, template name and update',
    summary: campaignSummary(),
    role: message('campaigns.owner'),
    meta: `Fantasy Template · ${UPDATED}`,
  },
  {
    name: 'Happy path: a game master who is not the owner shows the Game master pill',
    summary: campaignSummary({ viewer_owner: false, viewer_role: 'gm' }),
    role: message('campaigns.roleGm'),
    meta: `Fantasy Template · ${UPDATED}`,
  },
  {
    name: 'Happy path: a player shows the Player pill',
    summary: campaignSummary({ viewer_owner: false, viewer_role: 'player' }),
    role: message('campaigns.rolePlayer'),
    meta: `Fantasy Template · ${UPDATED}`,
  },
  {
    name: 'Edge: a campaign whose template is not in the catalog shows only the update',
    summary: campaignSummary({ template_id: 'missing_v1' }),
    role: message('campaigns.owner'),
    meta: UPDATED,
  },
  {
    name: 'Edge: an unreadable update date shows only the template name',
    summary: campaignSummary({ updated_at: 'not-a-date' }),
    role: message('campaigns.owner'),
    meta: 'Fantasy Template',
  },
];

describe('campaigns list and create (LGN-218)', () => {
  for (const testCase of rowCases) {
    it(testCase.name, async () => {
      const script = scripted([
        ...baseRoutes(),
        { method: 'GET', path: '/campaigns', respond: () => ok([testCase.summary]) },
      ]);
      renderApp('/campaigns', { client: clientFor(script.transport) });

      const row = await screen.findByRole('link', { name: /Ember Coast/ });
      expect(row).toHaveAttribute('href', `/campaigns/${CAMPAIGN_ID}`);
      expect(within(row).getByText(testCase.role)).toBeInTheDocument();
      expect(within(row).getByText(testCase.meta)).toBeInTheDocument();
      expect(
        screen.getByRole('list', { name: message('campaigns.listHeading') }),
      ).toBeInTheDocument();
    });
  }

  it('Happy path: rows keep the server order within one group', async () => {
    const script = scripted([
      ...baseRoutes(),
      {
        method: 'GET',
        path: '/campaigns',
        respond: () =>
          ok([campaignSummary(), campaignSummary({ id: OTHER_CAMPAIGN_ID, name: 'Ash Vale' })]),
      },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    const list = await screen.findByRole('list', { name: message('campaigns.listHeading') });
    const rows = within(list).getAllByRole('link');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('Ember Coast');
    expect(rows[1]).toHaveTextContent('Ash Vale');
  });

  it('Happy path: owned Campaigns first, then game masters, then players', async () => {
    const script = scripted([
      ...baseRoutes(),
      {
        method: 'GET',
        path: '/campaigns',
        respond: () =>
          ok([
            campaignSummary({
              id: OTHER_CAMPAIGN_ID,
              name: 'Ash Vale',
              viewer_owner: false,
              viewer_role: 'player',
            }),
            campaignSummary({
              id: THIRD_CAMPAIGN_ID,
              name: 'Bramble Wood',
              viewer_owner: false,
              viewer_role: 'gm',
            }),
            campaignSummary(),
          ]),
      },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    const list = await screen.findByRole('list', { name: message('campaigns.listHeading') });
    const rows = within(list).getAllByRole('link');
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent('Ember Coast');
    expect(rows[1]).toHaveTextContent('Bramble Wood');
    expect(rows[2]).toHaveTextContent('Ash Vale');
  });

  it('Edge: an empty list shows the empty copy in the designed empty block', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    const empty = await screen.findByText(message('campaigns.empty'));
    expect(empty).toHaveClass('lg-campaigns__empty');
    expect(
      screen.queryByRole('list', { name: message('campaigns.listHeading') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: a pending list shows the designed loading state', async () => {
    const script = scripted([
      ...baseRoutes(),
      {
        method: 'GET',
        path: '/campaigns',
        respond: () => new Promise<never>(() => {}),
      },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    const loading = await screen.findByText(message('campaigns.loading'));
    expect(loading).toHaveAttribute('role', 'status');
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

    expect(await screen.findByRole('link', { name: /Ember Coast/ })).toBeInTheDocument();
  });

  it('Edge: at the owned limit the notice shows, the button is disabled and described by it', async () => {
    const script = scripted([
      ...baseRoutes({ used: 1, limit: 1 }),
      { method: 'GET', path: '/campaigns', respond: () => ok([campaignSummary()]) },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    const noticeText = await screen.findByText(message('campaigns.limitReached'));
    const notice = noticeText.closest('.lg-campaigns__notice');
    expect(notice).toHaveAttribute('id', 'campaigns-limit');
    expect(notice).toHaveAttribute('role', 'status');
    expect(screen.getByRole('link', { name: message('campaigns.seePlans') })).toHaveAttribute(
      'href',
      '/plans',
    );

    const button = await screen.findByRole('button', { name: message('campaigns.new') });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-describedby', 'campaigns-limit');
    expect(screen.getByTestId('campaigns-usage')).toHaveTextContent(
      messageWith('campaigns.ownedCount', { used: 1, limit: 1 }),
    );
    expect(screen.queryByLabelText(message('campaigns.nameLabel'))).not.toBeInTheDocument();
  });

  it('Happy path: under the limit New Campaign opens the dialog with a focused name', async () => {
    const script = scripted([
      ...baseRoutes({ used: 0, limit: 2 }),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    const user = userEvent.setup();
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    const button = screen.getByRole('button', { name: message('campaigns.new') });
    expect(button).toBeEnabled();
    expect(button).not.toHaveAttribute('aria-describedby');
    expect(screen.queryByLabelText(message('campaigns.nameLabel'))).not.toBeInTheDocument();

    await user.click(button);

    const dialog = await screen.findByRole('dialog', { name: message('campaigns.create') });
    expect(within(dialog).getByLabelText(message('campaigns.nameLabel'))).toHaveFocus();
    expect(within(dialog).getByTestId('campaigns-template')).toHaveTextContent('Fantasy Template');
    expect(within(dialog).queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.getByTestId('campaigns-usage')).toHaveTextContent(
      messageWith('campaigns.ownedCount', { used: 0, limit: 2 }),
    );
  });

  it('Edge: a plan without owned_campaigns hides the count and keeps the button enabled', async () => {
    const script = scripted([
      ...baseRoutes(null),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    expect(screen.queryByTestId('campaigns-usage')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('campaigns.new') })).toBeEnabled();
  });

  it('Edge: Cancel and Escape close the dialog without a request and reopening is empty', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    const user = userEvent.setup();
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    await user.click(screen.getByRole('button', { name: message('campaigns.new') }));
    let dialog = await screen.findByRole('dialog', { name: message('campaigns.create') });
    await user.type(within(dialog).getByLabelText(message('campaigns.nameLabel')), 'Abandoned');
    await user.click(within(dialog).getByRole('button', { name: message('campaigns.cancel') }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(script.calls('POST', '/campaigns')).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: message('campaigns.new') }));
    dialog = await screen.findByRole('dialog', { name: message('campaigns.create') });
    expect(within(dialog).getByLabelText(message('campaigns.nameLabel'))).toHaveValue('');

    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(script.calls('POST', '/campaigns')).toHaveLength(0);
  });

  it('Edge: a blank name is refused on the client and sends no request', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    const user = userEvent.setup();
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    await user.click(screen.getByRole('button', { name: message('campaigns.new') }));
    const dialog = await screen.findByRole('dialog', { name: message('campaigns.create') });
    await user.type(within(dialog).getByLabelText(message('campaigns.nameLabel')), '   ');
    await user.click(within(dialog).getByRole('button', { name: message('campaigns.create') }));

    expect(await within(dialog).findByText(message('campaigns.nameRequired'))).toBeInTheDocument();
    expect(script.calls('POST', '/campaigns')).toHaveLength(0);
  });

  it('Error: PLAN_LIMIT_REACHED on create shows the plan copy in the dialog and keeps the name', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
      { method: 'POST', path: '/campaigns', respond: () => fail('PLAN_LIMIT_REACHED', 403) },
    ]);
    const user = userEvent.setup();
    renderApp('/campaigns', { client: clientFor(script.transport) });

    await screen.findByText(message('campaigns.empty'));
    await user.click(screen.getByRole('button', { name: message('campaigns.new') }));
    const dialog = await screen.findByRole('dialog', { name: message('campaigns.create') });
    const name = within(dialog).getByLabelText(message('campaigns.nameLabel'));
    await user.type(name, 'Ember Coast');
    await user.click(within(dialog).getByRole('button', { name: message('campaigns.create') }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      message('api.error.planLimitReached'),
    );
    expect(within(dialog).getByLabelText(message('campaigns.nameLabel'))).toHaveValue(
      'Ember Coast',
    );
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
    await user.click(screen.getByRole('button', { name: message('campaigns.new') }));
    const dialog = await screen.findByRole('dialog', { name: message('campaigns.create') });
    await user.type(within(dialog).getByLabelText(message('campaigns.nameLabel')), 'Ember Coast');
    await user.click(within(dialog).getByRole('button', { name: message('campaigns.create') }));

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

describe('campaign list helpers', () => {
  it('Edge: ordering an empty list returns an empty list', () => {
    expect(orderCampaigns([])).toEqual([]);
  });

  it('Happy path: ownership and role order the list, server order kept inside a group', () => {
    const player = summary({ id: 'p', name: 'Player', viewerOwner: false, viewerRole: 'player' });
    const gm = summary({ id: 'g', name: 'Game master', viewerOwner: false, viewerRole: 'gm' });
    const owner = summary({ id: 'o', name: 'Owner' });
    const secondGm = summary({
      id: 'g2',
      name: 'Second game master',
      viewerOwner: false,
      viewerRole: 'gm',
    });

    expect(orderCampaigns([player, gm, owner, secondGm]).map((entry) => entry.id)).toEqual([
      'o',
      'g',
      'g2',
      'p',
    ]);
  });

  it('Edge: ordering returns a new array and leaves the input untouched', () => {
    const player = summary({ id: 'p', viewerOwner: false, viewerRole: 'player' });
    const owner = summary({ id: 'o' });
    const input = [player, owner];

    const ordered = orderCampaigns(input);

    expect(ordered).not.toBe(input);
    expect(input.map((entry) => entry.id)).toEqual(['p', 'o']);
    expect(ordered.map((entry) => entry.id)).toEqual(['o', 'p']);
  });

  it('Happy path: a readable update date resolves to the medium date copy', () => {
    expect(updatedText('2026-11-17T10:00:00Z')).toBe(UPDATED);
  });

  it('Edge: an unreadable or empty update date resolves to nothing', () => {
    expect(updatedText('not-a-date')).toBeUndefined();
    expect(updatedText('')).toBeUndefined();
  });
});
