/**
 * The shared request caller (API-07, API-30 … API-33, R1): the one place the
 * fail-fast timeout, the single read retry, the envelope decode and the
 * `DomainFailure`/`DefensiveParseFailure` classification live.
 *
 * Both the domain client and the auth client build on this helper, so no
 * operation writes its own attempt loop and the credential posture stays an
 * explicit per-request value rather than a hard-coded constant.
 */

import { decodeEnvelope } from './envelope';
import { classifyDomainFailure, offlineFailure } from './error_mapper';
import { DefensiveParseFailure, DomainFailure } from './errors';
import type { HttpMethod, HttpRequest, HttpResponse, Transport } from './transport';

/** Fail-fast timeout bound on every request attempt, in milliseconds (API-30). */
export const DEFAULT_REQUEST_TIMEOUT_MS = 5000;

/** Fixed delay before the single read retry, in milliseconds (API-31). */
export const DEFAULT_RETRY_DELAY_MS = 300;

/** Construction options for {@link createCaller} (API-07). */
export interface CallerOptions {
  /** Resolved absolute base address, no trailing slash (API-25, API-26). */
  readonly baseUrl: string;
  /** The single transport every request goes through (API-07). */
  readonly transport: Transport;
  /** Per-attempt fail-fast timeout; defaults to {@link DEFAULT_REQUEST_TIMEOUT_MS} (API-30). */
  readonly timeoutMs?: number;
  /** Fixed delay before the single read retry; defaults to {@link DEFAULT_RETRY_DELAY_MS} (API-31). */
  readonly retryDelayMs?: number;
  /** Injected delay for the read retry, so the interval is asserted without waiting (API-31). */
  readonly sleep?: (ms: number) => Promise<void>;
}

/**
 * Per-request overrides every operation may declare.
 *
 * `credentials` defaults to `'omit'`; only the four `/auth/*` session calls opt
 * into `'include'` so the browser attaches the refresh cookie (WAS-06). The
 * `headers` carry the CSRF echo on the state-changing auth routes (WAS-18).
 */
export interface CallOptions {
  /** Ambient credential posture; `'omit'` when absent (API-29, WAS-06). */
  readonly credentials?: 'omit' | 'include';
  /** Extra request headers, merged over the content type (WAS-18). */
  readonly headers?: Readonly<Record<string, string>>;
}

/** The two call shapes every client builds its operations on (R1). */
export interface RequestCaller {
  /**
   * Issues one request and decodes its envelope.
   *
   * A read (`GET`) repeats exactly once when the attempt fails at the transport;
   * a response — any status — ends the attempt (API-30 … API-33).
   */
  call<T>(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    decode: (data: unknown) => T,
    options?: CallOptions,
  ): Promise<T>;
  /** Issues one request whose success carries no body, such as a `204` (WAS-19). */
  callNoContent(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    options?: CallOptions,
  ): Promise<void>;
}

function defaultSleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isReceivedFailure(error: unknown): error is DomainFailure | DefensiveParseFailure {
  return error instanceof DomainFailure || error instanceof DefensiveParseFailure;
}

function classifyReceived(error: DomainFailure | DefensiveParseFailure): never {
  throw error instanceof DomainFailure ? classifyDomainFailure(error) : error;
}

/** Builds the shared caller over a resolved base address and an injected transport (R1). */
export function createCaller(options: CallerOptions): RequestCaller {
  const { baseUrl, transport } = options;
  const timeoutMs = options.timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS;
  const retryDelayMs = options.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS;
  const sleep = options.sleep ?? defaultSleep;

  async function send(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    callOptions: CallOptions | undefined,
  ): Promise<HttpResponse> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);

    const request: HttpRequest = {
      url: `${baseUrl}${path}`,
      method,
      headers: {
        ...(body === undefined ? {} : { 'content-type': 'application/json' }),
        ...(callOptions?.headers ?? {}),
      },
      ...(body === undefined ? {} : { body }),
      credentials: callOptions?.credentials ?? 'omit',
      signal: controller.signal,
    };

    try {
      return await transport(request);
    } finally {
      clearTimeout(timer);
    }
  }

  async function call<T>(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    decode: (data: unknown) => T,
    callOptions?: CallOptions,
  ): Promise<T> {
    const attempts = method === 'GET' ? 2 : 1;

    for (let index = 0; index < attempts; index += 1) {
      try {
        const response = await send(method, path, body, callOptions);
        return decodeEnvelope({
          status: response.status,
          body: response.body,
          decode,
          ...(response.retryAfter === undefined ? {} : { retryAfter: response.retryAfter }),
        });
      } catch (error) {
        if (isReceivedFailure(error)) {
          classifyReceived(error);
        }

        if (index < attempts - 1) {
          await sleep(retryDelayMs);
        }
      }
    }

    throw offlineFailure();
  }

  async function callNoContent(
    method: HttpMethod,
    path: string,
    body: string | undefined,
    callOptions?: CallOptions,
  ): Promise<void> {
    let response: HttpResponse;

    try {
      response = await send(method, path, body, callOptions);
    } catch (error) {
      if (isReceivedFailure(error)) {
        classifyReceived(error);
      }
      throw offlineFailure();
    }

    if (response.status >= 200 && response.status < 300) {
      return;
    }

    try {
      decodeEnvelope({
        status: response.status,
        body: response.body,
        decode: () => undefined,
        ...(response.retryAfter === undefined ? {} : { retryAfter: response.retryAfter }),
      });
    } catch (error) {
      if (isReceivedFailure(error)) {
        classifyReceived(error);
      }
      throw offlineFailure();
    }

    throw offlineFailure();
  }

  return { call, callNoContent };
}
