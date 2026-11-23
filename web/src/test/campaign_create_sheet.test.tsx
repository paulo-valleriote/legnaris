import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import type { HttpRequest, HttpResponse } from '../api/transport';
import { failureMessage } from '../api/error_mapper';
import { message, messageWith } from '../l10n/strings';
import {
  CAMPAIGN_ID,
  SHEET_ID,
  TEMPLATE_ID,
  TEMPLATE_NAME,
  baseRoutes,
  campaignView,
  capturedTemplates,
  clientFor,
  fail,
  ok,
  scripted,
} from './campaign_helpers';
import type { Route } from './campaign_helpers';
import { renderApp } from './render';

const DETAIL = `/campaigns/${CAMPAIGN_ID}`;
const CHARACTER = `${DETAIL}/character`;
const NPCS = `${DETAIL}/npcs`;
const CREATE = '/sheets';

interface RouteOptions {
  readonly templateId?: string;
  readonly detail?: Record<string, unknown>;
  readonly owned?: (request: HttpRequest) => unknown[];
}

function templateDetail(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: TEMPLATE_ID,
    name: TEMPLATE_NAME,
    default: true,
    kinds: ['character', 'npc', 'enemy'],
    fields: {},
    ...overrides,
  };
}

function createdSheet(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: SHEET_ID,
    template_id: TEMPLATE_ID,
    kind: 'character',
    name: 'New Hero',
    data: {},
    created_at: '2026-11-17T10:00:00Z',
    updated_at: '2026-11-17T10:00:00Z',
    ...overrides,
  };
}

function ownedSheet(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: SHEET_ID,
    name: 'Brenna',
    template_id: TEMPLATE_ID,
    kind: 'character',
    data: {},
    campaign_id: CAMPAIGN_ID,
    created_at: '2026-11-17T10:00:00Z',
    updated_at: '2026-11-17T10:00:00Z',
    ...overrides,
  };
}

function campaignRoutes(
  view: Record<string, unknown>,
  extra: Route[] = [],
  options: RouteOptions = {},
): Route[] {
  const templateId = options.templateId ?? TEMPLATE_ID;
  return [
    ...baseRoutes(),
    { method: 'GET', path: DETAIL, respond: () => ok(view) },
    { method: 'GET', path: `${DETAIL}/sheets`, respond: () => ok([]) },
    { method: 'GET', path: NPCS, respond: () => ok([]) },
    {
      method: 'GET',
      path: '/sheets/owned',
      respond: (request) => ok(options.owned?.(request) ?? []),
    },
    {
      method: 'GET',
      path: `/templates/${templateId}`,
      respond: () => ok(options.detail ?? templateDetail({ id: templateId })),
    },
    ...extra,
  ];
}

function playerView(overrides: Record<string, unknown> = {}) {
  return campaignView({
    viewer_role: 'player',
    viewer_owner: false,
    viewer_user_id: 'viewer-player',
    ...overrides,
  });
}

function gmView(overrides: Record<string, unknown> = {}) {
  return campaignView({ viewer_role: 'gm', viewer_owner: false, ...overrides });
}

function newCharacter() {
  return { name: messageWith('campaign.newCharacterFor', { name: TEMPLATE_NAME }) };
}

function newNpc() {
  return { name: messageWith('campaign.newNpcFor', { name: TEMPLATE_NAME }) };
}

function appQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function renderCampaign(script: ReturnType<typeof scripted>) {
  return renderApp(DETAIL, {
    client: clientFor(script.transport),
    queryClient: appQueryClient(),
  });
}

async function openCreateDialog(script: ReturnType<typeof scripted>, buttonName: { name: string }) {
  renderCampaign(script);
  const user = userEvent.setup();
  await user.click(await screen.findByRole('button', buttonName));
  const dialog = await screen.findByRole('dialog');
  return { user, dialog };
}

function createBody(script: ReturnType<typeof scripted>): Record<string, unknown> {
  return JSON.parse(script.calls('POST', CREATE)[0]?.request.body ?? '') as Record<string, unknown>;
}

describe('campaign screen: create and link a new sheet (LGN-403)', () => {
  it('Happy path: a Player creates a character from a scope Template, links it and opens the Sheet', async () => {
    const view = playerView();
    const script = scripted(
      campaignRoutes(view, [
        { method: 'POST', path: CREATE, respond: () => ok(createdSheet(), 201) },
        { method: 'PUT', path: CHARACTER, respond: () => ok(view) },
      ]),
    );
    const user = userEvent.setup();
    renderCampaign(script);

    await user.click(await screen.findByRole('button', newCharacter()));
    const dialog = await screen.findByRole('dialog');

    expect(within(dialog).getByText(message('catalog.kindCharacter'))).toBeInTheDocument();
    expect(within(dialog).queryByRole('link', { name: message('generate.entry') })).toBeNull();
    expect(within(dialog).queryByRole('button', { name: message('generate.entry') })).toBeNull();

    await user.type(within(dialog).getByLabelText(message('sheet.nameLabel')), 'New Hero');
    await user.click(within(dialog).getByRole('button', { name: message('sheet.create') }));

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('New Hero');
    expect(createBody(script)).toEqual({
      template_id: TEMPLATE_ID,
      kind: 'character',
      sheet_name: 'New Hero',
      campaign_id: CAMPAIGN_ID,
    });
    expect(JSON.parse(script.calls('PUT', CHARACTER)[0]?.request.body ?? '')).toEqual({
      sheet_id: SHEET_ID,
    });
  });

  it('Happy path: a GM creates an NPC with only NPC and Enemy kinds and links it as an NPC', async () => {
    const view = gmView();
    const script = scripted(
      campaignRoutes(view, [
        {
          method: 'POST',
          path: CREATE,
          respond: () => ok(createdSheet({ kind: 'npc', name: 'Goblin' }), 201),
        },
        { method: 'PUT', path: NPCS, respond: () => ok(npcLink()) },
      ]),
    );
    const user = userEvent.setup();
    renderCampaign(script);

    await user.click(await screen.findByRole('button', newNpc()));
    const dialog = await screen.findByRole('dialog');
    const kindSelect = within(dialog).getByLabelText(message('catalog.kindLabel'));

    expect(kindSelect).toHaveValue('npc');
    expect(within(dialog).getByRole('option', { name: message('kind.npc') })).toBeInTheDocument();
    expect(within(dialog).getByRole('option', { name: message('kind.enemy') })).toBeInTheDocument();
    expect(
      within(dialog).queryByRole('option', { name: message('catalog.kindCharacter') }),
    ).toBeNull();

    await user.type(within(dialog).getByLabelText(message('sheet.nameLabel')), 'Goblin');
    await user.click(within(dialog).getByRole('button', { name: message('sheet.create') }));

    expect(await screen.findByTestId('sheet-screen')).toBeInTheDocument();
    expect(createBody(script)).toEqual({
      template_id: TEMPLATE_ID,
      kind: 'npc',
      sheet_name: 'Goblin',
      campaign_id: CAMPAIGN_ID,
    });
    expect(JSON.parse(script.calls('PUT', NPCS)[0]?.request.body ?? '')).toEqual({
      sheet_id: SHEET_ID,
    });
  });

  it('Edge: a Player gets no button when no Template accepts a character', async () => {
    const script = scripted(
      campaignRoutes(playerView(), [], {
        detail: templateDetail({ kinds: ['npc', 'enemy'] }),
      }),
    );
    renderCampaign(script);

    await screen.findByText('tomas');
    await waitFor(() => expect(script.calls('GET', `/templates/${TEMPLATE_ID}`)).toHaveLength(1));
    expect(screen.queryByRole('button', newCharacter())).toBeNull();
    expect(screen.queryByRole('button', { name: /New character/ })).toBeNull();
  });

  it('Edge: a GM gets no button when no Template accepts an NPC or Enemy', async () => {
    const script = scripted(
      campaignRoutes(gmView(), [], { detail: templateDetail({ kinds: ['character'] }) }),
    );
    renderCampaign(script);

    await screen.findByText('tomas');
    await waitFor(() => expect(script.calls('GET', `/templates/${TEMPLATE_ID}`)).toHaveLength(1));
    expect(screen.queryByRole('button', newNpc())).toBeNull();
    expect(screen.queryByRole('button', { name: /New NPC/ })).toBeNull();
  });

  it('Edge: a Campaign with no Templates offers no create button', async () => {
    const script = scripted(campaignRoutes(playerView({ template_ids: [] })));
    renderCampaign(script);

    await screen.findByText('tomas');
    expect(screen.queryByRole('button', { name: /New character/ })).toBeNull();
    expect(script.calls('GET', `/templates/${TEMPLATE_ID}`)).toHaveLength(0);
  });

  it('Edge: a failed Template detail offers no button and keeps the campaign readable', async () => {
    const script = scripted([
      ...campaignRoutes(playerView()).filter((route) => route.path !== `/templates/${TEMPLATE_ID}`),
      {
        method: 'GET',
        path: `/templates/${TEMPLATE_ID}`,
        respond: () => fail('INTERNAL_ERROR', 500),
      },
    ]);
    renderCampaign(script);

    expect(
      await screen.findByRole('heading', { name: 'Ember Coast', level: 1 }),
    ).toBeInTheDocument();
    await waitFor(() => expect(script.calls('GET', `/templates/${TEMPLATE_ID}`)).toHaveLength(1));
    expect(screen.queryByRole('button', { name: /New character/ })).toBeNull();
  });

  it('Happy path: a Campaign fixed to a Retired Template creates through the detail read', async () => {
    const retiredId = 'retired_v1';
    expect(capturedTemplates().some((entry) => (entry as { id?: string }).id === retiredId)).toBe(
      false,
    );
    const view = playerView({
      template_id: retiredId,
      scope_name: 'Retired Archive',
      template_ids: [retiredId],
    });
    const script = scripted(
      campaignRoutes(
        view,
        [
          {
            method: 'POST',
            path: CREATE,
            respond: () => ok(createdSheet({ template_id: retiredId, name: 'Old Hero' }), 201),
          },
          { method: 'PUT', path: CHARACTER, respond: () => ok(view) },
        ],
        {
          templateId: retiredId,
          detail: templateDetail({ id: retiredId, name: 'Retired Archive' }),
        },
      ),
    );
    const user = userEvent.setup();
    renderCampaign(script);

    const button = await screen.findByRole('button', {
      name: messageWith('campaign.newCharacterFor', { name: 'Retired Archive' }),
    });
    await user.click(button);
    const dialog = await screen.findByRole('dialog');
    await user.type(within(dialog).getByLabelText(message('sheet.nameLabel')), 'Old Hero');
    await user.click(within(dialog).getByRole('button', { name: message('sheet.create') }));

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Old Hero');
    expect(createBody(script)).toEqual({
      template_id: retiredId,
      kind: 'character',
      sheet_name: 'Old Hero',
      campaign_id: CAMPAIGN_ID,
    });
  });

  it('Error: a refused creation shows the plan copy, links nothing and keeps the dialog open', async () => {
    const script = scripted(
      campaignRoutes(playerView(), [
        { method: 'POST', path: CREATE, respond: () => fail('SHEET_LIMIT_REACHED', 409) },
      ]),
    );
    const { user, dialog } = await openCreateDialog(script, newCharacter());

    await user.type(within(dialog).getByLabelText(message('sheet.nameLabel')), 'New Hero');
    await user.click(within(dialog).getByRole('button', { name: message('sheet.create') }));

    expect(await screen.findByTestId('create-error')).toHaveTextContent(
      failureMessage('sheetLimitReached'),
    );
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(script.calls('POST', CREATE)).toHaveLength(1);
    expect(script.calls('PUT', CHARACTER)).toHaveLength(0);
  });

  it('Error: a failed link keeps the created Sheet and retries only the link', async () => {
    const view = playerView();
    let linkAttempts = 0;
    const script = scripted(
      campaignRoutes(view, [
        { method: 'POST', path: CREATE, respond: () => ok(createdSheet(), 201) },
        {
          method: 'PUT',
          path: CHARACTER,
          respond: () => {
            linkAttempts += 1;
            return linkAttempts === 1 ? fail('TEMPLATE_MISMATCH', 422) : ok(view);
          },
        },
      ]),
    );
    const { user, dialog } = await openCreateDialog(script, newCharacter());

    await user.type(within(dialog).getByLabelText(message('sheet.nameLabel')), 'New Hero');
    await user.click(within(dialog).getByRole('button', { name: message('sheet.create') }));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(message('campaign.createLinkFailed'));
    expect(alert).toHaveTextContent(failureMessage('templateMismatch'));
    expect(script.calls('POST', CREATE)).toHaveLength(1);

    await user.click(screen.getByRole('button', { name: message('campaign.createLinkRetry') }));

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('New Hero');
    expect(script.calls('POST', CREATE)).toHaveLength(1);
    expect(script.calls('PUT', CHARACTER)).toHaveLength(2);
  });

  it('Edge: a double activation of Create sends exactly one creation request', async () => {
    const script = scripted(
      campaignRoutes(playerView(), [
        {
          method: 'POST',
          path: CREATE,
          respond: () => new Promise<HttpResponse>(() => undefined),
        },
      ]),
    );
    const { user, dialog } = await openCreateDialog(script, newCharacter());

    await user.type(within(dialog).getByLabelText(message('sheet.nameLabel')), 'New Hero');
    await user.click(within(dialog).getByRole('button', { name: message('sheet.create') }));

    const busy = await screen.findByRole('button', { name: message('sheet.creating') });
    fireEvent.click(busy);

    expect(script.calls('POST', CREATE)).toHaveLength(1);
  });

  it('Edge: a Player who already has a character sees the replace warning before creating', async () => {
    const script = scripted(
      campaignRoutes(playerView(), [], {
        owned: (request) =>
          new URL(request.url).searchParams.get('kind') === 'character' ? [ownedSheet()] : [],
      }),
    );
    const { dialog } = await openCreateDialog(script, newCharacter());

    expect(within(dialog).getByText(message('campaign.createReplaceWarning'))).toBeInTheDocument();
    expect(script.calls('POST', CREATE)).toHaveLength(0);
  });
});

function npcLink(): Record<string, unknown> {
  return {
    id: '0190c5a0-0000-7000-8000-00000000c009',
    sheet_id: SHEET_ID,
    name: 'Goblin',
    kind: 'npc',
    owner_id: '0190c5a0-0000-7000-8000-00000000c004',
    owner_name: 'mara',
    revealed: false,
    linked_at: '2026-11-17T10:00:00Z',
    is_owner: true,
  };
}
