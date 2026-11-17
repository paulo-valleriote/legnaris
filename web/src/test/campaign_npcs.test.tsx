import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { HttpRequest } from '../api/transport';
import { message, messageWith } from '../l10n/strings';
import {
  CAMPAIGN_ID,
  MEMBER_ID,
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
const NPCS = `${DETAIL}/npcs`;
const UNLINK = `${NPCS}/${SHEET_ID}`;

function ownedSheet(overrides: Record<string, unknown> = {}) {
  return {
    id: SHEET_ID,
    name: 'Goblin',
    template_id: 'fantasy_v1',
    kind: 'npc',
    data: {},
    created_at: '2026-11-17T10:00:00Z',
    updated_at: '2026-11-17T10:00:00Z',
    ...overrides,
  };
}

function npcLink(overrides: Record<string, unknown> = {}) {
  return {
    id: '0190c5a0-0000-7000-8000-00000000c009',
    sheet_id: SHEET_ID,
    name: 'Goblin',
    kind: 'npc',
    owner_id: MEMBER_ID,
    owner_name: 'mara',
    revealed: false,
    linked_at: '2026-11-17T10:00:00Z',
    is_owner: true,
    ...overrides,
  };
}

interface World {
  readonly links: () => unknown[];
  readonly link?: Route['respond'];
  readonly unlink?: Route['respond'];
  readonly ownedNpcs?: unknown[];
  readonly ownedMonsters?: unknown[];
}

function npcRoutes(world: World, view = campaignView()): Route[] {
  return [
    ...baseRoutes(),
    { method: 'GET', path: DETAIL, respond: () => ok(view) },
    { method: 'GET', path: `${DETAIL}/sheets`, respond: () => ok([]) },
    {
      method: 'GET',
      path: '/sheets/owned',
      respond: (request: HttpRequest) =>
        ok(
          new URL(request.url).searchParams.get('kind') === 'monster'
            ? (world.ownedMonsters ?? [])
            : (world.ownedNpcs ?? [ownedSheet()]),
        ),
    },
    { method: 'GET', path: NPCS, respond: () => ok(world.links()) },
    ...(world.link === undefined ? [] : [{ method: 'PUT', path: NPCS, respond: world.link }]),
    {
      method: 'DELETE',
      path: UNLINK,
      respond: world.unlink ?? (() => ({ status: 204, body: '' })),
    },
  ];
}

function openNpc(name: string) {
  return { name: messageWith('campaign.openNpc', { name }) };
}

describe('campaign screen: NPC and Monster links (LGN-219)', () => {
  it('Happy path: a GM links an owned NPC, sees it hidden, and unlinks it', async () => {
    let links: unknown[] = [];
    const script = scripted(
      npcRoutes({
        links: () => links,
        link: () => {
          links = [npcLink()];
          return ok(npcLink());
        },
        unlink: () => {
          links = [];
          return { status: 204, body: '' };
        },
      }),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByLabelText(message('campaign.npcLinkLabel'))).toHaveValue(SHEET_ID);
    await user.click(screen.getByRole('button', { name: message('campaign.npcLink') }));

    expect(await screen.findByText(message('campaign.npcLinked'))).toBeInTheDocument();
    const row = (await screen.findByRole('link', openNpc('Goblin'))).closest('li');
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByText(message('campaign.npcHidden'))).toBeInTheDocument();
    expect(within(row as HTMLElement).getByText(/Owner: mara/)).toBeInTheDocument();
    expect(JSON.parse(script.calls('PUT', NPCS)[0]?.request.body ?? '')).toEqual({
      sheet_id: SHEET_ID,
    });

    await user.click(
      screen.getByRole('button', { name: messageWith('campaign.unlinkNpc', { name: 'Goblin' }) }),
    );

    await waitFor(() =>
      expect(screen.queryByRole('link', openNpc('Goblin'))).not.toBeInTheDocument(),
    );
    expect(script.calls('DELETE', UNLINK)).toHaveLength(1);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('Edge: re-linking the same sheet is a no-op and the list still shows one row', async () => {
    const script = scripted(
      npcRoutes({
        links: () => [npcLink()],
        link: () => ok(npcLink()),
      }),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByRole('link', openNpc('Goblin'));
    await user.click(screen.getByRole('button', { name: message('campaign.npcLink') }));
    await waitFor(() => expect(script.calls('PUT', NPCS)).toHaveLength(1));
    await user.click(screen.getByRole('button', { name: message('campaign.npcLink') }));

    await waitFor(() => expect(script.calls('PUT', NPCS)).toHaveLength(2));
    expect(screen.getAllByRole('link', openNpc('Goblin'))).toHaveLength(1);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('Error: a link refused because the user is no longer a GM shows the forbidden copy', async () => {
    const script = scripted(
      npcRoutes({
        links: () => [],
        link: () => fail('CAMPAIGN_FORBIDDEN', 403),
      }),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByLabelText(message('campaign.npcLinkLabel'));
    await user.click(screen.getByRole('button', { name: message('campaign.npcLink') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.campaignForbidden'),
    );
    expect(screen.getByLabelText(message('campaign.npcLinkLabel'))).toHaveValue(SHEET_ID);
  });

  it('Edge: unlinking an already unlinked sheet is handled gracefully and refreshes the list', async () => {
    let links: unknown[] = [npcLink()];
    const script = scripted(
      npcRoutes({
        links: () => links,
        unlink: () => {
          links = [];
          return fail('NOT_FOUND', 404);
        },
      }),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByRole('link', openNpc('Goblin'));
    await user.click(
      screen.getByRole('button', { name: messageWith('campaign.unlinkNpc', { name: 'Goblin' }) }),
    );

    await waitFor(() =>
      expect(screen.queryByRole('link', openNpc('Goblin'))).not.toBeInTheDocument(),
    );
    expect(script.calls('DELETE', UNLINK)).toHaveLength(1);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('Edge: reveal state is read-only info and another GM sheet carries no unlink action', async () => {
    const script = scripted(
      npcRoutes({
        links: () => [npcLink({ revealed: true, is_owner: false, owner_name: 'tomas' })],
      }),
    );
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const row = (await screen.findByRole('link', openNpc('Goblin'))).closest('li');
    expect(
      within(row as HTMLElement).getByText(message('campaign.npcRevealed')),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', {
        name: messageWith('campaign.unlinkNpc', { name: 'Goblin' }),
      }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: /revealed/i })).not.toBeInTheDocument();
  });

  it('Edge: a player sees no NPC section and never requests the GM link list', async () => {
    const script = scripted(
      npcRoutes({ links: () => [] }, campaignView({ viewer_role: 'player', viewer_owner: false })),
    );
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    expect(
      screen.queryByRole('heading', { name: message('campaign.npcsHeading') }),
    ).not.toBeInTheDocument();
    expect(script.calls('GET', NPCS)).toHaveLength(0);
  });

  it.each([
    ['no owned NPC or Monster sheet', [], []],
    ['owned sheets of another Template', [{ ...ownedSheet(), template_id: 'other_v1' }], []],
  ])('Edge: %s leaves nothing to link and says why', async (_label, ownedNpcs, ownedMonsters) => {
    const script = scripted(npcRoutes({ links: () => [], ownedNpcs, ownedMonsters }));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(message('campaign.noEligibleNpcs'))).toBeInTheDocument();
    expect(
      screen.queryByRole('combobox', { name: message('campaign.npcLinkLabel') }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('campaign.npcLink') })).toBeDisabled();
  });

  it('Happy path: a Monster sheet is offered beside an NPC and linked with its kind', async () => {
    let links: unknown[] = [];
    const monster = ownedSheet({
      id: '0190c5a0-0000-7000-8000-00000000c00a',
      name: 'Owlbear',
      kind: 'monster',
    });
    const script = scripted(
      npcRoutes({
        links: () => links,
        ownedNpcs: [],
        ownedMonsters: [monster],
        link: () => {
          links = [npcLink({ sheet_id: monster.id, name: 'Owlbear', kind: 'monster' })];
          return ok(links[0]);
        },
      }),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    const select = await screen.findByLabelText(message('campaign.npcLinkLabel'));
    expect(within(select).getByRole('option', { name: 'Owlbear' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: message('campaign.npcLink') }));

    expect(await screen.findByRole('link', openNpc('Owlbear'))).toBeInTheDocument();
  });
});
