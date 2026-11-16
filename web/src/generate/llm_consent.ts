import raw from './llm_consent.json';
import { drift } from '../api/parse';

export interface ConsentPoint {
  readonly heading: string;
  readonly body: string;
}

export interface ConsentDisclosure {
  readonly version: string;
  readonly title: string;
  readonly intro: string;
  readonly points: readonly ConsentPoint[];
  readonly detailsLinkText: string;
  readonly detailsUrl: string;
  readonly acceptLabel: string;
  readonly declineLabel: string;
}

interface RawLocale {
  readonly title: string;
  readonly intro: string;
  readonly points: readonly ConsentPoint[];
  readonly details_link_text: string;
  readonly details_url: string;
  readonly accept_label: string;
  readonly decline_label: string;
}

interface RawDisclosure {
  readonly version: string;
  readonly locales: { readonly en: RawLocale };
}

const source = raw as RawDisclosure;

function requireText(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.length === 0) {
    throw drift(`consent disclosure field "${field}" is empty`);
  }
  return value;
}

function decodeDisclosure(data: RawDisclosure): ConsentDisclosure {
  const locale = data.locales.en;
  return {
    version: requireText(data.version, 'version'),
    title: requireText(locale.title, 'title'),
    intro: requireText(locale.intro, 'intro'),
    points: locale.points.map((point) => ({
      heading: requireText(point.heading, 'points.heading'),
      body: requireText(point.body, 'points.body'),
    })),
    detailsLinkText: requireText(locale.details_link_text, 'details_link_text'),
    detailsUrl: requireText(locale.details_url, 'details_url'),
    acceptLabel: requireText(locale.accept_label, 'accept_label'),
    declineLabel: requireText(locale.decline_label, 'decline_label'),
  };
}

const disclosure = decodeDisclosure(source);

export const CONSENT_VERSION: string = disclosure.version;

export function consentDisclosure(): ConsentDisclosure {
  return disclosure;
}
