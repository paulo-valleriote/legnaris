import type { AcceptedGeneration, Generation } from './models/generation';
import {
  parseAcceptedGeneration,
  parseCreatedGenerationId,
  parseGeneration,
} from './models/generation';
import { BILLING_PATHS, parseRedirectUrl } from './models/billing';
import type { JoinedCampaign } from './models/campaign';
import { parseJoinedCampaign } from './models/campaign';
import type { DiceBlock } from './models/dice';
import { diceBlockToWire } from './models/dice';
import type { Layout } from './models/layout';
import { layoutToWire, parseLayoutResponse } from './models/layout';
import type { PlanUsage, SheetKind } from './models/plan';
import { parsePlanUsage } from './models/plan';
import type { Sheet } from './models/sheet';
import { parseSheet } from './models/sheet';
import type { TemplateDetail, TemplateSummary } from './models/template';
import { parseTemplateCatalog, parseTemplateDetail } from './models/template';
import { drift, hasEntries, isPlainObject } from './parse';
import type { CallerOptions } from './request';
import { createCaller, DEFAULT_REQUEST_TIMEOUT_MS, DEFAULT_RETRY_DELAY_MS } from './request';

export { DEFAULT_REQUEST_TIMEOUT_MS, DEFAULT_RETRY_DELAY_MS };

export interface HealthPayload {
  readonly status: string;
}

export interface CreateSheetInput {
  readonly templateId: string;
  readonly kind: SheetKind;
  readonly sheetName: string;
}

export interface JoinCampaignInput {
  readonly code: string;
  readonly password?: string | undefined;
}

export type ApiClientOptions = CallerOptions;

export interface ApiClient {
  health(): Promise<HealthPayload>;
  listTemplates(): Promise<TemplateSummary[]>;
  getTemplate(id: string): Promise<TemplateDetail>;
  createSheet(input: CreateSheetInput): Promise<Sheet>;
  getMyPlan(): Promise<PlanUsage>;
  createCheckout(): Promise<string>;
  openPortal(): Promise<string>;
  requestRefund(): Promise<void>;
  getSheet(id: string): Promise<Sheet>;
  getMySheet(): Promise<Sheet>;
  deleteSheet(id: string): Promise<void>;
  patchSheet(
    id: string,
    changeSet: Record<string, unknown>,
    dice?: DiceBlock,
  ): Promise<Record<string, unknown>>;
  previewSheet(id: string, changeSet: Record<string, unknown>): Promise<Record<string, unknown>>;
  setLayout(id: string, layout: Layout | null): Promise<Layout | null>;
  revokeLlmConsent(): Promise<void>;
  grantLlmConsent(version: string): Promise<void>;
  createGeneration(input: CreateGenerationInput): Promise<string>;
  getGeneration(id: string): Promise<Generation>;
  acceptGeneration(id: string, input: AcceptGenerationInput): Promise<AcceptedGeneration>;
  previewTemplate(
    templateId: string,
    changeSet: Record<string, unknown>,
  ): Promise<Record<string, unknown>>;
  joinCampaign(input: JoinCampaignInput): Promise<JoinedCampaign>;
}

export interface CreateGenerationInput {
  readonly templateId: string;
  readonly kind: 'character';
  readonly concept: string;
}

export interface AcceptGenerationInput {
  readonly sheetName: string;
  readonly values: Record<string, unknown>;
}

function decodeHealth(data: unknown): HealthPayload {
  if (!isPlainObject(data) || typeof data['status'] !== 'string') {
    throw drift('health payload is missing a string status');
  }

  return { status: data['status'] };
}

function saveBody(changeSet: Record<string, unknown>, dice: DiceBlock | undefined): object {
  if (dice === undefined) {
    return { data: changeSet };
  }
  return hasEntries(changeSet)
    ? { data: changeSet, ...diceBlockToWire(dice) }
    : diceBlockToWire(dice);
}

function joinBody(input: JoinCampaignInput): { code: string; password?: string } {
  return input.password === undefined
    ? { code: input.code }
    : { code: input.code, password: input.password };
}

function decodeChangeSet(data: unknown): Record<string, unknown> {
  if (!isPlainObject(data)) {
    throw drift('sheet update payload is not an object');
  }

  return data;
}

export function createApiClient(options: ApiClientOptions): ApiClient {
  const caller = createCaller(options);

  return {
    health: () => caller.call('GET', '/health', undefined, decodeHealth),
    listTemplates: () => caller.call('GET', '/templates', undefined, parseTemplateCatalog),
    getTemplate: (id) =>
      caller.call('GET', `/templates/${encodeURIComponent(id)}`, undefined, parseTemplateDetail),
    createSheet: (input) =>
      caller.call(
        'POST',
        '/sheets',
        JSON.stringify({
          template_id: input.templateId,
          kind: input.kind,
          sheet_name: input.sheetName,
        }),
        parseSheet,
      ),
    getMyPlan: () => caller.call('GET', '/me/plan', undefined, parsePlanUsage),
    createCheckout: () => caller.call('POST', BILLING_PATHS.checkout, undefined, parseRedirectUrl),
    openPortal: () => caller.call('POST', BILLING_PATHS.portal, undefined, parseRedirectUrl),
    requestRefund: () => caller.callNoContent('POST', BILLING_PATHS.refund, undefined),
    getSheet: (id) =>
      caller.call('GET', `/sheets/${encodeURIComponent(id)}`, undefined, parseSheet),
    getMySheet: () => caller.call('GET', '/sheets', undefined, parseSheet),
    deleteSheet: (id) =>
      caller.callNoContent('DELETE', `/sheets/${encodeURIComponent(id)}`, undefined),
    patchSheet: (id, changeSet, dice) =>
      caller.call(
        'PATCH',
        `/sheets/${encodeURIComponent(id)}`,
        JSON.stringify(saveBody(changeSet, dice)),
        decodeChangeSet,
      ),
    previewSheet: (id, changeSet) =>
      caller.call(
        'POST',
        `/sheets/${encodeURIComponent(id)}/preview`,
        JSON.stringify({ data: changeSet }),
        decodeChangeSet,
      ),
    setLayout: (id, layout) =>
      caller.call(
        'PUT',
        `/sheets/${encodeURIComponent(id)}/layout`,
        JSON.stringify({ layout: layout === null ? null : layoutToWire(layout) }),
        parseLayoutResponse,
      ),
    revokeLlmConsent: () => caller.callNoContent('DELETE', '/me/llm-consent', undefined),
    grantLlmConsent: (version) =>
      caller.callNoContent('PUT', '/me/llm-consent', JSON.stringify({ version })),
    createGeneration: (input) =>
      caller.call(
        'POST',
        '/generations',
        JSON.stringify({
          template_id: input.templateId,
          kind: input.kind,
          concept: input.concept,
        }),
        parseCreatedGenerationId,
      ),
    getGeneration: (id) =>
      caller.call('GET', `/generations/${encodeURIComponent(id)}`, undefined, parseGeneration),
    acceptGeneration: (id, input) =>
      caller.call(
        'POST',
        `/generations/${encodeURIComponent(id)}/accept`,
        JSON.stringify({ sheet_name: input.sheetName, values: input.values }),
        parseAcceptedGeneration,
      ),
    previewTemplate: (templateId, changeSet) =>
      caller.call(
        'POST',
        `/templates/${encodeURIComponent(templateId)}/preview`,
        JSON.stringify({ data: changeSet }),
        decodeChangeSet,
      ),
    joinCampaign: (input) =>
      caller.call('POST', '/campaigns/join', JSON.stringify(joinBody(input)), parseJoinedCampaign),
  };
}
