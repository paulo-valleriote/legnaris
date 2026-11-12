/**
 * The typed operations of the consumed backend contract (API-01 … API-07).
 *
 * Every call goes through one place: the client builds the request, issues it
 * through the injected transport and decodes the envelope exactly once, so a
 * screen never writes a URL, unwraps a response or branches on a status.
 */

import type { DiceBlock } from './models/dice';
import { diceBlockToWire } from './models/dice';
import type { Layout } from './models/layout';
import { layoutToWire, parseLayoutResponse } from './models/layout';
import type { Sheet } from './models/sheet';
import { parseSheet } from './models/sheet';
import type { TemplateDetail, TemplateSummary } from './models/template';
import { parseTemplateCatalog, parseTemplateDetail } from './models/template';
import { drift, isPlainObject } from './parse';
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
  return Object.keys(changeSet).length === 0
    ? diceBlockToWire(dice)
    : { data: changeSet, ...diceBlockToWire(dice) };
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
        JSON.stringify({ template_id: input.templateId, sheet_name: input.sheetName }),
        parseSheet,
      ),
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
  };
}
