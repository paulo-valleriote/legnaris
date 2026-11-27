import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message, messageWith } from '../l10n/strings';
import {
  CAMPAIGN_ID,
  OTHER_SHEET_ID,
  PLAYER_ID,
  SHEET_ID,
  baseRoutes,
  campaignView,
  clientFor,
  fail,
  ok,
  scripted,
} from './campaign_helpers';
import type { Route } from './campaign_helpers';
import { renderApp } from './render';

const DETAIL = `/campaigns/${CAMPAIGN_ID}`;

function detailRoutes(view: Record<string, unknown>, extra: Route[] = []): Route[] {
  return [
    ...baseRoutes(),
    { method: 'GET', path: DETAIL, respond: () => ok(view) },
    { method: 'GET', path: `${DETAIL}/sheets`, respond: () => ok([]) },
    { method: 'GET', path: `${DETAIL}/npcs`, respond: () => ok([]) },
    { method: 'GET', path: '/sheets/owned', respond: () => ok([ownedSheet()]) },
    ...extra,
  ];
}

function ownedSheet(overrides: Record<string, unknown> = {}) {
  return {
    id: SHEET_ID,
    name: 'Brenna',
    template_id: 'fantasy_v1',
    kind: 'character',
    data: {},
    created_at: '2026-11-17T10:00:00Z',
    updated_at: '2026-11-17T10:00:00Z',
    ...overrides,
  };
}

function playerView() {
  return campaignView({ viewer_role: 'player', viewer_owner: false });
}

describe('campaign screen: members and roles (LGN-218)', () => {
  it('Happy path: the header pairs the back link and title with the role pill', async () => {
    const script = scripted(detailRoutes(campaignView()));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(
      await screen.findByRole('heading', { name: 'Ember Coast', level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: message('campaign.back') })).toHaveAttribute(
      'href',
      '/campaigns',
    );
    expect(screen.getByText(message('campaigns.owner'))).toHaveClass('lg-campaigns__role');
  });

  it('Happy path: the header shows the scope name from the payload, even for a Retired Template', async () => {
    const script = scripted(
      detailRoutes(
        campaignView({
          template_id: 'retired_v1',
          scope_name: 'Retired Archive',
          template_ids: ['retired_v1'],
        }),
      ),
    );
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(
      await screen.findByRole('heading', { name: 'Ember Coast', level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByTestId('campaign-scope')).toHaveTextContent('Retired Archive');
  });

  it('Edge: a section heading carries its count in the accessible name', async () => {
    const script = scripted(detailRoutes(campaignView()));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    expect(
      screen.getByRole('heading', {
        name: `${message('campaign.membersHeading')} (2)`,
        level: 2,
      }),
    ).toBeInTheDocument();
  });

  it('Happy path: the owner sees every member with role, character and the actions the owner may take', async () => {
    const script = scripted(detailRoutes(campaignView()));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(
      await screen.findByRole('heading', { name: 'Ember Coast', level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByText('tomas')).toBeInTheDocument();
    expect(
      screen.getByText(messageWith('campaign.characterName', { name: 'Brenna' }), { exact: false }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: `${message('campaign.promote')} tomas` }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('campaign.delete') }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: message('campaign.settingsHeading') }));
    expect(screen.getByRole('button', { name: message('campaign.delete') })).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('campaign.leave') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: the owner row carries no member actions', async () => {
    const script = scripted(detailRoutes(campaignView()));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('mara');
    expect(
      screen.queryByRole('button', { name: `${message('campaign.remove')} mara` }),
    ).not.toBeInTheDocument();
  });

  it('Edge: a player sees no member actions and no delete, only leave and the character controls', async () => {
    const script = scripted(detailRoutes(playerView()));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    expect(
      screen.queryByRole('button', { name: message('campaign.remove') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('campaign.delete') }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('campaign.leave') })).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('invite.heading') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('campaign.settingsHeading') }),
    ).not.toBeInTheDocument();
    expect(await screen.findByLabelText(message('campaign.linkLabel'))).toBeInTheDocument();
  });

  it('Edge: a GM removes a player only after confirming, and a dismissed dialog sends nothing', async () => {
    const gmView = campaignView({ viewer_role: 'gm', viewer_owner: false });
    const script = scripted(detailRoutes(gmView));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.click(screen.getByRole('button', { name: `${message('campaign.remove')} tomas` }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: message('campaign.keepMember') }));

    expect(script.calls('POST', `${DETAIL}/members/${PLAYER_ID}/remove`)).toHaveLength(0);
  });

  it('Happy path: confirming a removal posts the remove path and shows the refreshed members', async () => {
    const gmView = campaignView({ viewer_role: 'gm', viewer_owner: false });
    const after = campaignView({
      viewer_role: 'gm',
      viewer_owner: false,
      members: [campaignView().members[0]],
      player_count: 0,
    });
    const script = scripted(
      detailRoutes(gmView, [
        { method: 'POST', path: `${DETAIL}/members/${PLAYER_ID}/remove`, respond: () => ok(after) },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.click(screen.getByRole('button', { name: `${message('campaign.remove')} tomas` }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: message('campaign.removeConfirm'),
      }),
    );

    await waitFor(() => expect(screen.queryByText('tomas')).not.toBeInTheDocument());
    expect(script.calls('POST', `${DETAIL}/members/${PLAYER_ID}/remove`)).toHaveLength(1);
  });

  it('Error: a refused member action shows the mapped copy in an alert', async () => {
    const script = scripted(
      detailRoutes(campaignView(), [
        {
          method: 'POST',
          path: `${DETAIL}/members/${PLAYER_ID}/promote`,
          respond: () => fail('CAMPAIGN_FULL', 409),
        },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.click(screen.getByRole('button', { name: `${message('campaign.promote')} tomas` }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('api.error.campaignFull'));
  });
});

describe('campaign screen: character link (LGN-218)', () => {
  it('Happy path: a player links a sheet by id and sees the confirmation', async () => {
    const script = scripted(
      detailRoutes(playerView(), [
        { method: 'PUT', path: `${DETAIL}/character`, respond: () => ok(playerView()) },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.selectOptions(await screen.findByLabelText(message('campaign.linkLabel')), SHEET_ID);
    await user.click(screen.getByRole('button', { name: message('campaign.link') }));

    expect(await screen.findByText(message('campaign.linked'))).toBeInTheDocument();
    expect(JSON.parse(script.calls('PUT', `${DETAIL}/character`)[0]?.request.body ?? '')).toEqual({
      sheet_id: SHEET_ID,
    });
  });

  it('Happy path: a player unlinks the character and the owned list refreshes', async () => {
    let unlinked = false;
    const script = scripted(
      detailRoutes(playerView(), [
        {
          method: 'DELETE',
          path: `${DETAIL}/character`,
          respond: () => {
            unlinked = true;
            return ok(playerView());
          },
        },
      ]).map((route) =>
        route.path === '/sheets/owned'
          ? {
              ...route,
              respond: () => ok([ownedSheet(unlinked ? {} : { campaign_id: CAMPAIGN_ID })]),
            }
          : route,
      ),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.click(await screen.findByRole('button', { name: message('campaign.unlink') }));

    expect(await screen.findByText(message('campaign.unlinked'))).toBeInTheDocument();
    expect(script.calls('DELETE', `${DETAIL}/character`)).toHaveLength(1);
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: message('campaign.unlink') })).toBeNull(),
    );
    expect(
      screen.queryByRole('link', {
        name: messageWith('campaign.openCharacter', { name: 'Brenna' }),
      }),
    ).toBeNull();
  });

  it.each([
    ['TEMPLATE_MISMATCH', 'api.error.templateMismatch'],
    ['SHEET_ALREADY_LINKED', 'api.error.sheetAlreadyLinked'],
    ['NOT_FOUND', 'api.error.notFound'],
    ['INVALID_REQUEST', 'api.error.invalidEdit'],
  ] as const)(
    'Error: linking refused with %s shows its copy and keeps the chosen sheet',
    async (code, key) => {
      const script = scripted(
        detailRoutes(playerView(), [
          { method: 'PUT', path: `${DETAIL}/character`, respond: () => fail(code, 422) },
        ]),
      );
      const user = userEvent.setup();
      renderApp(DETAIL, { client: clientFor(script.transport) });

      await screen.findByText('tomas');
      await user.selectOptions(
        await screen.findByLabelText(message('campaign.linkLabel')),
        SHEET_ID,
      );
      await user.click(screen.getByRole('button', { name: message('campaign.link') }));

      expect(await screen.findByRole('alert')).toHaveTextContent(message(key));
      expect(screen.getByLabelText(message('campaign.linkLabel'))).toHaveValue(SHEET_ID);
    },
  );

  it('Edge: with no free character of the Template the link is disabled and says why', async () => {
    const script = scripted(
      detailRoutes(playerView()).map((route) =>
        route.path === '/sheets/owned'
          ? { ...route, respond: () => ok([ownedSheet({ template_id: 'other_v1' })]) }
          : route,
      ),
    );
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(message('campaign.noEligibleSheets'))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('campaign.link') })).toBeDisabled();
    expect(script.calls('PUT', `${DETAIL}/character`)).toHaveLength(0);
  });

  it('Edge: a sheet linked to another Campaign is not offered for linking', async () => {
    const script = scripted(
      detailRoutes(playerView()).map((route) =>
        route.path === '/sheets/owned'
          ? { ...route, respond: () => ok([ownedSheet({ campaign_id: OTHER_SHEET_ID })]) }
          : route,
      ),
    );
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(message('campaign.noEligibleSheets'))).toBeInTheDocument();
    expect(
      screen.queryByRole('combobox', { name: message('campaign.linkLabel') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: the player own row is labelled and carries no actions', async () => {
    const script = scripted(
      detailRoutes(
        campaignView({ viewer_role: 'gm', viewer_owner: false, viewer_user_id: PLAYER_ID }),
      ),
    );
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(`tomas (${message('campaign.you')})`)).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: `${message('campaign.remove')} tomas` }),
    ).not.toBeInTheDocument();
  });
});

describe("campaign screen: the player's linked character", () => {
  function linkedRoutes(owned: unknown[], extra: Route[] = []): Route[] {
    return [
      ...detailRoutes(playerView(), extra).filter((route) => route.path !== '/sheets/owned'),
      { method: 'GET', path: '/sheets/owned', respond: () => ok(owned) },
    ];
  }

  it('Happy path: the linked character shows as a card that opens its sheet', async () => {
    const script = scripted(linkedRoutes([ownedSheet({ campaign_id: CAMPAIGN_ID })]));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const link = await screen.findByRole('link', {
      name: messageWith('campaign.openCharacter', { name: 'Brenna' }),
    });
    expect(link).toHaveAttribute('href', `/sheets/${SHEET_ID}`);
    const card = link.closest('li') as HTMLElement;
    expect(within(card).getByText(/Updated Nov 17, 2026/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('campaign.unlink') })).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: message('campaign.linkLabel') })).toBeNull();
    expect(screen.queryByRole('button', { name: message('campaign.replace') })).toBeNull();
  });

  it('Edge: with nothing linked the section offers the free sheet and no unlink action', async () => {
    const script = scripted(linkedRoutes([ownedSheet()]));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const select = await screen.findByRole('combobox', { name: message('campaign.linkLabel') });
    expect(select).toHaveValue('');
    expect(within(select).getByRole('option', { name: 'Brenna' })).toBeInTheDocument();
    expect(
      within(select).getByRole('option', { name: message('campaign.chooseSheet') }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('campaign.link') })).toBeDisabled();
    expect(screen.queryByRole('button', { name: message('campaign.unlink') })).toBeNull();
    expect(
      screen.queryByRole('link', {
        name: messageWith('campaign.openCharacter', { name: 'Brenna' }),
      }),
    ).toBeNull();
  });

  it('Edge: the linked character is not offered again and another one replaces it', async () => {
    const script = scripted(
      linkedRoutes([
        ownedSheet({ campaign_id: CAMPAIGN_ID }),
        ownedSheet({ id: OTHER_SHEET_ID, name: 'Rowan' }),
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const select = await screen.findByRole('combobox', { name: message('campaign.linkLabel') });
    expect(select).toHaveValue('');
    expect(within(select).queryByRole('option', { name: 'Brenna' })).toBeNull();
    expect(within(select).getByRole('option', { name: 'Rowan' })).toBeInTheDocument();
    expect(screen.getByText(message('campaign.linkHint'))).toBeInTheDocument();
    const replace = screen.getByRole('button', { name: message('campaign.replace') });
    expect(replace).toBeDisabled();
    await user.selectOptions(select, OTHER_SHEET_ID);
    expect(replace).toBeEnabled();
  });

  it('Happy path: replacing the character links the chosen sheet and shows it', async () => {
    let replaced = false;
    const script = scripted([
      ...detailRoutes(playerView(), [
        {
          method: 'PUT',
          path: `${DETAIL}/character`,
          respond: () => {
            replaced = true;
            return ok(playerView());
          },
        },
      ]).filter((route) => route.path !== '/sheets/owned'),
      {
        method: 'GET',
        path: '/sheets/owned',
        respond: () =>
          ok([
            ownedSheet(replaced ? {} : { campaign_id: CAMPAIGN_ID }),
            ownedSheet({
              id: OTHER_SHEET_ID,
              name: 'Rowan',
              campaign_id: replaced ? CAMPAIGN_ID : undefined,
            }),
          ]),
      },
    ]);
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const select = await screen.findByRole('combobox', { name: message('campaign.linkLabel') });
    await user.selectOptions(select, OTHER_SHEET_ID);
    await user.click(screen.getByRole('button', { name: message('campaign.replace') }));

    expect(
      await screen.findByRole('link', {
        name: messageWith('campaign.openCharacter', { name: 'Rowan' }),
      }),
    ).toBeInTheDocument();
    expect(JSON.parse(script.calls('PUT', `${DETAIL}/character`)[0]?.request.body ?? '')).toEqual({
      sheet_id: OTHER_SHEET_ID,
    });
  });

  it('Happy path: a link refreshes the owned list and the card appears without a reload', async () => {
    let linked = false;
    const script = scripted([
      ...detailRoutes(playerView(), [
        {
          method: 'PUT',
          path: `${DETAIL}/character`,
          respond: () => {
            linked = true;
            return ok(playerView());
          },
        },
      ]).filter((route) => route.path !== '/sheets/owned'),
      {
        method: 'GET',
        path: '/sheets/owned',
        respond: () => ok([ownedSheet(linked ? { campaign_id: CAMPAIGN_ID } : {})]),
      },
    ]);
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.selectOptions(await screen.findByLabelText(message('campaign.linkLabel')), SHEET_ID);
    await user.click(screen.getByRole('button', { name: message('campaign.link') }));

    expect(
      await screen.findByRole('link', {
        name: messageWith('campaign.openCharacter', { name: 'Brenna' }),
      }),
    ).toBeInTheDocument();
    expect(script.calls('GET', '/sheets/owned').length).toBeGreaterThanOrEqual(2);
  });
});

describe('campaign screen: linked sheets and GM read (LGN-218)', () => {
  it('Happy path: a GM sees the linked character sheets as links to the read-only view', async () => {
    const script = scripted([
      ...detailRoutes(campaignView({ viewer_role: 'gm', viewer_owner: false })).filter(
        (route) => route.path !== `${DETAIL}/sheets`,
      ),
      {
        method: 'GET',
        path: `${DETAIL}/sheets`,
        respond: () =>
          ok([
            {
              id: SHEET_ID,
              name: 'Brenna',
              template_id: 'fantasy_v1',
              owner_id: PLAYER_ID,
              updated_at: '2026-11-17T10:00:00Z',
            },
          ]),
      },
    ]);
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const link = await screen.findByRole('link', {
      name: messageWith('campaign.openSheet', { name: 'Brenna' }),
    });
    expect(link).toHaveAttribute('href', `${DETAIL}/sheets/${SHEET_ID}`);
    const card = link.closest('li') as HTMLElement;
    expect(
      within(card).getByText(messageWith('campaign.playedBy', { name: 'tomas' })),
    ).toBeInTheDocument();
    expect(within(card).getByText(/Updated Nov 17, 2026/)).toBeInTheDocument();
  });

  it('Edge: the sheet grid shows a loading line while the list is in flight', async () => {
    const script = scripted([
      ...detailRoutes(campaignView({ viewer_role: 'gm', viewer_owner: false })).filter(
        (route) => route.path !== `${DETAIL}/sheets`,
      ),
      { method: 'GET', path: `${DETAIL}/sheets`, respond: () => new Promise(() => undefined) },
    ]);
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const section = (
      await screen.findByRole('heading', { name: message('campaign.sheetsHeading') })
    ).closest('section') as HTMLElement;
    expect(await within(section).findByRole('status')).toHaveTextContent(
      message('campaign.listLoading'),
    );
  });

  it('Edge: a sheet whose owner is no longer a member shows no player and a bad date no update line', async () => {
    const script = scripted([
      ...detailRoutes(campaignView({ viewer_role: 'gm', viewer_owner: false })).filter(
        (route) => route.path !== `${DETAIL}/sheets`,
      ),
      {
        method: 'GET',
        path: `${DETAIL}/sheets`,
        respond: () =>
          ok([
            {
              id: SHEET_ID,
              name: 'Brenna',
              template_id: 'fantasy_v1',
              owner_id: 'departed-user',
              updated_at: 'not-a-date',
            },
          ]),
      },
    ]);
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const link = await screen.findByRole('link', {
      name: messageWith('campaign.openSheet', { name: 'Brenna' }),
    });
    const card = link.closest('li') as HTMLElement;
    expect(within(card).queryByText(/Played by/)).not.toBeInTheDocument();
    expect(within(card).queryByText(/Updated/)).not.toBeInTheDocument();
  });

  it('Edge: a player never requests the GM sheet list', async () => {
    const script = scripted(detailRoutes(playerView()));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    expect(script.calls('GET', `${DETAIL}/sheets`)).toHaveLength(0);
  });
});

describe('campaign screen: leave and delete (LGN-218)', () => {
  it('Happy path: a player leaves after confirming and returns to the list', async () => {
    const script = scripted([
      ...detailRoutes(playerView(), [
        { method: 'POST', path: `${DETAIL}/leave`, respond: () => ok(null) },
      ]),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.click(screen.getByRole('button', { name: message('campaign.leave') }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: message('campaign.leaveConfirm'),
      }),
    );

    expect(await screen.findByText(message('campaigns.empty'))).toBeInTheDocument();
    expect(script.calls('POST', `${DETAIL}/leave`)).toHaveLength(1);
  });

  it('Happy path: the owner deletes only after the dialog confirmation and returns to the list', async () => {
    const script = scripted([
      ...detailRoutes(campaignView(), [
        {
          method: 'DELETE',
          path: DETAIL,
          respond: () => ({ status: 202, body: JSON.stringify({ data: { status: 'deleting' } }) }),
        },
      ]),
      { method: 'GET', path: '/campaigns', respond: () => ok([]) },
    ]);
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.click(screen.getByRole('button', { name: message('campaign.settingsHeading') }));
    await user.click(screen.getByRole('button', { name: message('campaign.delete') }));
    expect(script.calls('DELETE', DETAIL)).toHaveLength(0);

    const dialog = await screen.findByRole('dialog');
    await user.click(
      within(dialog).getByRole('button', { name: message('campaign.deleteConfirm') }),
    );

    expect(await screen.findByText(message('campaigns.empty'))).toBeInTheDocument();
    expect(script.calls('DELETE', DETAIL)).toHaveLength(1);
  });

  it('Error: a delete refused as forbidden shows the copy and stays on the page', async () => {
    const script = scripted(
      detailRoutes(campaignView(), [
        { method: 'DELETE', path: DETAIL, respond: () => fail('CAMPAIGN_FORBIDDEN', 403) },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.click(screen.getByRole('button', { name: message('campaign.settingsHeading') }));
    await user.click(screen.getByRole('button', { name: message('campaign.delete') }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: message('campaign.deleteConfirm'),
      }),
    );

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.campaignForbidden'),
    );
    expect(screen.getByRole('heading', { name: 'Ember Coast', level: 1 })).toBeInTheDocument();
  });

  it('Edge: a campaign in the deleting state shows the deleting notice and no destructive action', async () => {
    const script = scripted(detailRoutes(campaignView({ status: 'deleting' })));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(message('campaign.deleting'))).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: message('campaign.settingsHeading') }));
    expect(screen.getByRole('button', { name: message('campaign.delete') })).toBeDisabled();
  });

  it('Edge: a campaign that is gone or deleting reads as not found with its copy', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: DETAIL, respond: () => fail('CAMPAIGN_NOT_FOUND', 404) },
    ]);
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(message('campaign.notFound'))).toBeInTheDocument();
  });

  it('Error: a forbidden read of the campaign shows the forbidden copy', async () => {
    const script = scripted([
      ...baseRoutes(),
      { method: 'GET', path: DETAIL, respond: () => fail('CAMPAIGN_FORBIDDEN', 403) },
    ]);
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.campaignForbidden'),
    );
  });
});

describe('campaign screen: sensitive panels stay collapsed (LGN-218)', () => {
  const CODE = 'DetailProbeCode-0123456789';

  it.each([
    ['the invite code', () => screen.queryByText(CODE)],
    ['the share link', () => screen.queryByLabelText(message('invite.linkLabel'))],
    [
      'the join password form',
      () => screen.queryByLabelText(message('invite.passwordLabel'), { exact: false }),
    ],
    ['the delete action', () => screen.queryByRole('button', { name: message('campaign.delete') })],
  ])('Edge: %s is not in the DOM until its panel is opened', async (_label, find) => {
    const script = scripted(detailRoutes(campaignView({ invite_code: CODE })));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Ember Coast', level: 1 });
    expect(find()).not.toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
