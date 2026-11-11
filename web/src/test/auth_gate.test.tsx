import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createAuthApi } from '../api/auth_api';
import { createApiClient } from '../api/client';
import type { Sheet } from '../api/models/sheet';
import type { Transport } from '../api/transport';
import { createAuthSession } from '../auth/auth_session';
import { clearSessionData } from '../library/session_data';
import { createMemorySheetStore } from '../library/sheet_store';
import { message } from '../l10n/strings';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

const SESSION_BODY = { access_token: 'access-1', token_type: 'Bearer', expires_in: 900 };

const CHALLENGE_BODY = { challenge_id: 'challenge-1', expires_in: 600, resend_after: 60 };

const SHEET: Sheet = {
  id: '01a0f967-f42b-710e-851f-8f293a634921',
  templateId: 'fantasy_v1',
  name: 'Aragorn',
  data: {},
  createdAt: '2026-10-01T21:38:54.635Z',
  updatedAt: '2026-10-02T09:00:00.000Z',
};

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

/** A session restoring over the injected auth transport, the way the gate mounts one. */
function restoringSession(
  transport: Transport,
  clear: () => Promise<void> = () => Promise.resolve(),
) {
  return createAuthSession({
    authApi: createAuthApi({ baseUrl: BASE_URL, transport, readCsrfToken: () => 'csrf-token' }),
    clearSessionData: clear,
  });
}

describe('auth gate (WAS-03, WAS-04, WAS-05, WAS-08, WAS-24)', () => {
  it('Edge: a rejected cookie shows the sign-in form with no session (WAS-04)', async () => {
    const fake = createDeferredTransport();
    const session = restoringSession(fake.transport);

    renderApp('/sign-in', { session });

    fake.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );
    expect(session.getSnapshot().state).toBe('unauthenticated');
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
  });

  it('Edge: a transient restore failure shows the sign-in form and keeps the session-local data (WAS-05)', async () => {
    const fake = createDeferredTransport();
    const store = createMemorySheetStore();
    await store.putSheet(SHEET);
    const session = restoringSession(fake.transport, () => clearSessionData(store));

    renderApp('/sign-in', { session, sheetStore: store });

    fake.resolve({
      status: 500,
      body: JSON.stringify({ error: { code: 'INTERNAL_ERROR', message: 'boom' } }),
    });

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );
    expect(session.getSnapshot().state).toBe('unauthenticated');
    expect(store.records()).toHaveLength(1);
  });

  it('Edge: no session renders neither the catalog, the sheet nor the own-sheet surface (WAS-08)', async () => {
    const fake = createDeferredTransport();
    const session = restoringSession(fake.transport);

    renderApp('/sign-in', { session });
    fake.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-screen')).not.toBeInTheDocument();
    expect(screen.queryByTestId('your-sheet-screen')).not.toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Primary' })).not.toBeInTheDocument();
  });

  it('Edge: a deep link while signed out carries the destination and after sign-in returns to the sheet (WAS-24)', async () => {
    const user = userEvent.setup();
    const sessionTransport = createDeferredTransport();
    const session = restoringSession(sessionTransport.transport);
    const catalog = createDeferredTransport();

    renderApp('/sheets/abc', { session, client: clientWith(catalog.transport) });

    sessionTransport.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );
    expect(screen.queryByTestId('sheet-screen')).not.toBeInTheDocument();

    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));
    sessionTransport.resolve(successOutcome(SESSION_BODY));
    catalog.resolve(successOutcome([]));

    await waitFor(() => expect(screen.getByTestId('sheet-screen')).toBeInTheDocument());
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
  });

  it('Edge: a hostile next parameter is ignored and sign-in lands on the catalog', async () => {
    const user = userEvent.setup();
    const sessionTransport = createDeferredTransport();
    const session = restoringSession(sessionTransport.transport);
    const catalog = createDeferredTransport();

    renderApp('/sign-in?next=%2F%2Fevil.com', { session, client: clientWith(catalog.transport) });
    sessionTransport.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );
    expect(screen.getByText(message('auth.subtitleLogin'))).toBeInTheDocument();

    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));
    sessionTransport.resolve(successOutcome(SESSION_BODY));
    catalog.resolve(successOutcome([]));

    await waitFor(() => expect(screen.getByTestId('catalog')).toBeInTheDocument());
  });

  it('Happy path: a sheet destination changes the sign-in subtitle', async () => {
    const sessionTransport = createDeferredTransport();
    const session = restoringSession(sessionTransport.transport);

    renderApp('/sheets/abc', { session });
    sessionTransport.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));

    await waitFor(() =>
      expect(screen.getByText(message('auth.subtitleLoginSheet'))).toBeInTheDocument(),
    );
  });

  it('Edge: a session answer never shows the code surface (WCS-01)', async () => {
    const user = userEvent.setup();
    const sessionTransport = createDeferredTransport();
    const catalog = createDeferredTransport();
    const session = restoringSession(sessionTransport.transport);

    renderApp('/sign-in', { session, client: clientWith(catalog.transport) });

    sessionTransport.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );

    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));
    sessionTransport.resolve(successOutcome(SESSION_BODY));
    catalog.resolve(successOutcome([]));

    await waitFor(() => expect(screen.getByTestId('catalog')).toBeInTheDocument());
    expect(screen.queryByLabelText(message('auth.codeLabel'))).not.toBeInTheDocument();
  });

  it('Happy path: a valid cookie restores through exactly one silent refresh, showing restoring until it settles (WAS-03)', async () => {
    const deferred = createDeferredTransport();
    const session = restoringSession(deferred.transport);

    renderApp('/sign-in', { session, client: clientWith(createDeferredTransport().transport) });

    expect(screen.getByTestId('auth-restoring')).toHaveTextContent(message('auth.restoring'));

    deferred.resolve(successOutcome(SESSION_BODY));

    await waitFor(() => expect(screen.getByTestId('catalog')).toBeInTheDocument());
    expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument();
    expect(
      deferred.requests.filter((request) => request.url.endsWith('/auth/refresh')),
    ).toHaveLength(1);
  });

  it('Happy path: a code-step answer renders the code surface in place of the credential form and holds no session (WCS-01, WCS-02)', async () => {
    const user = userEvent.setup();
    const sessionTransport = createDeferredTransport();
    const session = restoringSession(sessionTransport.transport);

    renderApp('/sign-in', { session });

    sessionTransport.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );

    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));
    sessionTransport.resolve(successOutcome(CHALLENGE_BODY, 202));

    await waitFor(() =>
      expect(screen.getByLabelText(message('auth.codeLabel'))).toBeInTheDocument(),
    );
    expect(screen.queryByLabelText(message('auth.emailLabel'))).not.toBeInTheDocument();
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
    expect(session.getSnapshot().state).toBe('pendingCode');
    expect(session.getSnapshot().accessToken).toBeNull();
    expect(session.currentAccessToken()).toBeNull();
  });

  it('Happy path: abandoning the step returns the form with the address and the mode, an empty password and the focus (WCS-21, WCS-22)', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY, 202));
    const session = createAuthSession({
      authApi: createAuthApi({
        baseUrl: BASE_URL,
        transport: fake.transport,
        readCsrfToken: () => 'csrf-token',
      }),
    });

    renderApp('/sign-in', { session });

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );

    await user.click(screen.getByRole('tab', { name: message('auth.modeRegister') }));
    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.usernameLabel')), 'aragorn');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitRegister') }));

    await waitFor(() =>
      expect(screen.getByLabelText(message('auth.codeLabel'))).toBeInTheDocument(),
    );

    await user.click(screen.getByTestId('code-back'));

    await waitFor(() =>
      expect(screen.getByLabelText(message('auth.emailLabel'))).toBeInTheDocument(),
    );
    expect(screen.getByLabelText(message('auth.emailLabel'))).toHaveValue('a@b.test');
    expect(screen.getByLabelText(message('auth.usernameLabel'))).toHaveValue('aragorn');
    expect(screen.getByLabelText(message('auth.passwordLabel'))).toHaveValue('');
    expect(screen.getByRole('tab', { name: message('auth.modeRegister') })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByLabelText(message('auth.emailLabel'))).toHaveFocus();
  });
});
