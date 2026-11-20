import { describe, expect, it, vi } from 'vitest';
import {
  AD_SCRIPT_PREFIX,
  CMP_SCRIPT_PREFIX,
  adScriptUrl,
  consentFromTcData,
  loadCmp,
  watchConsent,
} from '../ads/consent';

const CLIENT = 'ca-pub-1234567890123456';

function detachedDocument(): Document {
  return document.implementation.createHTMLDocument('ads');
}

function fakeDocument(defaultView: Window | null): Document {
  return {
    defaultView,
    querySelector: () => null,
    createElement: () => ({ dataset: {}, async: false, src: '' }),
    head: { appendChild: () => {} },
  } as unknown as Document;
}

describe('TCF consent mapping', () => {
  it('Happy path: gdprApplies false is accepted without a decided event', () => {
    expect(consentFromTcData({ gdprApplies: false, eventStatus: 'cmpuishown' })).toBe('accepted');
  });

  it('Happy path: a decided event with purpose 1 consent is accepted', () => {
    for (const eventStatus of ['tcloaded', 'useractioncomplete']) {
      expect(
        consentFromTcData({ gdprApplies: true, eventStatus, purpose: { consents: { '1': true } } }),
      ).toBe('accepted');
    }
  });

  it('Edge: a decided event without purpose 1 consent is declined', () => {
    for (const purpose of [undefined, {}, { consents: {} }, { consents: { '1': false } }]) {
      expect(consentFromTcData({ eventStatus: 'tcloaded', purpose })).toBe('declined');
    }
  });

  it('Edge: cmpuishown and an undecided event stay pending', () => {
    for (const tcData of [
      { eventStatus: 'cmpuishown', purpose: { consents: { '1': true } } },
      { eventStatus: 'tcloaded-unknown' },
      {},
      null,
      undefined,
      [],
      'tc',
      3,
    ]) {
      expect(consentFromTcData(tcData)).toBe('pending');
    }
  });
});

describe('consent watching', () => {
  it('Happy path: it registers a version 2 listener and removes it by listener id', () => {
    let callback: ((tcData: unknown, success: boolean) => void) | undefined;
    const api = vi.fn(
      (
        command: string,
        version: number,
        handler: (tcData: unknown, success: boolean) => void,
        parameter?: number,
      ) => {
        if (command === 'addEventListener') {
          callback = handler;
          expect(version).toBe(2);
          expect(parameter).toBeUndefined();
          handler(
            { eventStatus: 'tcloaded', purpose: { consents: { '1': true } }, listenerId: 77 },
            true,
          );
        }
      },
    );
    const win = { __tcfapi: api } as unknown as Window;
    const onChange = vi.fn();

    const stop = watchConsent(win, onChange);

    expect(onChange).toHaveBeenLastCalledWith('accepted');
    callback?.({ eventStatus: 'useractioncomplete', purpose: { consents: { '1': false } } }, true);
    expect(onChange).toHaveBeenLastCalledWith('declined');

    stop();
    expect(api).toHaveBeenCalledWith('removeEventListener', 2, expect.any(Function), 77);
  });

  it('Edge: without __tcfapi it reports nothing and its unsubscribe is safe', () => {
    const onChange = vi.fn();
    const stop = watchConsent({} as unknown as Window, onChange);

    expect(() => stop()).not.toThrow();
    expect(onChange).not.toHaveBeenCalled();
  });
});

describe('CMP loading', () => {
  it('Happy path: it injects one async funding choices script', () => {
    const doc = detachedDocument();

    const script = loadCmp(doc, CLIENT);

    const scripts = doc.querySelectorAll<HTMLScriptElement>('script[data-lg-ads-cmp]');
    expect(scripts).toHaveLength(1);
    expect(script).toBe(scripts[0]);
    expect(script?.async).toBe(true);
    expect(script?.src).toBe(`${CMP_SCRIPT_PREFIX}${CLIENT}?ers=1`);
  });

  it('Edge: a second call returns the existing script and injects nothing', () => {
    const doc = detachedDocument();

    const first = loadCmp(doc, CLIENT);
    const second = loadCmp(doc, CLIENT);

    expect(second).toBe(first);
    expect(doc.querySelectorAll('script[data-lg-ads-cmp]')).toHaveLength(1);
  });

  it('Happy path: the lg-ads policy allows both provider URLs and rejects a foreign one', () => {
    let rules: { createScriptURL(input: string): string } | undefined;
    const createPolicy = vi.fn(
      (name: string, supplied: { createScriptURL(input: string): string }) => {
        expect(name).toBe('lg-ads');
        rules = supplied;
        return { createScriptURL: (input: string) => supplied.createScriptURL(input) };
      },
    );
    const win = { trustedTypes: { createPolicy } } as unknown as Window;

    loadCmp(fakeDocument(win), CLIENT);

    expect(createPolicy).toHaveBeenCalledTimes(1);
    expect(rules?.createScriptURL(`${CMP_SCRIPT_PREFIX}${CLIENT}?ers=1`)).toBe(
      `${CMP_SCRIPT_PREFIX}${CLIENT}?ers=1`,
    );
    expect(rules?.createScriptURL(`${AD_SCRIPT_PREFIX}${CLIENT}`)).toBe(
      `${AD_SCRIPT_PREFIX}${CLIENT}`,
    );
    expect(() => rules?.createScriptURL('https://example.test/ads.js')).toThrow(TypeError);
  });

  it('Edge: without the Trusted Types API the raw URL is used', () => {
    const url = `${CMP_SCRIPT_PREFIX}${CLIENT}?ers=1`;

    expect(adScriptUrl(null, url)).toBe(url);
    expect(adScriptUrl({} as unknown as Window, url)).toBe(url);
  });
});
