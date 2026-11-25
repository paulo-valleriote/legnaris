import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { UserEvent } from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { createAuthApi, type AuthApi } from '../api/auth_api';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { createAuthSession } from '../auth/auth_session';
import { message } from '../l10n/strings';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
  withPlanUsage,
} from './fake_transport';
import { renderApp } from './render';

const here = path.dirname(fileURLToPath(import.meta.url));
const BASE_URL = 'http://api.test';
const SESSION_BODY = { access_token: 'access-1', token_type: 'Bearer', expires_in: 900 };
const SHEET_ID = '01a0f967-f42b-710e-851f-8f293a634921';

function authApiWith(transport: Transport): AuthApi {
  return createAuthApi({ baseUrl: BASE_URL, transport, readCsrfToken: () => 'csrf-token' });
}

function restoringSession(transport: Transport) {
  return createAuthSession({ authApi: authApiWith(transport) });
}

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function appQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function createdFifthSheet(): Record<string, unknown> {
  return {
    id: SHEET_ID,
    template_id: 'fifth_character_v1',
    kind: 'character',
    name: message('landing.startSheetName'),
    data: {},
    dice: { section: {}, row: {} },
    created_at: '2026-11-20T10:00:00.000Z',
    updated_at: '2026-11-20T10:00:00.000Z',
  };
}

function fifthTemplate(): Record<string, unknown> {
  const capture = JSON.parse(
    readFileSync(path.resolve(here, '../../../testdata/templates/fifth_character_v1.json'), 'utf8'),
  ) as { data: Record<string, unknown> };
  return capture.data;
}

function postRequests(requests: readonly HttpRequest[]): HttpRequest[] {
  return requests.filter((request) => request.method === 'POST' && request.url.endsWith('/sheets'));
}

function deferredSheetCreation() {
  const requests: HttpRequest[] = [];
  let settle: ((response: HttpResponse) => void) | undefined;
  const transport: Transport = (request) => {
    requests.push(request);
    if (request.method === 'POST' && request.url.endsWith('/sheets')) {
      return new Promise<HttpResponse>((resolve) => {
        settle = resolve;
      });
    }
    if (request.method === 'GET' && request.url.includes('/templates/')) {
      return Promise.resolve(successOutcome(fifthTemplate()));
    }
    return Promise.resolve(successOutcome(undefined));
  };
  return {
    transport,
    requests,
    resolve(response: HttpResponse) {
      settle?.(response);
    },
  };
}

async function submitRegister(user: UserEvent) {
  await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
  await user.type(screen.getByLabelText(message('auth.usernameLabel')), 'aragorn');
  await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
  await user.click(screen.getByRole('button', { name: message('auth.submitRegister') }));
}

async function submitLogin(user: UserEvent) {
  await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
  await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
  await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));
}

async function registerAt(path: string, auth: ReturnType<typeof createDeferredTransport>) {
  const user = userEvent.setup();
  const session = restoringSession(auth.transport);
  const client = createFakeTransport(
    successOutcome(createdFifthSheet(), 201),
    successOutcome(fifthTemplate()),
  );

  renderApp(path, {
    session,
    client: clientWith(withPlanUsage(client.transport)),
    queryClient: appQueryClient(),
  });

  auth.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
  await screen.findByLabelText(message('auth.emailLabel'));
  await submitRegister(user);

  await waitFor(() => expect(auth.attempts).toBe(2));
  auth.resolve(successOutcome(SESSION_BODY));

  return { user, client };
}

describe('a sign-up started on the home page', () => {
  it('Happy path: it creates a 5E sheet through the normal API and opens it', async () => {
    const auth = createDeferredTransport();
    const { client } = await registerAt('/sign-up?start=fifth', auth);

    await waitFor(() => expect(postRequests(client.requests)).toHaveLength(1));

    const [creation] = postRequests(client.requests);
    expect(JSON.parse(creation.body ?? '{}')).toEqual({
      template_id: 'fifth_character_v1',
      kind: 'character',
      sheet_name: message('landing.startSheetName'),
    });

    expect(await screen.findByTestId('sheet-screen')).toBeInTheDocument();
    expect(await screen.findByTestId('sheet-name')).toHaveTextContent(
      message('landing.startSheetName'),
    );
    expect(screen.queryByTestId('catalog')).not.toBeInTheDocument();
  });

  it('Happy path: the fantasy alias creates the same 5E sheet and opens it', async () => {
    const auth = createDeferredTransport();
    const { client } = await registerAt('/sign-up?start=fantasy', auth);

    await waitFor(() => expect(postRequests(client.requests)).toHaveLength(1));

    const [creation] = postRequests(client.requests);
    expect(JSON.parse(creation.body ?? '{}')).toEqual({
      template_id: 'fifth_character_v1',
      kind: 'character',
      sheet_name: message('landing.startSheetName'),
    });
    expect(await screen.findByTestId('sheet-screen')).toBeInTheDocument();
  });

  it('Happy path: the start intent is stripped before the sheet request settles', async () => {
    const user = userEvent.setup();
    const auth = createDeferredTransport();
    const session = restoringSession(auth.transport);
    const creation = deferredSheetCreation();

    renderApp('/sign-up?start=fifth', {
      session,
      client: clientWith(withPlanUsage(creation.transport)),
      queryClient: appQueryClient(),
    });

    auth.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
    await screen.findByLabelText(message('auth.emailLabel'));
    await submitRegister(user);

    await waitFor(() => expect(auth.attempts).toBe(2));
    auth.resolve(successOutcome(SESSION_BODY));

    expect(await screen.findByTestId('catalog')).toBeInTheDocument();
    expect(screen.queryByTestId('start-sheet')).not.toBeInTheDocument();
    expect(postRequests(creation.requests)).toHaveLength(1);

    creation.resolve(successOutcome(createdFifthSheet(), 201));

    expect(await screen.findByTestId('sheet-screen')).toBeInTheDocument();
    expect(postRequests(creation.requests)).toHaveLength(1);
  });

  it('Happy path: the intent survives the email code step and creates the sheet after verification', async () => {
    const user = userEvent.setup();
    const auth = createDeferredTransport();
    const session = restoringSession(auth.transport);
    const client = createFakeTransport(
      successOutcome(createdFifthSheet(), 201),
      successOutcome(fifthTemplate()),
    );

    renderApp('/sign-up?start=fifth', {
      session,
      client: clientWith(withPlanUsage(client.transport)),
      queryClient: appQueryClient(),
    });

    auth.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
    await screen.findByLabelText(message('auth.emailLabel'));
    await submitRegister(user);

    await waitFor(() => expect(auth.attempts).toBe(2));
    auth.resolve(
      successOutcome({ challenge_id: 'challenge-1', expires_in: 600, resend_after: 60 }, 202),
    );

    const code = await screen.findByLabelText(message('auth.codeLabel'));
    await user.click(code);
    await user.paste('042917');
    await user.click(screen.getByRole('button', { name: message('auth.codeSubmit') }));

    await waitFor(() => expect(auth.attempts).toBe(3));
    auth.resolve(successOutcome(SESSION_BODY));

    await waitFor(() => expect(postRequests(client.requests)).toHaveLength(1));
    expect(await screen.findByTestId('sheet-screen')).toBeInTheDocument();
  });

  it('Edge: a sheet that cannot be created falls back to the catalog with no second attempt', async () => {
    const auth = createDeferredTransport();
    const session = restoringSession(auth.transport);
    const client = createFakeTransport(
      errorOutcome('SHEET_LIMIT_REACHED', 'no room', 409),
      successOutcome([]),
    );
    const user = userEvent.setup();

    renderApp('/sign-up?start=fifth', {
      session,
      client: clientWith(withPlanUsage(client.transport)),
      queryClient: appQueryClient(),
    });

    auth.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
    await screen.findByLabelText(message('auth.emailLabel'));
    await submitRegister(user);

    await waitFor(() => expect(auth.attempts).toBe(2));
    auth.resolve(successOutcome(SESSION_BODY));

    expect(await screen.findByTestId('catalog')).toBeInTheDocument();
    expect(postRequests(client.requests)).toHaveLength(1);
    expect(screen.queryByTestId('sheet-screen')).not.toBeInTheDocument();
  });

  it('Edge: an unknown intent keeps the normal catalog landing and creates no sheet', async () => {
    const auth = createDeferredTransport();
    const { client } = await registerAt('/sign-up?start=druid', auth);

    expect(await screen.findByTestId('catalog')).toBeInTheDocument();
    expect(postRequests(client.requests)).toHaveLength(0);
  });

  it('Edge: the intent on a sign-in path is ignored and creates no sheet', async () => {
    const auth = createDeferredTransport();
    const session = restoringSession(auth.transport);
    const client = createFakeTransport(successOutcome([]));
    const user = userEvent.setup();

    renderApp('/sign-in?start=fifth', {
      session,
      client: clientWith(withPlanUsage(client.transport)),
      queryClient: appQueryClient(),
    });

    auth.resolve(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
    await screen.findByLabelText(message('auth.emailLabel'));
    await submitLogin(user);

    await waitFor(() => expect(auth.attempts).toBe(2));
    auth.resolve(successOutcome(SESSION_BODY));

    expect(await screen.findByTestId('catalog')).toBeInTheDocument();
    expect(postRequests(client.requests)).toHaveLength(0);
  });
});
