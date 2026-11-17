export type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';

export interface HttpRequest {
  readonly url: string;
  readonly method: HttpMethod;
  readonly headers: Readonly<Record<string, string>>;
  readonly body?: string;
  readonly credentials: 'omit' | 'include';
  readonly signal: AbortSignal;
}

export interface HttpResponse {
  readonly status: number;
  readonly body: string;
  readonly retryAfter?: string;
  readonly etag?: string;
}

export type Transport = (request: HttpRequest) => Promise<HttpResponse>;

export type FetchLike = (input: string, init: RequestInit) => Promise<Response>;

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

    const body = await response.text();
    const retryAfter = response.headers.get('retry-after');
    const etag = response.headers.get('etag');

    return {
      status: response.status,
      body,
      ...(retryAfter === null ? {} : { retryAfter }),
      ...(etag === null ? {} : { etag }),
    };
  };
}
