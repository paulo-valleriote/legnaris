import type { FieldNode, FieldSchema } from '../api/models/template';
import { isPlainObject } from '../api/parse';
import { buildSections } from './field_grouping';

export function seedDefaults(fields: Record<string, FieldNode>): Record<string, unknown> {
  const data: Record<string, unknown> = {};
  for (const section of buildSections(fields)) {
    for (const field of section.leaves) {
      setPathValue(data, field.path, seedValue(field.schema));
    }
  }
  return data;
}

export function seedFromSheet(
  fields: Record<string, FieldNode>,
  data: Record<string, unknown>,
): Record<string, unknown> {
  return mergeCascade(seedDefaults(fields), data);
}

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
