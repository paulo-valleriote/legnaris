import { describe, expect, it, vi } from 'vitest';
import { AD_SCRIPT_PREFIX, adScriptUrl } from '../ads/consent';
import { loadAdScript } from '../ads/loader';

const CLIENT = 'ca-pub-1234567890123456';

function detachedDocument(): Document {
  return document.implementation.createHTMLDocument('ads');
}

function adsWindow(): Window {
  return {} as unknown as Window;
}

describe('ad loader', () => {
  it('Happy path: accepted consent injects one async anonymous script', () => {
    const doc = detachedDocument();
    const win = adsWindow();

    const script = loadAdScript(doc, win, CLIENT, 'accepted');

    const scripts = doc.querySelectorAll<HTMLScriptElement>('script[data-lg-ads-loader]');
    expect(scripts).toHaveLength(1);
    expect(script).toBe(scripts[0]);
    expect(script?.async).toBe(true);
    expect(script?.crossOrigin).toBe('anonymous');
    expect(script?.src).toBe(`${AD_SCRIPT_PREFIX}${CLIENT}`);
    expect(win.adsbygoogle).toBeUndefined();
  });

  it('Edge: pending consent injects nothing and touches no queue', () => {
    const doc = detachedDocument();
    const win = adsWindow();

    expect(loadAdScript(doc, win, CLIENT, 'pending')).toBeNull();
    expect(doc.querySelectorAll('script')).toHaveLength(0);
    expect(win.adsbygoogle).toBeUndefined();
  });

  it('Happy path: declined consent asks for non-personalized ads before injecting', () => {
    const doc = detachedDocument();
    const win = adsWindow();

    loadAdScript(doc, win, CLIENT, 'declined');

    expect(win.adsbygoogle?.requestNonPersonalizedAds).toBe(1);
    expect(doc.querySelectorAll('script[data-lg-ads-loader]')).toHaveLength(1);
  });

  it('Edge: repeated calls inject exactly one script and keep the existing queue flag', () => {
    const doc = detachedDocument();
    const win = adsWindow();

    const first = loadAdScript(doc, win, CLIENT, 'declined');
    const second = loadAdScript(doc, win, CLIENT, 'declined');

    expect(second).toBe(first);
    expect(doc.querySelectorAll('script[data-lg-ads-loader]')).toHaveLength(1);
    expect(win.adsbygoogle?.requestNonPersonalizedAds).toBe(1);
  });

  it('Edge: an existing queue from the host page is reused', () => {
    const doc = detachedDocument();
    const win = { adsbygoogle: [] } as unknown as Window;

    loadAdScript(doc, win, CLIENT, 'declined');

    expect(win.adsbygoogle).toHaveLength(0);
    expect(win.adsbygoogle?.requestNonPersonalizedAds).toBe(1);
  });

  it('Error: the lg-ads policy refuses a URL outside the provider prefixes', () => {
    let rules: { createScriptURL(input: string): string } | undefined;
    const createPolicy = vi.fn(
      (_name: string, supplied: { createScriptURL(input: string): string }) => {
        rules = supplied;
        return { createScriptURL: (input: string) => supplied.createScriptURL(input) };
      },
    );
    const win = { trustedTypes: { createPolicy } } as unknown as Window;

    loadAdScript(detachedDocument(), win, CLIENT, 'accepted');

    expect(rules?.createScriptURL(`${AD_SCRIPT_PREFIX}${CLIENT}`)).toBe(
      `${AD_SCRIPT_PREFIX}${CLIENT}`,
    );
    expect(() => rules?.createScriptURL('https://ads.example.test/loader.js')).toThrow(TypeError);
    expect(() => adScriptUrl(win, 'https://ads.example.test/loader.js')).toThrow(TypeError);
  });
});
