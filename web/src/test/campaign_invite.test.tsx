import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { UserEvent } from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { inviteLink } from '../campaign/invite_panel';
import {
  CAMPAIGN_ID,
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
const CODE = 'AbC123-_xyzQWERTYuiopAS';

function gmRoutes(view: Record<string, unknown>, extra: Route[] = []): Route[] {
  return [
    ...baseRoutes(),
    { method: 'GET', path: DETAIL, respond: () => ok(view) },
    { method: 'GET', path: `${DETAIL}/sheets`, respond: () => ok([]) },
    { method: 'GET', path: `${DETAIL}/npcs`, respond: () => ok([]) },
    { method: 'GET', path: '/sheets/owned', respond: () => ok([]) },
    ...extra,
  ];
}

function gmView(overrides: Record<string, unknown> = {}) {
  return campaignView({
    viewer_role: 'gm',
    viewer_owner: true,
    invite_code: CODE,
    password_set: false,
    ...overrides,
  });
}

function inviteToggle() {
  return screen.getByRole('button', { name: message('invite.heading') });
}

function settingsToggle() {
  return screen.getByRole('button', { name: message('campaign.settingsHeading') });
}

async function openInvite(user: UserEvent) {
  await user.click(await screen.findByRole('button', { name: message('invite.heading') }));
}

async function openSettings(user: UserEvent) {
  await user.click(
    await screen.findByRole('button', { name: message('campaign.settingsHeading') }),
  );
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('GM invite controls (LGN-200 on the campaign page)', () => {
  it('Happy path: a GM opens the invite panel and sees the code and its share link', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Ember Coast', level: 1 });
    expect(inviteToggle()).toHaveAttribute('aria-expanded', 'false');
    await openInvite(user);

    expect(inviteToggle()).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText(CODE)).toBeInTheDocument();
    expect(screen.getByLabelText(message('invite.linkLabel'))).toHaveValue(
      inviteLink(window.location.origin, CODE),
    );
  });

  it('Edge: a GM with no invite code yet sees the empty copy, not a broken link', async () => {
    const script = scripted(gmRoutes(gmView({ invite_code: undefined })));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openInvite(user);

    expect(await screen.findByText(message('invite.none'))).toBeInTheDocument();
    expect(screen.queryByLabelText(message('invite.linkLabel'))).not.toBeInTheDocument();
  });

  it('Edge: the toggles start collapsed and reveal nothing sensitive until opened', async () => {
    const script = scripted(gmRoutes(gmView()));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByRole('heading', { name: 'Ember Coast', level: 1 });
    expect(inviteToggle()).toHaveAttribute('aria-expanded', 'false');
    expect(settingsToggle()).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText(CODE)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(message('invite.linkLabel'))).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText(message('invite.passwordLabel'), { exact: false }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('campaign.delete') }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: an open panel closes again and its content leaves the DOM', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openInvite(user);
    expect(screen.getByText(CODE)).toBeInTheDocument();

    await openInvite(user);
    expect(inviteToggle()).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText(CODE)).not.toBeInTheDocument();
  });

  it('Happy path: both panels stay open independently', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openInvite(user);
    await openSettings(user);

    expect(inviteToggle()).toHaveAttribute('aria-expanded', 'true');
    expect(settingsToggle()).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText(CODE)).toBeInTheDocument();
    expect(
      screen.getByLabelText(message('invite.passwordLabel'), { exact: false }),
    ).toBeInTheDocument();
  });

  it('Happy path: a player never sees the invite or settings controls', async () => {
    const script = scripted(gmRoutes(campaignView({ viewer_role: 'player', viewer_owner: false })));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    expect(
      screen.queryByRole('button', { name: message('invite.heading') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('campaign.settingsHeading') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { name: message('invite.heading') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: a GM who does not own the Campaign sees Invite but not Settings', async () => {
    const script = scripted(gmRoutes(gmView({ viewer_owner: false })));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(
      await screen.findByRole('button', { name: message('invite.heading') }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('campaign.settingsHeading') }),
    ).not.toBeInTheDocument();

    await openInvite(user);
    expect(screen.getByText(CODE)).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('campaign.delete') }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: Copy link writes the share link and confirms inline', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openInvite(user);
    await user.click(screen.getByRole('button', { name: message('invite.copyLink') }));

    expect(await screen.findByRole('status')).toHaveTextContent(message('invite.copied'));
    expect(writeText).toHaveBeenCalledWith(inviteLink(window.location.origin, CODE));
  });

  it('Error: a refused clipboard write shows the fallback copy', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openInvite(user);
    await user.click(screen.getByRole('button', { name: message('invite.copyLink') }));

    expect(await screen.findByRole('alert')).toHaveTextContent(message('invite.copyFailed'));
  });

  it('Happy path: rotating asks first, then replaces the code and the link', async () => {
    const NEW = 'NEWcode-0123456789abcdef';
    const script = scripted(
      gmRoutes(gmView(), [
        {
          method: 'POST',
          path: `${DETAIL}/invite/rotate`,
          respond: () => ok(gmView({ invite_code: NEW })),
        },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openInvite(user);
    await user.click(screen.getByRole('button', { name: message('invite.rotate') }));
    expect(script.calls('POST', `${DETAIL}/invite/rotate`)).toHaveLength(0);

    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: message('invite.rotateConfirm'),
      }),
    );
    expect(await screen.findByText(NEW)).toBeInTheDocument();
    expect(script.calls('POST', `${DETAIL}/invite/rotate`)).toHaveLength(1);
  });

  it('Edge: keeping the current code on the rotate dialog sends nothing', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openInvite(user);
    await user.click(screen.getByRole('button', { name: message('invite.rotate') }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: message('invite.keepCode'),
      }),
    );

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(script.calls('POST', `${DETAIL}/invite/rotate`)).toHaveLength(0);
  });

  it('Error: a refused rotate shows its mapped copy and keeps the old code', async () => {
    const script = scripted(
      gmRoutes(gmView(), [
        {
          method: 'POST',
          path: `${DETAIL}/invite/rotate`,
          respond: () => fail('CAMPAIGN_FORBIDDEN', 403),
        },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openInvite(user);
    await user.click(screen.getByRole('button', { name: message('invite.rotate') }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: message('invite.rotateConfirm'),
      }),
    );

    expect(await screen.findByRole('alert')).toHaveTextContent(
      message('api.error.campaignForbidden'),
    );
    expect(screen.getByText(CODE)).toBeInTheDocument();
  });
});

describe('Campaign settings panel (owners only)', () => {
  it('Happy path: the settings panel carries the password and the danger zone', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openSettings(user);

    expect(settingsToggle()).toHaveAttribute('aria-expanded', 'true');
    expect(
      screen.getByRole('heading', { name: message('campaign.settingsHeading') }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { name: message('invite.passwordHeading') }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { name: message('campaign.dangerHeading') }),
    ).toBeInTheDocument();
    expect(screen.getByText(message('campaign.deleteNote'))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('campaign.delete') })).toBeInTheDocument();
  });

  it('Edge: a password under 8 characters is refused on the client and sends nothing', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openSettings(user);
    await user.type(
      screen.getByLabelText(message('invite.passwordLabel'), { exact: false }),
      'short',
    );
    await user.click(screen.getByRole('button', { name: message('invite.passwordSave') }));

    expect(await screen.findByText(message('invite.passwordTooShort'))).toBeInTheDocument();
    expect(script.calls('PUT', `${DETAIL}/password`)).toHaveLength(0);
  });

  it('Happy path: a valid password is saved and the status flips to password required', async () => {
    const script = scripted(
      gmRoutes(gmView(), [
        {
          method: 'PUT',
          path: `${DETAIL}/password`,
          respond: () => ok(gmView({ password_set: true })),
        },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openSettings(user);
    await user.type(
      screen.getByLabelText(message('invite.passwordLabel'), { exact: false }),
      'open sesame',
    );
    await user.click(screen.getByRole('button', { name: message('invite.passwordSave') }));

    expect(await screen.findByText(message('invite.passwordSet'))).toBeInTheDocument();
    expect(JSON.parse(script.calls('PUT', `${DETAIL}/password`)[0]?.request.body ?? '')).toEqual({
      password: 'open sesame',
    });
    expect(
      screen.getByRole('button', { name: message('invite.passwordClear') }),
    ).toBeInTheDocument();
  });

  it('Happy path: clearing the password calls DELETE and shows the open-link status', async () => {
    const script = scripted(
      gmRoutes(gmView({ password_set: true }), [
        {
          method: 'DELETE',
          path: `${DETAIL}/password`,
          respond: () => ok(gmView({ password_set: false })),
        },
      ]),
    );
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await openSettings(user);
    await user.click(await screen.findByRole('button', { name: message('invite.passwordClear') }));

    expect(await screen.findByText(message('invite.passwordNone'))).toBeInTheDocument();
    expect(script.calls('DELETE', `${DETAIL}/password`)).toHaveLength(1);
  });
});
