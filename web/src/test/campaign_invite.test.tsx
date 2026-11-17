import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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

describe('GM invite controls (LGN-200 on the campaign page)', () => {
  it('Happy path: a GM sees the invite code and its share link', async () => {
    const script = scripted(gmRoutes(gmView()));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(CODE)).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: inviteLink(window.location.origin, CODE) }),
    ).toHaveAttribute('href', `${window.location.origin}/join/${CODE}`);
  });

  it('Edge: a GM with no invite code yet sees the empty copy, not a broken link', async () => {
    const script = scripted(gmRoutes(gmView({ invite_code: undefined })));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    expect(await screen.findByText(message('invite.none'))).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /\/join\// })).not.toBeInTheDocument();
  });

  it('Happy path: a player never sees the invite section', async () => {
    const script = scripted(gmRoutes(campaignView({ viewer_role: 'player', viewer_owner: false })));
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText('tomas');
    expect(
      screen.queryByRole('heading', { name: message('invite.heading') }),
    ).not.toBeInTheDocument();
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

    await screen.findByText(CODE);
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

    await screen.findByText(CODE);
    await user.click(screen.getByRole('button', { name: message('invite.rotate') }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: message('invite.keepCode'),
      }),
    );

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(script.calls('POST', `${DETAIL}/invite/rotate`)).toHaveLength(0);
  });

  it('Edge: a password under 8 characters is refused on the client and sends nothing', async () => {
    const script = scripted(gmRoutes(gmView()));
    const user = userEvent.setup();
    renderApp(DETAIL, { client: clientFor(script.transport) });

    await screen.findByText(CODE);
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

    await screen.findByText(CODE);
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

    await user.click(await screen.findByRole('button', { name: message('invite.passwordClear') }));

    expect(await screen.findByText(message('invite.passwordNone'))).toBeInTheDocument();
    expect(script.calls('DELETE', `${DETAIL}/password`)).toHaveLength(1);
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

    await screen.findByText(CODE);
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
