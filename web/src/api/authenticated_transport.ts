import type { HttpRequest, Transport } from './transport';

export interface SessionDelegate {
  currentAccessToken(): string | null;
  refresh(): Promise<string | null>;
  onUnauthorized(): Promise<void>;
  sessionVersion(): number;
}

export interface AuthenticatedTransportInput {
  readonly inner: Transport;
  readonly session: SessionDelegate;
}

const HEALTH_PATH = '/health';
const AUTH_PREFIX = '/auth/';

function isPublicPath(url: string): boolean {
  let pathname: string;

  try {
    pathname = new URL(url).pathname;
  } catch {
    pathname = url;
  }

  return pathname === HEALTH_PATH || pathname.startsWith(AUTH_PREFIX);
}

function withBearer(request: HttpRequest, token: string): HttpRequest {
  return { ...request, headers: { ...request.headers, Authorization: `Bearer ${token}` } };
}

export function createAuthenticatedTransport({
  inner,
  session,
}: AuthenticatedTransportInput): Transport {
  return async (request) => {
    if (isPublicPath(request.url)) {
      return inner(request);
    }

    const token = session.currentAccessToken();
    const version = session.sessionVersion();
    const response = await inner(token === null ? request : withBearer(request, token));

    if (token === null || response.status !== 401) {
      return response;
    }

    const refreshed = await session.refresh();

    if (session.sessionVersion() !== version) {
      return response;
    }

    if (refreshed === null) {
      await session.onUnauthorized();
      return response;
    }

    return inner(withBearer(request, refreshed));
  };
}
