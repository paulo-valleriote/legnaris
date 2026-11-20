import { describe, expect, it, vi } from 'vitest';
import { readAdsConfig } from '../ads/ads_config';
import { ConfigurationFailure } from '../api/errors';
import { loadRuntimeConfiguration } from '../api/runtime_config';

const CLIENT = 'ca-pub-1234567890123456';

function response(body: unknown, init?: ResponseInit): Response {
  return new Response(typeof body === 'string' ? body : JSON.stringify(body), init);
}

describe('ads runtime configuration', () => {
  it('Happy path: a valid document yields the client and every valid slot', () => {
    const config = readAdsConfig({
      adsense: {
        client: CLIENT,
        slots: { catalog: '1111111111', campaigns: '2222222222' },
      },
    });

    expect(config).toEqual({
      client: CLIENT,
      slots: { catalog: '1111111111', campaigns: '2222222222' },
    });
  });

  it('Edge: the removed landing slot is ignored', () => {
    const config = readAdsConfig({
      adsense: { client: CLIENT, slots: { catalog: '1111111111', landing: '3333333333' } },
    });

    expect(config).toEqual({ client: CLIENT, slots: { catalog: '1111111111' } });
  });

  it('Edge: a missing slots object is a valid client with no slot', () => {
    expect(readAdsConfig({ adsense: { client: CLIENT } })).toEqual({ client: CLIENT, slots: {} });
  });

  it('Edge: an invalid slot is omitted and the valid slots stay', () => {
    const config = readAdsConfig({
      adsense: {
        client: CLIENT,
        slots: { catalog: '123', campaigns: 'not-digits' },
      },
    });

    expect(config).toEqual({ client: CLIENT, slots: { catalog: '123' } });
  });

  it('Edge: unknown slot names are ignored', () => {
    const config = readAdsConfig({
      adsense: { client: CLIENT, slots: { catalog: '1', sidebar: '2', landing: '3' } },
    });

    expect(config).toEqual({ client: CLIENT, slots: { catalog: '1' } });
  });

  it('Edge: a document without adsense, or a non-object document, reads as absent', () => {
    for (const document of [undefined, null, {}, [], 'ads', 42]) {
      expect(readAdsConfig(document)).toBeNull();
    }
  });

  it('Edge: an adsense value that is not an object reads as absent', () => {
    for (const adsense of [null, 'ca-pub-1234567890', [], 7]) {
      expect(readAdsConfig({ adsense })).toBeNull();
    }
  });

  it('Edge: a missing client reads as absent', () => {
    expect(readAdsConfig({ adsense: { slots: { catalog: '1' } } })).toBeNull();
  });

  it('Edge: a client outside the ca-pub digit range reads as absent', () => {
    for (const client of [
      'ca-pub-123456789',
      'ca-pub-123456789012345678901',
      'pub-1234567890',
      42,
    ]) {
      expect(readAdsConfig({ adsense: { client } })).toBeNull();
    }
  });

  it('Edge: a slots value that is not an object reads as absent', () => {
    for (const slots of [null, [], 'slots', 3]) {
      expect(readAdsConfig({ adsense: { client: CLIENT, slots } })).toBeNull();
    }
  });

  it('Happy path: the runtime document carries both the API address and the ads config', async () => {
    const fetcher = vi.fn().mockResolvedValue(
      response({
        apiBaseUrl: 'https://api.example.test',
        adsense: { client: CLIENT, slots: { catalog: '99' } },
      }),
    );

    const result = await loadRuntimeConfiguration({ fetcher, buildValue: undefined });

    expect(result.configuration.baseUrl).toBe('https://api.example.test');
    expect(result.ads).toEqual({ client: CLIENT, slots: { catalog: '99' } });
  });

  it('Edge: a malformed ads block leaves the API address intact and the ads config absent', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(response({ apiBaseUrl: 'https://api.example.test', adsense: 'bad' }));

    const result = await loadRuntimeConfiguration({ fetcher, buildValue: undefined });

    expect(result.configuration.baseUrl).toBe('https://api.example.test');
    expect(result.ads).toBeNull();
  });

  it('Happy path: a build-time API address still reads the ads config from config.json', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(response({ adsense: { client: CLIENT, slots: { campaigns: '7' } } }));

    const result = await loadRuntimeConfiguration({
      fetcher,
      buildValue: 'http://localhost:8080',
    });

    expect(result.configuration.baseUrl).toBe('http://localhost:8080');
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(result.ads).toEqual({ client: CLIENT, slots: { campaigns: '7' } });
  });

  it('Edge: a failed config.json read with a build-time API address yields no ads and no failure', async () => {
    const fetcher = vi.fn().mockRejectedValue(new TypeError('offline'));

    const result = await loadRuntimeConfiguration({
      fetcher,
      buildValue: 'http://localhost:8080',
    });

    expect(result.configuration.baseUrl).toBe('http://localhost:8080');
    expect(result.ads).toBeNull();
  });

  it('Edge: an unreadable config.json with a build-time API address is swallowed', async () => {
    const fetcher = vi.fn().mockResolvedValue(response('{oops'));

    const result = await loadRuntimeConfiguration({
      fetcher,
      buildValue: 'http://localhost:8080',
    });

    expect(result.ads).toBeNull();
  });

  it('Error: without a build value a malformed API address still fails startup', async () => {
    const fetcher = vi.fn().mockResolvedValue(response({ apiBaseUrl: 'not a url' }));

    await expect(
      loadRuntimeConfiguration({ fetcher, buildValue: undefined }),
    ).rejects.toBeInstanceOf(ConfigurationFailure);
  });
});
