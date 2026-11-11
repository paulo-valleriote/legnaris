import { describe, expect, it, vi } from 'vitest';
import { createAuthApi } from '../api/auth_api';
import type { SessionDelegate } from '../api/authenticated_transport';
import { createAuthenticatedTransport } from '../api/authenticated_transport';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { createAuthSession } from '../auth/auth_session';
import { createDeferredTransport, errorOutcome, successOutcome } from './fake_transport';

const BASE_URL = 'http://api.test';

const SESSION_BODY = { access_token: 'access-2', token_type: 'Bearer', expires_in: 900 };

function request(url: string): HttpRequest {
  return {
    url,
    method: 'GET',
    headers: {},
    credentials: 'omit',
    signal: new AbortController().signal,
  };
}

interface SessionHarness {
  readonly session: SessionDelegate;
  readonly refresh: ReturnType<typeof vi.fn>;
  readonly onUnauthorized: ReturnType<typeof vi.fn>;
  setVersion(version: number): void;
}

function sessionHarness(
  options: {
    readonly token?: string | null;
    readonly refresh?: () => Promise<string | null>;
  } = {},
): SessionHarness {
  const token = options.token === undefined ? 'access-1' : options.token;
  let version = 0;
  const refresh = vi.fn(options.refresh ?? (async () => 'access-2'));
  const onUnauthorized = vi.fn(async () => {});

  return {
    session: {
      currentAccessToken: () => token,
      refresh,
      onUnauthorized,
      sessionVersion: () => version,
    },
    refresh,
    onUnauthorized,
    setVersion: (next) => {
      version = next;
    },
  };
}

function recordingInner(outcomes: HttpResponse[]): {
  transport: Transport;
  requests: HttpRequest[];
} {
  const requests: HttpRequest[] = [];
  return {
    transport: (incoming) => {
      requests.push(incoming);
      const outcome = outcomes[Math.min(requests.length - 1, outcomes.length - 1)];
      return Promise.resolve(outcome);
    },
    requests,
  };
}

const UNAUTHORIZED: HttpResponse = errorOutcome('UNAUTHORIZED', 'expired', 401);
const OK: HttpResponse = successOutcome({ ok: true });

describe('authenticated transport (WAS-11 … WAS-17)', () => {
  it('Happy path: a protected call carries the Bearer token and passes a non-401 response through (WAS-11)', async () => {
    const inner = recordingInner([OK]);
    const { session, refresh } = sessionHarness();
    const transport = createAuthenticatedTransport({ inner: inner.transport, session });

    const response = await transport(request(`${BASE_URL}/sheets/1`));

    expect(response.status).toBe(200);
    expect(inner.requests).toHaveLength(1);
    expect(inner.requests[0].headers['Authorization']).toBe('Bearer access-1');
    expect(refresh).not.toHaveBeenCalled();
  });

  it('Edge: a protected call with no token sends no Bearer and starts no refresh (WAS-11)', async () => {
    const inner = recordingInner([UNAUTHORIZED]);
    const { session, refresh, onUnauthorized } = sessionHarness({ token: null });
    const transport = createAuthenticatedTransport({ inner: inner.transport, session });

    const response = await transport(request(`${BASE_URL}/sheets/1`));

    expect(response.status).toBe(401);
    expect(inner.requests[0].headers['Authorization']).toBeUndefined();
    expect(refresh).not.toHaveBeenCalled();
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it('Edge: /health and every /auth/* path carry no Bearer and can never start a refresh (WAS-16)', async () => {
    const paths = ['/health', '/auth/register', '/auth/login', '/auth/refresh', '/auth/logout'];

    for (const path of paths) {
      const inner = recordingInner([UNAUTHORIZED]);
      const { session, refresh } = sessionHarness();
      const transport = createAuthenticatedTransport({ inner: inner.transport, session });

      const response = await transport(request(`${BASE_URL}${path}`));

      expect(response.status).toBe(401);
      expect(inner.requests[0].headers['Authorization']).toBeUndefined();
      expect(refresh).not.toHaveBeenCalled();
    }
  });

  it('Happy path: a 401 produces exactly one refresh and one replayed request with the new Bearer (WAS-12, WAS-13)', async () => {
    const inner = recordingInner([UNAUTHORIZED, OK]);
    const { session, refresh } = sessionHarness();
    const transport = createAuthenticatedTransport({ inner: inner.transport, session });

    const response = await transport(request(`${BASE_URL}/sheets/1`));

    expect(response.status).toBe(200);
    expect(inner.requests).toHaveLength(2);
    expect(inner.requests[1].headers['Authorization']).toBe('Bearer access-2');
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('Edge: a replay that is again 401 is not replayed a second time (WAS-12)', async () => {
    const inner = recordingInner([UNAUTHORIZED, UNAUTHORIZED]);
    const { session, refresh, onUnauthorized } = sessionHarness();
    const transport = createAuthenticatedTransport({ inner: inner.transport, session });

    const response = await transport(request(`${BASE_URL}/sheets/1`));

    expect(response.status).toBe(401);
    expect(inner.requests).toHaveLength(2);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it('Happy path: a refresh rejected with 401 forces the logout once and returns the original 401 (WAS-14)', async () => {
    const inner = recordingInner([UNAUTHORIZED]);
    const { session, onUnauthorized } = sessionHarness({ refresh: async () => null });
    const transport = createAuthenticatedTransport({ inner: inner.transport, session });

    const response = await transport(request(`${BASE_URL}/sheets/1`));

    expect(response.status).toBe(401);
    expect(inner.requests).toHaveLength(1);
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it('Edge: a transient refresh rethrows and leaves the session untouched (WAS-15)', async () => {
    const inner = recordingInner([UNAUTHORIZED]);
    const failure = new Error('network');
    const { session, onUnauthorized } = sessionHarness({
      refresh: async () => Promise.reject(failure),
    });
    const transport = createAuthenticatedTransport({ inner: inner.transport, session });

    await expect(transport(request(`${BASE_URL}/sheets/1`))).rejects.toBe(failure);

    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it('Edge: a 401 belonging to a superseded session is dropped — no replay, no forced logout (WAS-17)', async () => {
    const inner = recordingInner([UNAUTHORIZED, OK]);
    const harness = sessionHarness({
      refresh: async () => {
        harness.setVersion(1);
        return 'access-2';
      },
    });
    const transport = createAuthenticatedTransport({
      inner: inner.transport,
      session: harness.session,
    });

    const response = await transport(request(`${BASE_URL}/sheets/1`));

    expect(response.status).toBe(401);
    expect(inner.requests).toHaveLength(1);
    expect(harness.onUnauthorized).not.toHaveBeenCalled();
  });

  it('Happy path: two concurrent 401s share a single in-flight refresh (WAS-13)', async () => {
    const authFake = createDeferredTransport();
    const session = createAuthSession({
      authApi: createAuthApi({
        baseUrl: BASE_URL,
        transport: authFake.transport,
        readCsrfToken: () => 'csrf-token',
      }),
      initialState: 'authenticated',
      initialAccessToken: 'access-1',
    });
    const innerFake = createDeferredTransport();
    const transport = createAuthenticatedTransport({ inner: innerFake.transport, session });

    const first = transport(request(`${BASE_URL}/sheets/1`));
    const second = transport(request(`${BASE_URL}/sheets/2`));
    await vi.waitFor(() => expect(innerFake.attempts).toBe(2));

    innerFake.resolve(UNAUTHORIZED);
    innerFake.resolve(UNAUTHORIZED);

    await vi.waitFor(() => expect(authFake.attempts).toBe(1));

    authFake.resolve(successOutcome(SESSION_BODY));

    await vi.waitFor(() => expect(innerFake.attempts).toBe(4));
    expect(innerFake.requests[2].headers['Authorization']).toBe('Bearer access-2');
    expect(innerFake.requests[3].headers['Authorization']).toBe('Bearer access-2');

    innerFake.resolve(OK);
    innerFake.resolve(OK);

    await expect(first).resolves.toMatchObject({ status: 200 });
    await expect(second).resolves.toMatchObject({ status: 200 });
  });
});
