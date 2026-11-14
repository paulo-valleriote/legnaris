/**
 * The typed operations of the consumed backend contract (API-01 … API-07).
 *
 * Every call goes through one place: the client builds the request, issues it
 * through the injected transport and decodes the envelope exactly once, so a
 * screen never writes a URL, unwraps a response or branches on a status.
 */

import type { AcceptedGeneration, Generation } from './models/generation';
import {
  parseAcceptedGeneration,
  parseCreatedGenerationId,
  parseGeneration,
} from './models/generation';
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

/** The `/health` payload (API-02). */
export interface HealthPayload {
  readonly status: string;
}

/** The sheet-creation input (API-05). */
export interface CreateSheetInput {
  readonly templateId: string;
  /** The Sheet kind; the plan's allowance is counted per kind (LGN-183). */
  readonly kind: SheetKind;
  readonly sheetName: string;
}

/** Construction options for {@link createApiClient} (API-07). */
export type ApiClientOptions = CallerOptions;

/** The consumed operations, one method each (API-01, WAS-25, WAS-28). */
export interface ApiClient {
  health(): Promise<HealthPayload>;
  listTemplates(): Promise<TemplateSummary[]>;
  getTemplate(id: string): Promise<TemplateDetail>;
  createSheet(input: CreateSheetInput): Promise<Sheet>;
  /** Reads the caller's plan and the Sheets held per kind against its allowance (LGN-183). */
  getMyPlan(): Promise<PlanUsage>;
  getSheet(id: string): Promise<Sheet>;
  /** Reads the signed-in caller's single owner-scoped sheet (WAS-25). */
  getMySheet(): Promise<Sheet>;
  /** Deletes one sheet by identifier; a `204` carries no body (WAS-28). */
  deleteSheet(id: string): Promise<void>;
  /**
   * Saves the sheet (ADR 0006): the data delta and, when given, the full Dice block, in one
   * request. An empty delta is left out of a save that carries a block, so the server skips
   * the recalculation. Resolves with the cascade.
   */
  patchSheet(
    id: string,
    changeSet: Record<string, unknown>,
    dice?: DiceBlock,
  ): Promise<Record<string, unknown>>;
  previewSheet(id: string, changeSet: Record<string, unknown>): Promise<Record<string, unknown>>;
  /**
   * Replaces the sheet's Layout in full; `null` resets it to the Template default.
   * Resolves with the Layout as stored, or `null` after a reset. No recalculation runs.
   */
  setLayout(id: string, layout: Layout | null): Promise<Layout | null>;
  /**
   * Withdraws the caller's LLM consent and deletes the generation data the server holds for
   * them (ADR 0008). A `204` carries no body; a failure is never repeated.
   */
  revokeLlmConsent(): Promise<void>;
  /** Records the caller's consent to the disclosure `version` (ADR 0008, LGN-194). A `204`. */
  grantLlmConsent(version: string): Promise<void>;
  /** Queues a character generation (LGN-186); resolves with the generation identifier (202). */
  createGeneration(input: CreateGenerationInput): Promise<string>;
  /** Reads one of the caller's generations; a foreign or expired id is `NOT_FOUND`. */
  getGeneration(id: string): Promise<Generation>;
  /** Turns a succeeded generation into a Sheet; repeating it returns the same Sheet (201, then 200). */
  acceptGeneration(id: string, input: AcceptGenerationInput): Promise<AcceptedGeneration>;
  /** The template-seeded dry run: the derived values for a data delta, nothing stored (LGN-196). */
  previewTemplate(
    templateId: string,
    changeSet: Record<string, unknown>,
  ): Promise<Record<string, unknown>>;
}

/** The generation request (LGN-186). Only the character kind exists in F1a. */
export interface CreateGenerationInput {
  readonly templateId: string;
  readonly kind: 'character';
  readonly concept: string;
}

/** The accept request: the Sheet name and the values the player kept (LGN-186). */
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

function decodeChangeSet(data: unknown): Record<string, unknown> {
  if (!isPlainObject(data)) {
    throw drift('sheet update payload is not an object');
  }

  return data;
}

/**
 * Builds the client over a resolved base address and an injected transport
 * (API-07).
 *
 * Every attempt is bounded by a fail-fast timeout; a read that fails at the
 * transport is repeated exactly once after a fixed delay, while a response —
 * any status — ends the attempt, and a write never repeats (API-30 … API-33).
 */
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
  };
}
