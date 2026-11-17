import { PathFailure } from '../errors';
import { drift, isPlainObject, requireString } from '../parse';
import type { Layout } from './layout';
import type { DiceBlock } from './dice';
import { parseDiceBlock } from './dice';
import { parseOptionalLayout } from './layout';
import { SHEET_KINDS } from './plan';
import type { SheetKind } from './plan';

export interface Sheet {
  readonly id: string;
  readonly templateId: string;
  readonly kind?: SheetKind;
  readonly name: string;
  readonly data: Record<string, unknown>;
  readonly layout?: Layout;
  readonly dice?: DiceBlock;
  readonly campaignId?: string;
  readonly createdAt: string;
  readonly updatedAt: string;
}

function parseData(value: unknown): Record<string, unknown> {
  if (value === null || value === undefined) {
    return {};
  }

  if (!isPlainObject(value)) {
    throw drift('sheet "data" is not an object');
  }

  return value;
}

function withLayout(layout: Layout | undefined): { readonly layout?: Layout } {
  return layout === undefined ? {} : { layout };
}

function parseKind(payload: Record<string, unknown>): SheetKind | undefined {
  const value = payload['kind'];
  if (value === undefined) {
    return undefined;
  }
  const match = SHEET_KINDS.find((kind) => kind === value);
  if (match === undefined) {
    throw drift('sheet "kind" is not a known value');
  }
  return match;
}

export function isLinkableSheetKind(kind: SheetKind | undefined): kind is 'npc' | 'monster' {
  return kind === 'npc' || kind === 'monster';
}

export function parseSheet(payload: unknown): Sheet {
  if (!isPlainObject(payload)) {
    throw drift('sheet payload is not an object');
  }

  return {
    id: requireString(payload, 'id'),
    templateId: requireString(payload, 'template_id'),
    kind: parseKind(payload),
    name: requireString(payload, 'name'),
    data: parseData(payload['data']),
    ...withLayout(parseOptionalLayout(payload['layout'])),
    dice: parseDiceBlock(payload['dice']),
    ...(typeof payload['campaign_id'] === 'string' ? { campaignId: payload['campaign_id'] } : {}),
    createdAt: requireString(payload, 'created_at'),
    updatedAt: requireString(payload, 'updated_at'),
  };
}

export function valueAtPath(data: Record<string, unknown>, path: string): unknown {
  const segments = path.split('.');

  if (path === '' || segments.some((segment) => segment === '')) {
    throw new PathFailure(path);
  }

  let current: unknown = data;
  for (const segment of segments) {
    if (!isPlainObject(current)) {
      return null;
    }
    const next = ownValue(current, segment);
    if (next === undefined) {
      return null;
    }
    current = next;
  }

  return current === undefined ? null : current;
}

function ownValue(source: Record<string, unknown>, key: string): unknown {
  return Object.hasOwn(source, key) ? source[key] : undefined;
}

export function parseSheetList(payload: unknown): Sheet[] {
  if (!Array.isArray(payload)) {
    throw drift('sheet list is not an array');
  }
  return payload.map(parseSheet);
}
