import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { DECLARED_ROUTES } from '../app/routes';
import { message, messageWith } from '../l10n/strings';
import { campaignView } from './campaign_helpers';
import { errorOutcome, FREE_PLAN_USAGE, successOutcome } from './fake_transport';
import {
  FIFTH_CHARACTER_SHEET_ID,
  FIFTH_CHARACTER_TEMPLATE_ID,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import { renderApp } from './render';

const CODE = 'Abcdefghijklmnopqrstuvwx';
const CAMPAIGN_WIRE = campaignView({
  template_id: FIFTH_CHARACTER_TEMPLATE_ID,
  template_ids: [FIFTH_CHARACTER_TEMPLATE_ID],
  scope_name: '5E',
  viewer_role: 'player',
  viewer_owner: false,
});
const INVITE = {
  name: 'Ember Coast',
  gm_username: 'mara',
  scope_name: '5E',
  template_ids: [FIFTH_CHARACTER_TEMPLATE_ID],
  template_id: FIFTH_CHARACTER_TEMPLATE_ID,
  password_required: false,
  player_count: 2,
  already_member: false,
};
const CAMPAIGN_PATH = `/campaigns/${CAMPAIGN_WIRE.id}`;
const JOIN = '/campaigns/join';

interface JoinOptions {
  readonly invite?: HttpResponse;
  readonly preview?: (password: string) => HttpResponse;
  readonly join?: HttpResponse;
  readonly owned?: readonly unknown[];
  readonly plan?: unknown;
}

function renderJoin(options: JoinOptions = {}) {
  const requests: HttpRequest[] = [];
  const transport: Transport = (request) => {
    requests.push(request);
    const url = new URL(request.url);
    const path = url.pathname;
    if (path === `/campaigns/invite/${CODE}` && request.method === 'GET') {
      return Promise.resolve(options.invite ?? successOutcome(INVITE));
    }
    if (path === `/campaigns/invite/${CODE}/preview` && request.method === 'POST') {
      const password = String(JSON.parse(request.body ?? '{}').password ?? '');
      return Promise.resolve(options.preview?.(password) ?? successOutcome(INVITE));
    }
    if (path === JOIN && request.method === 'POST') {
      return Promise.resolve(options.join ?? successOutcome(CAMPAIGN_WIRE));
    }
    if (path === '/sheets/owned') {
      return Promise.resolve(successOutcome(options.owned ?? []));
    }
    if (path === '/me/plan') {
      return Promise.resolve(successOutcome(options.plan ?? FREE_PLAN_USAGE));
    }
    if (path === `/templates/${FIFTH_CHARACTER_TEMPLATE_ID}`) {
      return Promise.resolve(successOutcome(fifthCharacterTemplatePayload()));
    }
    if (path === '/sheets' && request.method === 'POST') {
      return Promise.resolve(successOutcome(fifthCharacterSheetPayload(), 201));
    }
    if (path === CAMPAIGN_PATH) {
      return Promise.resolve(successOutcome(CAMPAIGN_WIRE));
    }
    if (request.method === 'GET') {
      return Promise.resolve(successOutcome([]));
    }
    return Promise.resolve(successOutcome({}));
  };
  renderApp(`/join/${CODE}`, {
    client: createApiClient({ baseUrl: 'http://api.test', transport }),
  });
  return requests;
}

function joinBody(requests: readonly HttpRequest[]) {
  const request = requests.find(
    (entry) => entry.method === 'POST' && new URL(entry.url).pathname === JOIN,
  );
  return request === undefined ? undefined : JSON.parse(request.body ?? '{}');
}

describe('join route (LGN-200, LGN-502)', () => {
  it('Happy path: the join route is declared as a session route with a deep link', () => {
    const route = DECLARED_ROUTES.find((declared) => declared.path === '/join/:code');

    expect(route).toMatchObject({ audience: 'session', deepLink: true, notFound: false });
  });

  it('Happy path: an open invite shows the campaign and joins with the chosen sheet', async () => {
    const requests = renderJoin({
      owned: [fifthCharacterSheetPayload()],
    });
    const user = userEvent.setup();

    expect(await screen.findByRole('heading', { name: 'Ember Coast' })).toBeInTheDocument();
    expect(screen.getByText(messageWith('join.gm', { name: 'mara' }))).toBeInTheDocument();
    expect(screen.getByText('5E')).toBeInTheDocument();
    expect(screen.getByText(messageWith('join.players', { count: 2 }))).toBeInTheDocument();

    await user.selectOptions(
      await screen.findByLabelText(message('join.sheetLabel')),
      FIFTH_CHARACTER_SHEET_ID,
    );
    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByTestId('campaign-joined-notice')).toHaveTextContent(
      messageWith('campaign.joinedNotice', { name: CAMPAIGN_WIRE.name }),
    );
    expect(joinBody(requests)).toEqual({ code: CODE, sheet_id: FIFTH_CHARACTER_SHEET_ID });
  });

  it('Happy path: joining without a sheet sends no sheet id', async () => {
    const requests = renderJoin({ owned: [fifthCharacterSheetPayload()] });
    const user = userEvent.setup();

    await screen.findByRole('heading', { name: 'Ember Coast' });
    await user.click(await screen.findByRole('button', { name: message('join.submit') }));

    await waitFor(() => expect(joinBody(requests)).toEqual({ code: CODE }));
  });

  it('Happy path: an invite with a password reveals the details only after the correct one', async () => {
    const requests = renderJoin({
      invite: errorOutcome('PASSWORD_REQUIRED', 'password', 403),
      preview: (password) =>
        password === 'right horse'
          ? successOutcome(INVITE)
          : errorOutcome('INVALID_INVITE', 'invalid', 404),
    });
    const user = userEvent.setup();

    expect(
      await screen.findByRole('heading', { name: message('join.passwordTitle') }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Ember Coast')).not.toBeInTheDocument();

    const password = screen.getByLabelText(message('join.passwordLabel'));
    await user.type(password, 'wrong horse');
    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    await waitFor(() => expect(password).toHaveAttribute('aria-invalid', 'true'));
    expect(document.getElementById('join-password-error')).toHaveTextContent(
      message('api.error.invalidInvite'),
    );
    expect(screen.queryByText('Ember Coast')).not.toBeInTheDocument();

    await user.clear(password);
    await user.type(password, 'right horse');
    await user.click(screen.getByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('heading', { name: 'Ember Coast' })).toBeInTheDocument();
    expect(screen.getByText(messageWith('join.gm', { name: 'mara' }))).toBeInTheDocument();
    expect(joinBody(requests)).toBeUndefined();
  });

  it('Happy path: an existing member goes straight to the campaign', async () => {
    const requests = renderJoin({
      invite: successOutcome({ ...INVITE, already_member: true }),
    });

    expect(await screen.findByTestId('campaign-joined-notice')).toHaveTextContent(
      messageWith('campaign.joinedNotice', { name: CAMPAIGN_WIRE.name }),
    );
    await waitFor(() => expect(joinBody(requests)).toEqual({ code: CODE }));
  });

  it('Edge: an invalid invite shows the invalid state and no form', async () => {
    renderJoin({ invite: errorOutcome('INVALID_INVITE', 'invalid', 404) });

    expect(
      await screen.findByRole('heading', { name: message('join.invalidTitle') }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText(message('join.sheetLabel'))).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('join.submit') })).not.toBeInTheDocument();
    expect(screen.getByText(message('join.invalidBody'))).toBeInTheDocument();
  });

  it('Edge: no eligible sheet offers creating one or joining without a sheet', async () => {
    const requests = renderJoin({ owned: [] });
    const user = userEvent.setup();

    await screen.findByRole('heading', { name: 'Ember Coast' });
    expect(screen.getByText(message('join.noEligible'))).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('join.createSheet') }));
    await user.type(await screen.findByLabelText(message('sheet.nameLabel')), 'Brenna');
    await user.click(screen.getByRole('button', { name: message('sheet.create') }));

    await waitFor(() =>
      expect(joinBody(requests)).toEqual({ code: CODE, sheet_id: FIFTH_CHARACTER_SHEET_ID }),
    );
  });

  it('Edge: a reached character limit explains the plan limit and still allows joining without a sheet', async () => {
    const plan = {
      ...FREE_PLAN_USAGE,
      sheets: { ...FREE_PLAN_USAGE.sheets, character: { used: 1, limit: 1 } },
    };
    const requests = renderJoin({ owned: [], plan });
    const user = userEvent.setup();

    await screen.findByRole('heading', { name: 'Ember Coast' });
    expect(screen.getByText(message('join.planLimit'))).toBeInTheDocument();
    expect(screen.getByRole('link', { name: message('campaigns.seePlans') })).toHaveAttribute(
      'href',
      '/plans',
    );

    await user.click(screen.getByRole('button', { name: message('join.joinWithout') }));
    await waitFor(() => expect(joinBody(requests)).toEqual({ code: CODE }));
  });

  it('Error: a refused join shows the mapped copy and keeps the invite', async () => {
    renderJoin({
      owned: [fifthCharacterSheetPayload()],
      join: errorOutcome('TEMPLATE_MISMATCH', 'mismatch', 422),
    });
    const user = userEvent.setup();

    await screen.findByRole('heading', { name: 'Ember Coast' });
    await user.selectOptions(
      await screen.findByLabelText(message('join.sheetLabel')),
      FIFTH_CHARACTER_SHEET_ID,
    );
    await user.click(await screen.findByRole('button', { name: message('join.submit') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.templateMismatch'),
    );
    expect(screen.getByRole('heading', { name: 'Ember Coast' })).toBeInTheDocument();
  });
});
