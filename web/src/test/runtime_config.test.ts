import { describe, expect, it, vi } from 'vitest';
import { DEFAULT_API_BASE_URL } from '../api/config';
import { ConfigurationFailure } from '../api/errors';
import { RUNTIME_CONFIG_PATH, loadApiConfiguration } from '../api/runtime_config';

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(typeof body === 'string' ? body : JSON.stringify(body), init);
}

describe('runtime api configuration', () => {
  it('Happy path: the address in config.json is used and fetched uncached', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(jsonResponse({ apiBaseUrl: 'https://k3d.example.test/' }));

    const config = await loadApiConfiguration({ fetcher, buildValue: undefined });

    expect(config.baseUrl).toBe('https://k3d.example.test');
    expect(config.source).toBe('override');
    expect(fetcher).toHaveBeenCalledWith(RUNTIME_CONFIG_PATH, { cache: 'no-store' });
  });

  it('Happy path: a build-time value wins and config.json is never requested', async () => {
    const fetcher = vi.fn();

    const config = await loadApiConfiguration({ fetcher, buildValue: 'http://localhost:8080' });

    expect(config.baseUrl).toBe('http://localhost:8080');
    expect(fetcher).not.toHaveBeenCalled();
  });

  it('Edge: a blank build-time value does not suppress the runtime lookup', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(jsonResponse({ apiBaseUrl: 'https://a.example.test' }));

    const config = await loadApiConfiguration({ fetcher, buildValue: '  ' });

    expect(config.baseUrl).toBe('https://a.example.test');
  });

  it.each([
    ['an empty object', {}],
    ['an empty string', { apiBaseUrl: '' }],
    ['a whitespace string', { apiBaseUrl: '   ' }],
  ])('Edge: %s falls back to the secure default', async (_name, body) => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse(body));

    const config = await loadApiConfiguration({ fetcher, buildValue: undefined });

    expect(config.baseUrl).toBe(DEFAULT_API_BASE_URL);
    expect(config.source).toBe('default');
  });

  it.each([
    ['a non-URL value', { apiBaseUrl: 'not a url' }],
    ['a non-http scheme', { apiBaseUrl: 'ftp://example.test' }],
    ['a non-string value', { apiBaseUrl: 42 }],
    ['a null document', null],
    ['an array document', []],
  ])('Error: %s is refused with a configuration failure', async (_name, body) => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse(body));

    await expect(loadApiConfiguration({ fetcher, buildValue: undefined })).rejects.toBeInstanceOf(
      ConfigurationFailure,
    );
  });

  it('Error: malformed JSON is refused with a configuration failure', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse('{oops'));

    await expect(loadApiConfiguration({ fetcher, buildValue: undefined })).rejects.toBeInstanceOf(
      ConfigurationFailure,
    );
  });

  it('Error: a non-2xx response names the status in the failure', async () => {
    const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', { status: 404 }));

    await expect(loadApiConfiguration({ fetcher, buildValue: undefined })).rejects.toThrow(/404/);
  });

  it('Error: a network failure is wrapped in a configuration failure', async () => {
    const fetcher = vi.fn().mockRejectedValue(new TypeError('offline'));

    await expect(loadApiConfiguration({ fetcher, buildValue: undefined })).rejects.toBeInstanceOf(
      ConfigurationFailure,
    );
  });
});
