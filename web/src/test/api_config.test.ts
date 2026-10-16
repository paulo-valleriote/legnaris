import { describe, expect, it } from 'vitest';
import { DEFAULT_API_BASE_URL, resolveApiConfig } from '../api/config';
import { ConfigurationFailure } from '../api/errors';

describe('api base address configuration', () => {
  it('Happy path: with no override the client targets the public API over HTTPS (API-25)', () => {
    const config = resolveApiConfig(undefined);

    expect(config.baseUrl).toBe(DEFAULT_API_BASE_URL);
    expect(config.baseUrl.startsWith('https://')).toBe(true);
    expect(config.source).toBe('default');
    expect(config.secure).toBe(true);
    expect(config.isDevOverride).toBe(false);
  });

  it('Happy path: a supplied override is used exactly (API-26)', () => {
    const config = resolveApiConfig('https://staging.example.test');

    expect(config.baseUrl).toBe('https://staging.example.test');
    expect(config.source).toBe('override');
    expect(config.secure).toBe(true);
    expect(config.isDevOverride).toBe(true);
  });

  it('Edge: an empty override falls back to the secure default (API-26, API-27)', () => {
    for (const raw of ['', '   ']) {
      const config = resolveApiConfig(raw);

      expect(config.baseUrl).toBe(DEFAULT_API_BASE_URL);
      expect(config.source).toBe('default');
      expect(config.secure).toBe(true);
      expect(config.isDevOverride).toBe(false);
    }
  });

  it('Edge: a trailing slash is normalized away so the base address joins cleanly (API-26)', () => {
    expect(resolveApiConfig('https://staging.example.test/').baseUrl).toBe(
      'https://staging.example.test',
    );
  });

  it('Happy path: a plaintext override reports the connection as not secure (API-28)', () => {
    const config = resolveApiConfig('http://localhost:8080');

    expect(config.baseUrl).toBe('http://localhost:8080');
    expect(config.secure).toBe(false);
    expect(config.isDevOverride).toBe(true);
    expect(config.source).toBe('override');
  });

  it('Edge: an uppercase scheme is still recognised as secure (API-28)', () => {
    expect(resolveApiConfig('HTTPS://staging.example.test').secure).toBe(true);
  });

  it('Error: a bare host is refused before any request is issued (API-27)', () => {
    expect(() => resolveApiConfig('api.example.test')).toThrow(ConfigurationFailure);
  });

  it('Error: a relative path is refused before any request is issued (API-27)', () => {
    expect(() => resolveApiConfig('/api')).toThrow(ConfigurationFailure);
  });

  it('Error: a non-HTTP scheme is refused before any request is issued (API-27)', () => {
    for (const raw of ['ftp://example.test', 'ws://example.test', 'file:///tmp/api']) {
      expect(() => resolveApiConfig(raw)).toThrow(ConfigurationFailure);
    }
  });

  it('Error: the resolver rejects rather than coerces a malformed value (API-27)', () => {
    const thrown = (() => {
      try {
        resolveApiConfig('api.example.test');
        return undefined;
      } catch (error) {
        return error;
      }
    })();

    expect(thrown).toBeInstanceOf(ConfigurationFailure);
    expect((thrown as ConfigurationFailure).name).toBe('ConfigurationFailure');
  });
});
