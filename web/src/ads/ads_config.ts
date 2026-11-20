export const AD_SURFACES = ['catalog', 'campaigns'] as const;

export type AdSurface = (typeof AD_SURFACES)[number];

export interface AdsConfig {
  readonly client: string;
  readonly slots: Readonly<Partial<Record<AdSurface, string>>>;
}

const CLIENT_PATTERN = /^ca-pub-\d{10,20}$/;

const SLOT_PATTERN = /^\d{1,20}$/;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function readAdsConfig(document: unknown): AdsConfig | null {
  if (!isRecord(document)) {
    return null;
  }

  const raw = document['adsense'];
  if (raw === undefined) {
    return null;
  }
  if (!isRecord(raw)) {
    return null;
  }

  const client = raw['client'];
  if (typeof client !== 'string' || !CLIENT_PATTERN.test(client)) {
    return null;
  }

  const rawSlots = raw['slots'];
  if (rawSlots === undefined) {
    return { client, slots: {} };
  }
  if (!isRecord(rawSlots)) {
    return null;
  }

  const slots: Partial<Record<AdSurface, string>> = {};
  for (const surface of AD_SURFACES) {
    const value = rawSlots[surface];
    if (typeof value === 'string' && SLOT_PATTERN.test(value)) {
      slots[surface] = value;
    }
  }

  return { client, slots };
}
