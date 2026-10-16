/**
 * Sheet payload model and the dotted-path accessor (API-10 … API-14).
 *
 * `data` is a dynamic nested structure preserved verbatim: it is never coerced,
 * flattened or mutated, so a whole number stays whole and a string that looks
 * numeric stays a string.
 */

import { PathFailure } from '../errors';
import { drift, isPlainObject, requireString } from '../parse';

/** A stored sheet (API-10, API-11). */
export interface Sheet {
  /** Server-generated identifier, opaque to the client. */
  readonly id: string;
  /** The template this sheet renders. */
  readonly templateId: string;
  /** User-supplied name; rendered as text, never as markup. */
  readonly name: string;
  /** Dynamic nested structure, preserved verbatim (API-11). */
  readonly data: Record<string, unknown>;
  /** ISO-8601 creation timestamp, as received. */
  readonly createdAt: string;
  /** ISO-8601 update timestamp, as received. */
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

/** Parses one sheet payload into its typed model (API-10). */
export function parseSheet(payload: unknown): Sheet {
  if (!isPlainObject(payload)) {
    throw drift('sheet payload is not an object');
  }

  return {
    id: requireString(payload, 'id'),
    templateId: requireString(payload, 'template_id'),
    name: requireString(payload, 'name'),
    data: parseData(payload['data']),
    createdAt: requireString(payload, 'created_at'),
    updatedAt: requireString(payload, 'updated_at'),
  };
}

/**
 * Reads the value a dotted [path] addresses inside [data] (API-12, API-13).
 *
 * A path that points at nothing — a missing segment or a non-object
 * intermediate — yields `null` without raising, because absent data is
 * legitimate; only a malformed path raises a {@link PathFailure} (API-14).
 */
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
    current = current[segment];
  }

  return current === undefined ? null : current;
}
