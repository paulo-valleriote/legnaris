import { describe, expect, it, vi } from 'vitest';
import { createFetchTransport } from '../api/transport';
import type { HttpRequest } from '../api/transport';

function request(overrides: Partial<HttpRequest> = {}): HttpRequest {
  return {
    url: 'https://api.example.com/templates',
    method: 'GET',
    headers: {},
    credentials: 'omit',
    signal: new AbortController().signal,
    ...overrides,
  };
}

describe('native fetch transport', () => {
  it('Happy path: passes url, method, headers, body, credentials and signal through unchanged (API-07, API-29)', async () => {
    const platformFetch = vi.fn().mockResolvedValue(new Response('{"data":1}', { status: 200 }));
    const transport = createFetchTransport(platformFetch);
    const signal = new AbortController().signal;

    await transport(
      request({
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: '{"data":{"stats":{"dexterity":16}}}',
        signal,
      }),
    );

    expect(platformFetch).toHaveBeenCalledTimes(1);
    const [url, init] = platformFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('https://api.example.com/templates');
    expect(init.method).toBe('PATCH');
    expect(init.credentials).toBe('omit');
    expect(init.signal).toBe(signal);
    expect(init.body).toBe('{"data":{"stats":{"dexterity":16}}}');
    expect(new Headers(init.headers).get('content-type')).toBe('application/json');
  });

  it('Happy path: reads the status and the raw text back (API-07)', async () => {
    const platformFetch = vi
      .fn()
      .mockResolvedValue(new Response('{"data":{"status":"healthy"}}', { status: 201 }));
    const transport = createFetchTransport(platformFetch);

    const response = await transport(request());

    expect(response.status).toBe(201);
    expect(response.body).toBe('{"data":{"status":"healthy"}}');
  });

  it('Edge: an empty body arrives as an empty string, never undefined (API-04)', async () => {
    const platformFetch = vi.fn().mockResolvedValue(new Response('', { status: 200 }));
    const transport = createFetchTransport(platformFetch);

    const response = await transport(request());

    expect(response.status).toBe(200);
    expect(response.body).toBe('');
  });

  it('Edge: omits a body key entirely when the request carries none (API-07)', async () => {
    const platformFetch = vi.fn().mockResolvedValue(new Response('{"data":1}', { status: 200 }));
    const transport = createFetchTransport(platformFetch);

    await transport(request());

    const [, init] = platformFetch.mock.calls[0] as [string, RequestInit];
    expect(init).not.toHaveProperty('body');
  });

  it('Error: a platform rejection propagates as a transport failure (API-20)', async () => {
    const platformFetch = vi.fn().mockRejectedValue(new DOMException('aborted', 'AbortError'));
    const transport = createFetchTransport(platformFetch);

    await expect(transport(request())).rejects.toThrow('aborted');
  });

  it('Error: a non-JSON response is not parsed here, so drift reaches the decoder (API-04)', async () => {
    const platformFetch = vi
      .fn()
      .mockResolvedValue(new Response('<html>502</html>', { status: 502 }));
    const transport = createFetchTransport(platformFetch);

    const response = await transport(request());

    expect(response.body).toBe('<html>502</html>');
  });
});

describe('native fetch transport Retry-After', () => {
  it('Happy path: surfaces the raw Retry-After header', async () => {
    const platformFetch = vi
      .fn()
      .mockResolvedValue(new Response('{}', { status: 409, headers: { 'Retry-After': '1' } }));

    const response = await createFetchTransport(platformFetch)(request());

    expect(response.retryAfter).toBe('1');
  });

  it('Edge: omits the key entirely when the header is absent', async () => {
    const platformFetch = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));

    const response = await createFetchTransport(platformFetch)(request());

    expect('retryAfter' in response).toBe(false);
  });
});
