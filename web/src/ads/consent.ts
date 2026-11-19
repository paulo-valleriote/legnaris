export type ConsentState = 'pending' | 'accepted' | 'declined';

export const CMP_SCRIPT_PREFIX = 'https://fundingchoicesmessages.google.com/i/';

export const AD_SCRIPT_PREFIX =
  'https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=';

const ALLOWED_SCRIPT_URL_PREFIXES: readonly string[] = [CMP_SCRIPT_PREFIX, AD_SCRIPT_PREFIX];

interface ScriptUrlPolicy {
  createScriptURL(input: string): string;
}

const policies = new WeakMap<object, ScriptUrlPolicy>();

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function policyFor(win: Window): ScriptUrlPolicy | undefined {
  const trustedTypes = win.trustedTypes;
  if (trustedTypes === undefined) {
    return undefined;
  }

  const cached = policies.get(trustedTypes);
  if (cached !== undefined) {
    return cached;
  }

  const policy = trustedTypes.createPolicy('lg-ads', {
    createScriptURL: (input: string) => {
      if (ALLOWED_SCRIPT_URL_PREFIXES.some((prefix) => input.startsWith(prefix))) {
        return input;
      }
      throw new TypeError(`ads script URL is not allowed: ${input}`);
    },
  }) as ScriptUrlPolicy;

  policies.set(trustedTypes, policy);
  return policy;
}

export function adScriptUrl(win: Window | null | undefined, url: string): string {
  if (win === null || win === undefined) {
    return url;
  }
  const policy = policyFor(win);
  return policy === undefined ? url : policy.createScriptURL(url);
}

export function consentFromTcData(tcData: unknown): ConsentState {
  if (!isRecord(tcData)) {
    return 'pending';
  }

  if (tcData['gdprApplies'] === false) {
    return 'accepted';
  }

  const status = tcData['eventStatus'];
  if (status !== 'tcloaded' && status !== 'useractioncomplete') {
    return 'pending';
  }

  const purpose = tcData['purpose'];
  const consents = isRecord(purpose) ? purpose['consents'] : undefined;
  const granted = isRecord(consents) ? consents['1'] === true : false;

  return granted ? 'accepted' : 'declined';
}

export function watchConsent(win: Window, onChange: (state: ConsentState) => void): () => void {
  const api = win.__tcfapi;
  if (typeof api !== 'function') {
    return () => {};
  }

  let listenerId: number | undefined;
  api('addEventListener', 2, (tcData: unknown) => {
    if (isRecord(tcData) && typeof tcData['listenerId'] === 'number') {
      listenerId = tcData['listenerId'];
    }
    onChange(consentFromTcData(tcData));
  });

  return () => {
    if (listenerId !== undefined) {
      api('removeEventListener', 2, () => {}, listenerId);
    }
  };
}

export function loadCmp(doc: Document, client: string): HTMLScriptElement | null {
  const existing = doc.querySelector<HTMLScriptElement>('script[data-lg-ads-cmp]');
  if (existing !== null) {
    return existing;
  }

  const script = doc.createElement('script');
  script.async = true;
  script.dataset['lgAdsCmp'] = 'true';
  script.src = adScriptUrl(doc.defaultView, `${CMP_SCRIPT_PREFIX}${client}?ers=1`);

  doc.head.appendChild(script);
  return script;
}
