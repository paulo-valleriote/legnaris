import { drift, isPlainObject } from '../parse';

export interface Layout {
  readonly sectionOrder: readonly string[];
  readonly pinned: readonly string[];
}

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

export function parseLayout(payload: unknown): Layout {
  if (!isPlainObject(payload)) {
    throw drift('layout payload is not an object');
  }

  return {
    sectionOrder: parseKeys(payload['section_order'], 'section_order'),
    pinned: parseKeys(payload['pinned'], 'pinned'),
  };
}

export function parseOptionalLayout(value: unknown): Layout | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  return parseLayout(value);
}

function isKeyList(value: unknown): value is readonly string[] {
  return Array.isArray(value) && value.every((entry) => typeof entry === 'string');
}

export function isLayout(value: unknown): value is Layout {
  if (!isPlainObject(value)) {
    return false;
  }
  return isKeyList(value['sectionOrder']) && isKeyList(value['pinned']);
}

export function isEmptyLayout(layout: Layout | undefined): boolean {
  return layout === undefined || (layout.sectionOrder.length === 0 && layout.pinned.length === 0);
}

export function layoutToWire(layout: Layout): LayoutWire {
  return {
    ...(layout.sectionOrder.length > 0 ? { section_order: layout.sectionOrder } : {}),
    ...(layout.pinned.length > 0 ? { pinned: layout.pinned } : {}),
  };
}

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
