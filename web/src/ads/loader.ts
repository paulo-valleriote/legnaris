import { AD_SCRIPT_PREFIX, adScriptUrl } from './consent';
import type { ConsentState } from './consent';

interface AdWindow extends Window {
  adsbygoogle?: unknown[] & { requestNonPersonalizedAds?: number };
}

export function pushAd(win: Window): void {
  const adWindow = win as AdWindow;
  const queue = adWindow.adsbygoogle ?? (adWindow.adsbygoogle = []);
  queue.push({});
}

export function loadAdScript(
  doc: Document,
  win: Window,
  client: string,
  consent: ConsentState,
): HTMLScriptElement | null {
  if (consent === 'pending') {
    return null;
  }

  const adWindow = win as AdWindow;

  if (consent === 'declined') {
    const queue = adWindow.adsbygoogle ?? (adWindow.adsbygoogle = []);
    queue.requestNonPersonalizedAds = 1;
  }

  const existing = doc.querySelector<HTMLScriptElement>('script[data-lg-ads-loader]');
  if (existing !== null) {
    return existing;
  }

  const script = doc.createElement('script');
  script.async = true;
  script.crossOrigin = 'anonymous';
  script.dataset['lgAdsLoader'] = 'true';
  script.src = adScriptUrl(win, `${AD_SCRIPT_PREFIX}${client}`);

  doc.head.appendChild(script);
  return script;
}
