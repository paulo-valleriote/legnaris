import { SIGN_UP_PATH } from '../app/routes';

export const START_PARAM = 'start';

export const FIFTH_TEMPLATE_ID = 'fifth_character_v1';

const START_INTENTS = ['fifth', 'fantasy'] as const;

export type StartIntent = (typeof START_INTENTS)[number];

const START_TEMPLATES: Readonly<Record<StartIntent, string>> = {
  fifth: FIFTH_TEMPLATE_ID,
  fantasy: FIFTH_TEMPLATE_ID,
};

export function signUpCtaPath(intent: StartIntent): string {
  return `${SIGN_UP_PATH}?${START_PARAM}=${encodeURIComponent(intent)}`;
}

export function readStartIntent(params: URLSearchParams): StartIntent | null {
  const value = params.get(START_PARAM);
  return START_INTENTS.find((intent) => intent === value) ?? null;
}

export function startTemplateId(intent: StartIntent): string {
  return START_TEMPLATES[intent];
}
