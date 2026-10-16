/**
 * Shared derivation of a sheet draft from a template's field tree (ENG-19 …
 * ENG-21): every leaf seeded with its schema default under its dotted path, a
 * stored sheet overlaid per leaf, and an incremental cascade merged without
 * mutating either input. Pure derivation — no state, no request, no per-system
 * literal — shared by the editable sheet and the creation preview so the two
 * cannot drift.
 */

import type { FieldNode, FieldSchema } from '../api/models/template';
import { isPlainObject } from '../api/parse';
import { buildSections } from './field_grouping';

/**
 * Builds the initial values of [fields]: every leaf gets its seed value under
 * its dotted path, in section order (ENG-19).
 */
export function seedDefaults(fields: Record<string, FieldNode>): Record<string, unknown> {
  const data: Record<string, unknown> = {};
  for (const section of buildSections(fields)) {
    for (const field of section.leaves) {
      setPathValue(data, field.path, seedValue(field.schema));
    }
  }
  return data;
}

/**
 * Builds the initial values of an opened sheet: the schema defaults with the
 * sheet's stored [data] overlaid per leaf (ENG-20). A stored value wins at its
 * dotted path, a schema leaf the sheet omits keeps its default, and a stored
 * key absent from the schema is carried over unchanged.
 */
export function seedFromSheet(
  fields: Record<string, FieldNode>,
  data: Record<string, unknown>,
): Record<string, unknown> {
  return mergeCascade(seedDefaults(fields), data);
}

/**
 * Deep-merges [overrides] onto [base], recursing where both sides hold a plain
 * object at a key and replacing the leaf otherwise (ENG-21). Returns a fresh
 * tree and never mutates an input; an empty [overrides] returns a copy equal to
 * [base].
 */
export function mergeCascade(
  base: Record<string, unknown>,
  overrides: Record<string, unknown>,
): Record<string, unknown> {
  const result: Record<string, unknown> = { ...base };
  for (const [key, value] of Object.entries(overrides)) {
    const existing = result[key];
    result[key] =
      isPlainObject(existing) && isPlainObject(value) ? mergeCascade(existing, value) : value;
  }
  return result;
}

/**
 * The initial value of one leaf: empty for text, `false` for booleans, and the
 * numeric default for everything else (ENG-19) — mirroring the backend's
 * `SeedValue`, whose JSON `default` key stays numeric for every type.
 */
export function seedValue(schema: FieldSchema): unknown {
  const type = schema.type;
  if (type === 'text' || type === 'string') {
    return '';
  }
  if (type === 'bool') {
    return false;
  }
  return schema.defaultValue;
}

/**
 * Sets [value] at the dotted [path] of [data], creating the intermediate objects
 * the path needs and overwriting the leaf when it already exists (ENG-19). Used
 * only inside this module, on a freshly built object, never on a caller's.
 */
export function setPathValue(data: Record<string, unknown>, path: string, value: unknown): void {
  const segments = path.split('.');
  let cursor = data;
  for (const segment of segments.slice(0, -1)) {
    const next = cursor[segment];
    if (isPlainObject(next)) {
      cursor = next;
    } else {
      const created: Record<string, unknown> = {};
      cursor[segment] = created;
      cursor = created;
    }
  }
  cursor[segments[segments.length - 1]] = value;
}
