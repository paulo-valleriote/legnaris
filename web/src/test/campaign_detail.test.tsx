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
    ...extra,
  ];
}

function playerView() {
  return campaignView({ viewer_role: 'player', viewer_owner: false });
}

describe('campaign screen: members and roles (LGN-218)', () => {
  it('Happy path: the owner sees every member with role, character and the actions the owner may take', async () => {
    const script = scripted(detailRoutes(campaignView()));
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
    expect(screen.getByLabelText(message('campaign.linkLabel'))).toBeInTheDocument();
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
    await user.type(screen.getByLabelText(message('campaign.linkLabel')), SHEET_ID);
    await user.click(screen.getByRole('button', { name: message('campaign.link') }));

    expect(await screen.findByText(message('campaign.linked'))).toBeInTheDocument();
    expect(JSON.parse(script.calls('PUT', `${DETAIL}/character`)[0]?.request.body ?? '')).toEqual({
      sheet_id: SHEET_ID,
    });
  });

  it('Happy path: a player unlinks the character', async () => {
    const script = scripted(
      detailRoutes(playerView(), [
        { method: 'DELETE', path: `${DETAIL}/character`, respond: () => ok(playerView()) },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    await user.click(screen.getByRole('button', { name: message('campaign.unlink') }));

    expect(await screen.findByText(message('campaign.unlinked'))).toBeInTheDocument();
    expect(script.calls('DELETE', `${DETAIL}/character`)).toHaveLength(1);
  });

  it.each([
    ['TEMPLATE_MISMATCH', 'api.error.templateMismatch'],
    ['SHEET_ALREADY_LINKED', 'api.error.sheetAlreadyLinked'],
    ['NOT_FOUND', 'api.error.notFound'],
    ['INVALID_REQUEST', 'api.error.invalidEdit'],
  ] as const)(
    'Error: linking refused with %s shows its copy and keeps the typed id',
    async (code, key) => {
      const script = scripted(
        detailRoutes(playerView(), [
          { method: 'PUT', path: `${DETAIL}/character`, respond: () => fail(code, 422) },
        ]),
      );
      const user = userEvent.setup();
      renderApp(DETAIL, { client: clientFor(script.transport) });

      await screen.findByText('tomas');
      await user.type(screen.getByLabelText(message('campaign.linkLabel')), OTHER_SHEET_ID);
      await user.click(screen.getByRole('button', { name: message('campaign.link') }));

      expect(await screen.findByRole('alert')).toHaveTextContent(message(key));
      expect(screen.getByLabelText(message('campaign.linkLabel'))).toHaveValue(OTHER_SHEET_ID);
    },
  );

  it('Edge: a blank sheet id is not sent', async () => {
    const script = scripted(detailRoutes(playerView()));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    expect(screen.getByRole('button', { name: message('campaign.link') })).toBeDisabled();
    expect(script.calls('PUT', `${DETAIL}/character`)).toHaveLength(0);
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
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(message('campaign.deleting'))).toBeInTheDocument();
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
