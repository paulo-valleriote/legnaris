/**
 * The generation models (LGN-196): the wire shape of `GET /generations/{id}` and the
 * accept result. A generation is server state; its status decides which step the
 * generate flow shows, so an unknown status is contract drift, never a guess.
 */

import { drift, isPlainObject, optionalString, requireString } from '../parse';

/** The statuses the backend reports, in lifecycle order (LGN-186, LGN-230). */
export const GENERATION_STATUSES = [
  'queued',
  'running',
  'succeeded',
  'failed',
  'accepted',
] as const;

/** One generation status. */
export type GenerationStatus = (typeof GENERATION_STATUSES)[number];

/** A generation as the owner reads it. `values` is present only once it succeeded. */
export interface Generation {
  readonly id: string;
  readonly status: GenerationStatus;
  readonly kind: string;
  readonly templateId: string;
  /** The generated values, keyed like the Template's fields; `undefined` before success. */
  readonly values: Record<string, unknown> | undefined;
  /** The generation's failure code (such as `invalid_output`); `undefined` unless it failed. */
  readonly errorCode: string | undefined;
  /** The Sheet name the server derived from the values; `undefined` when there is none to offer. */
  readonly suggestedName: string | undefined;
}

/** The accept answer: the Sheet the generation became (201 first, 200 on a repeat). */
export interface AcceptedGeneration {
  readonly sheetId: string;
}

function isGenerationStatus(value: string): value is GenerationStatus {
  return (GENERATION_STATUSES as readonly string[]).includes(value);
}

function decodeValues(source: Record<string, unknown>): Record<string, unknown> | undefined {
  const values = source['values'];
  if (values === undefined || values === null) {
    return undefined;
  }
  if (!isPlainObject(values)) {
    throw drift('generation values are not an object');
  }
  return values;
}

/** A blank or absent suggestion is `undefined`: the name field then stays empty. */
function decodeSuggestedName(source: Record<string, unknown>): string | undefined {
  const name = optionalString(source, 'suggested_name');
  if (name === undefined || name.trim() === '') {
    return undefined;
  }
  return name;
}

/** Decodes a generation payload; a status the client does not know is drift. */
export function parseGeneration(data: unknown): Generation {
  if (!isPlainObject(data)) {
    throw drift('generation payload is not an object');
  }
  const status = requireString(data, 'status');
  if (!isGenerationStatus(status)) {
    throw drift(`generation status "${status}" is unknown`);
  }

  return {
    id: requireString(data, 'id'),
    status,
    kind: requireString(data, 'kind'),
    templateId: requireString(data, 'template_id'),
    values: decodeValues(data),
    errorCode: optionalString(data, 'error_code'),
    suggestedName: decodeSuggestedName(data),
  };
}

/** Decodes the `POST /generations/{id}/accept` payload. */
export function parseAcceptedGeneration(data: unknown): AcceptedGeneration {
  if (!isPlainObject(data)) {
    throw drift('accept payload is not an object');
  }
  return { sheetId: requireString(data, 'sheet_id') };
}

/** Decodes the `POST /generations` payload (202) into the generation identifier. */
export function parseCreatedGenerationId(data: unknown): string {
  if (!isPlainObject(data)) {
    throw drift('create generation payload is not an object');
  }
  return requireString(data, 'id');
}
