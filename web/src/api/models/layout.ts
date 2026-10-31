/**
 * The Layout model: the player's chosen order of Sections and which Rows are
 * pinned (ADR 0001).
 *
 * It is user data that belongs to the Sheet, so it is decoded as defensively as
 * any other payload: a list of the wrong shape is contract drift, never a
 * coerced value. Keys are opaque here; resolving them against a Template is the
 * engine's job.
 */

import { drift, isPlainObject } from '../parse';

/** The arrangement as the backend stores and returns it; every list may be partial or absent. */
export interface Layout {
  /** Section keys in the chosen order. */
  readonly sectionOrder: readonly string[];
  /** Pinned Row keys, in the order they were pinned. */
  readonly pinned: readonly string[];
}

/** The wire shape of a Layout: the same two lists in snake_case, each omitted when empty. */
export interface LayoutWire {
  readonly section_order?: readonly string[];
  readonly pinned?: readonly string[];
}

function parseKeys(value: unknown, name: string): readonly string[] {
  if (value === undefined || value === null) {
    return [];
  }

  if (!Array.isArray(value)) {
    throw drift(`layout "${name}" is not a list`);
  }

  const keys: string[] = [];
  for (const entry of value) {
    if (typeof entry !== 'string') {
      throw drift(`layout "${name}" holds a non-string entry`);
    }
    keys.push(entry);
  }
  return keys;
}

/**
 * Parses one layout payload into its typed model.
 *
 * An absent or `null` list decodes as empty (the backend omits empty lists); a
 * list of the wrong shape or a non-string key is a
 * {@link DefensiveParseFailure}.
 */
export function parseLayout(payload: unknown): Layout {
  if (!isPlainObject(payload)) {
    throw drift('layout payload is not an object');
  }

  return {
    sectionOrder: parseKeys(payload['section_order'], 'section_order'),
    pinned: parseKeys(payload['pinned'], 'pinned'),
  };
}

/** Parses the optional `layout` of a sheet: absent or `null` is "no layout", the Template default. */
export function parseOptionalLayout(value: unknown): Layout | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  return parseLayout(value);
}

function isKeyList(value: unknown): value is readonly string[] {
  return Array.isArray(value) && value.every((entry) => typeof entry === 'string');
}

/** Structural check of an already decoded Layout, for records read back from local storage. */
export function isLayout(value: unknown): value is Layout {
  if (!isPlainObject(value)) {
    return false;
  }
  return isKeyList(value['sectionOrder']) && isKeyList(value['pinned']);
}

/** True when a Layout carries no ordering at all, which is what a reset to default looks like. */
export function isEmptyLayout(layout: Layout | undefined): boolean {
  return layout === undefined || (layout.sectionOrder.length === 0 && layout.pinned.length === 0);
}

/** Serializes a Layout to the wire shape, omitting every empty list as the backend does. */
export function layoutToWire(layout: Layout): LayoutWire {
  return {
    ...(layout.sectionOrder.length > 0 ? { section_order: layout.sectionOrder } : {}),
    ...(layout.pinned.length > 0 ? { pinned: layout.pinned } : {}),
  };
}

/**
 * Decodes the `PUT /sheets/:id/layout` answer: `{"layout": {...}}` echoes what
 * was stored and `{"layout": null}` confirms a reset to default.
 */
export function parseLayoutResponse(payload: unknown): Layout | null {
  if (!isPlainObject(payload) || !('layout' in payload)) {
    throw drift('layout response is missing "layout"');
  }
  const layout = payload['layout'];
  if (layout === null) {
    return null;
  }
  return parseLayout(layout);
}
