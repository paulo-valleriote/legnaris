/**
 * The transport seam: the one place the client reaches the network, and the one
 * place a credential will ever be attached (API-07, API-29).
 *
 * The client depends on this narrow shape rather than on the platform request
 * function, so the security posture is an inspectable value (`credentials`) and
 * the suites can count attempts without a server.
 */

/** The HTTP methods the consumed operations use. */
export type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'DELETE';

/** A request as the client describes it, before any platform mapping. */
export interface HttpRequest {
  /** Absolute address; the base address is resolved before the transport runs. */
  readonly url: string;
  readonly method: HttpMethod;
  /** `content-type: application/json` exactly when a body is present. */
  readonly headers: Readonly<Record<string, string>>;
  /** JSON text, absent for reads. */
  readonly body?: string;
  /**
   * Credential posture. `'omit'` is the default and attaches no ambient
   * credential (API-29); `'include'` attaches the browser's cookies and is used
   * only by the four `/auth/*` session calls (WAS-06).
   */
  readonly credentials: 'omit' | 'include';
  /** Aborted when the attempt outlives the timeout, or when the caller cancels. */
  readonly signal: AbortSignal;
}

/** A response as the transport reports it, undecoded. */
export interface HttpResponse {
  readonly status: number;
  /** Raw text; an empty string means the server sent no body. */
  readonly body: string;
}

/**
 * The injected request function (API-07).
 *
 * Rejecting means the transport itself failed — network, timeout, abort — which
 * is the only condition that can trigger a read retry. Resolving means the server
 * answered, whatever the status, and ends the attempt.
 */
export type Transport = (request: HttpRequest) => Promise<HttpResponse>;

/** The subset of the platform request function the adapter needs. */
export type FetchLike = (input: string, init: RequestInit) => Promise<Response>;

/**
 * Builds the production transport over the platform request function.
 *
 * This is the only module in the client that touches a browser global, so every
 * other module stays testable without a DOM.
 */
export function createFetchTransport(fetchImpl: FetchLike): Transport {
  return async (request) => {
    const init: RequestInit = {
      method: request.method,
      headers: { ...request.headers },
      credentials: request.credentials,
      signal: request.signal,
    };

    if (request.body !== undefined) {
      init.body = request.body;
    }

    const response = await fetchImpl(request.url, init);

    return { status: response.status, body: await response.text() };
  };
}
