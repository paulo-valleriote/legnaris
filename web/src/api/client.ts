import type {
  Campaign,
  CampaignInvite,
  CampaignSheet,
  CampaignSheetResult,
  CampaignSummary,
  NpcLink,
} from './models/campaign';
import {
  parseCampaign,
  parseCampaignInvite,
  parseCampaignSheets,
  parseJoinedCampaign,
  parseCampaignSummaries,
  parseNpcLink,
  parseNpcLinks,
} from './models/campaign';
import type { AcceptedGeneration, Generation } from './models/generation';
import {
  parseAcceptedGeneration,
  parseCreatedGenerationId,
  parseGeneration,
} from './models/generation';
import { BILLING_PATHS, parseRedirectUrl } from './models/billing';

import type { DiceBlock } from './models/dice';
import { diceBlockToWire } from './models/dice';
import type { Layout } from './models/layout';
import { layoutToWire, parseLayoutResponse } from './models/layout';
import type { PlanUsage, SheetKind } from './models/plan';
import { parsePlanUsage } from './models/plan';
import type { Sheet } from './models/sheet';
import { parseSheet, parseSheetList } from './models/sheet';
import type { TemplateDetail, TemplateSummary } from './models/template';
import { parseTemplateCatalog, parseTemplateDetail } from './models/template';
import { drift, hasEntries, isPlainObject } from './parse';
import type { CallerOptions } from './request';
import type { RequestCaller } from './request';
import { createCaller, DEFAULT_REQUEST_TIMEOUT_MS, DEFAULT_RETRY_DELAY_MS } from './request';

export { DEFAULT_REQUEST_TIMEOUT_MS, DEFAULT_RETRY_DELAY_MS };

export interface HealthPayload {
  readonly status: string;
}

export interface CreateSheetInput {
  readonly templateId: string;
  readonly kind: SheetKind;
  readonly sheetName: string;
  readonly campaignId?: string;
}

export interface JoinCampaignInput {
  readonly code: string;
  readonly password?: string | undefined;
  readonly sheetId?: string | undefined;
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
  duplicateSheet(id: string): Promise<Sheet>;
  patchSheet(
    id: string,
    changeSet: Record<string, unknown>,
    dice?: DiceBlock,
  ): Promise<Record<string, unknown>>;
  previewSheet(id: string, changeSet: Record<string, unknown>): Promise<Record<string, unknown>>;
  resetSheet(id: string, actionId: string, requestId: string): Promise<Record<string, unknown>>;
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
  listCampaigns(): Promise<CampaignSummary[]>;
  createCampaign(input: CreateCampaignInput): Promise<Campaign>;
  getCampaign(id: string): Promise<Campaign>;
  leaveCampaign(id: string): Promise<void>;
  deleteCampaign(id: string): Promise<void>;
  changeMemberRole(campaignId: string, userId: string, action: MemberAction): Promise<Campaign>;
  removeMember(campaignId: string, userId: string): Promise<Campaign>;
  linkCharacter(campaignId: string, sheetId: string): Promise<Campaign>;
  unlinkCharacter(campaignId: string): Promise<Campaign>;
  listCampaignSheets(campaignId: string): Promise<CampaignSheet[]>;
  getCampaignSheet(
    campaignId: string,
    sheetId: string,
    etag: string | undefined,
  ): Promise<CampaignSheetResult>;
  linkNpc(campaignId: string, sheetId: string): Promise<NpcLink>;
  unlinkNpc(campaignId: string, sheetId: string): Promise<void>;
  revealNpc(campaignId: string, sheetId: string): Promise<NpcLink>;
  hideNpc(campaignId: string, sheetId: string): Promise<NpcLink>;
  listNpcLinks(campaignId: string): Promise<NpcLink[]>;
  getCampaignNpc(
    campaignId: string,
    sheetId: string,
    etag: string | undefined,
  ): Promise<CampaignSheetResult>;
  joinCampaign(input: JoinCampaignInput): Promise<Campaign>;
  getCampaignInvite(code: string): Promise<CampaignInvite>;
  previewCampaignInvite(code: string, password: string): Promise<CampaignInvite>;
  rotateInvite(campaignId: string): Promise<Campaign>;
  setCampaignPassword(campaignId: string, password: string): Promise<Campaign>;
  clearCampaignPassword(campaignId: string): Promise<Campaign>;
  listOwnedSheets(kind: SheetKind): Promise<Sheet[]>;
}

export type CreateCampaignInput =
  | { readonly name: string; readonly gameSystem: string }
  | { readonly name: string; readonly templateId: string };

export type MemberAction = 'promote' | 'demote';

export interface CreateGenerationInput {
  readonly templateId: string;
  readonly kind: SheetKind;
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

function joinBody(input: JoinCampaignInput): {
  code: string;
  password?: string;
  sheet_id?: string;
} {
  return {
    code: input.code,
    ...(input.password === undefined ? {} : { password: input.password }),
    ...(input.sheetId === undefined ? {} : { sheet_id: input.sheetId }),
  };
}

function createCampaignBody(input: CreateCampaignInput): object {
  return 'gameSystem' in input
    ? { name: input.name, game_system: input.gameSystem }
    : { name: input.name, template_id: input.templateId };
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
          ...(input.campaignId === undefined ? {} : { campaign_id: input.campaignId }),
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
    duplicateSheet: (id) =>
      caller.call('POST', `/sheets/${encodeURIComponent(id)}/duplicate`, undefined, parseSheet),
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
    resetSheet: (id, actionId, requestId) =>
      caller.call(
        'POST',
        `/sheets/${encodeURIComponent(id)}/resets/${encodeURIComponent(actionId)}`,
        JSON.stringify({ request_id: requestId }),
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
    listCampaigns: () => caller.call('GET', '/campaigns', undefined, parseCampaignSummaries),
    createCampaign: (input) =>
      caller.call('POST', '/campaigns', JSON.stringify(createCampaignBody(input)), parseCampaign),
    getCampaign: (id) =>
      caller.call('GET', `/campaigns/${encodeURIComponent(id)}`, undefined, parseCampaign),
    leaveCampaign: (id) =>
      caller.callNoContent('POST', `/campaigns/${encodeURIComponent(id)}/leave`, undefined),
    deleteCampaign: (id) =>
      caller.callNoContent('DELETE', `/campaigns/${encodeURIComponent(id)}`, undefined),
    changeMemberRole: (campaignId, userId, action) =>
      caller.call('POST', memberPath(campaignId, userId, action), undefined, parseCampaign),
    removeMember: (campaignId, userId) =>
      caller.call('POST', memberPath(campaignId, userId, 'remove'), undefined, parseCampaign),
    linkCharacter: (campaignId, sheetId) =>
      caller.call(
        'PUT',
        `/campaigns/${encodeURIComponent(campaignId)}/character`,
        JSON.stringify({ sheet_id: sheetId }),
        parseCampaign,
      ),
    unlinkCharacter: (campaignId) =>
      caller.call(
        'DELETE',
        `/campaigns/${encodeURIComponent(campaignId)}/character`,
        undefined,
        parseCampaign,
      ),
    listCampaignSheets: (campaignId) =>
      caller.call(
        'GET',
        `/campaigns/${encodeURIComponent(campaignId)}/sheets`,
        undefined,
        parseCampaignSheets,
      ),
    getCampaignSheet: (campaignId, sheetId, etag) =>
      readCampaignSheet(caller, campaignSheetPath(campaignId, 'sheets', sheetId), etag),
    linkNpc: (campaignId, sheetId) =>
      caller.call(
        'PUT',
        `/campaigns/${encodeURIComponent(campaignId)}/npcs`,
        JSON.stringify({ sheet_id: sheetId }),
        parseNpcLink,
      ),
    unlinkNpc: (campaignId, sheetId) =>
      caller.callNoContent(
        'DELETE',
        `/campaigns/${encodeURIComponent(campaignId)}/npcs/${encodeURIComponent(sheetId)}`,
        undefined,
      ),
    revealNpc: (campaignId, sheetId) =>
      caller.call(
        'PUT',
        `/campaigns/${encodeURIComponent(campaignId)}/npcs/${encodeURIComponent(sheetId)}/reveal`,
        undefined,
        parseNpcLink,
      ),
    hideNpc: (campaignId, sheetId) =>
      caller.call(
        'DELETE',
        `/campaigns/${encodeURIComponent(campaignId)}/npcs/${encodeURIComponent(sheetId)}/reveal`,
        undefined,
        parseNpcLink,
      ),
    listNpcLinks: (campaignId) =>
      caller.call(
        'GET',
        `/campaigns/${encodeURIComponent(campaignId)}/npcs`,
        undefined,
        parseNpcLinks,
      ),
    getCampaignNpc: (campaignId, sheetId, etag) =>
      readCampaignSheet(caller, campaignSheetPath(campaignId, 'npcs', sheetId), etag),
    rotateInvite: (campaignId) =>
      caller.call(
        'POST',
        `/campaigns/${encodeURIComponent(campaignId)}/invite/rotate`,
        undefined,
        parseCampaign,
      ),
    setCampaignPassword: (campaignId, password) =>
      caller.call(
        'PUT',
        `/campaigns/${encodeURIComponent(campaignId)}/password`,
        JSON.stringify({ password }),
        parseCampaign,
      ),
    clearCampaignPassword: (campaignId) =>
      caller.call(
        'DELETE',
        `/campaigns/${encodeURIComponent(campaignId)}/password`,
        undefined,
        parseCampaign,
      ),
    listOwnedSheets: (kind) =>
      caller.call(
        'GET',
        `/sheets/owned?kind=${encodeURIComponent(kind)}`,
        undefined,
        parseSheetList,
      ),
    joinCampaign: (input) =>
      caller.call('POST', '/campaigns/join', JSON.stringify(joinBody(input)), parseJoinedCampaign),
    getCampaignInvite: (code) =>
      caller.call(
        'GET',
        `/campaigns/invite/${encodeURIComponent(code)}`,
        undefined,
        parseCampaignInvite,
      ),
    previewCampaignInvite: (code, password) =>
      caller.call(
        'POST',
        `/campaigns/invite/${encodeURIComponent(code)}/preview`,
        JSON.stringify({ password }),
        parseCampaignInvite,
      ),
  };
}

function campaignSheetPath(
  campaignId: string,
  segment: 'sheets' | 'npcs',
  sheetId: string,
): string {
  return `/campaigns/${encodeURIComponent(campaignId)}/${segment}/${encodeURIComponent(sheetId)}`;
}

async function readCampaignSheet(
  caller: RequestCaller,
  path: string,
  etag: string | undefined,
): Promise<CampaignSheetResult> {
  const headers: Record<string, string> = etag === undefined ? {} : { 'if-none-match': etag };
  const result = await caller.callConditional(path, parseSheet, { headers });
  return result.notModified
    ? result
    : { notModified: false, sheet: result.value, etag: result.etag };
}

function memberPath(campaignId: string, userId: string, action: string): string {
  return `/campaigns/${encodeURIComponent(campaignId)}/members/${encodeURIComponent(userId)}/${action}`;
}
