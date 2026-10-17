/**
 * The authenticated transport decorator (WAS-11 … WAS-17, R4).
 *
 * It wraps the F2 `Transport` and owns the Bearer posture and the silent-refresh
 * lifecycle: it attaches `Authorization: Bearer <access>` to a protected path,
 * captures the session version, and on a protected `401` performs exactly one
 * single-flight refresh (shared by concurrent refusals through the session) and
 * replays the refused request once. A refresh the server definitively rejects
 * forces the logout; a transient refresh propagates so the session survives; and
 * a response that belongs to a session that has since ended changes nothing.
 *
 * `/health` and every `/auth/*` path are public: they never carry a Bearer and
 * can never start a refresh (WAS-16).
 */

import type { HttpRequest, Transport } from './transport';

/** The session surface the decorator drives (R4). */
export interface SessionDelegate {
  /** The in-memory access token, or `null` when no session is held (WAS-11). */
  currentAccessToken(): string | null;
  /**
   * Exchanges the cookie for a fresh token; `null` on a definitive `401`, and
   * throws on a transient failure (WAS-12, WAS-14, WAS-15).
   */
  refresh(): Promise<string | null>;
  /** Forced logout: drops the token, clears the session-local data, ends the session (WAS-14). */
  onUnauthorized(): Promise<void>;
  /** The session generation, bumped only at a definitive boundary (WAS-17). */
  sessionVersion(): number;
}

/** Construction input for {@link createAuthenticatedTransport}. */
export interface AuthenticatedTransportInput {
  readonly inner: Transport;
  readonly session: SessionDelegate;
}

const HEALTH_PATH = '/health';
const AUTH_PREFIX = '/auth/';

/** True for the two route families that are never Bearer-authenticated (WAS-16). */
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

/** Builds the decorator over the inner transport and the session (R4). */
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
