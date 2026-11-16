import { drift, isPlainObject, optionalString, requireString } from '../parse';

export const GENERATION_STATUSES = [
  'queued',
  'running',
  'succeeded',
  'failed',
  'accepted',
] as const;

export type GenerationStatus = (typeof GENERATION_STATUSES)[number];

export interface Generation {
  readonly id: string;
  readonly status: GenerationStatus;
  readonly kind: string;
  readonly templateId: string;
  readonly values: Record<string, unknown> | undefined;
  readonly errorCode: string | undefined;
  readonly suggestedName: string | undefined;
}

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

function decodeSuggestedName(source: Record<string, unknown>): string | undefined {
  const name = optionalString(source, 'suggested_name');
  if (name === undefined || name.trim() === '') {
    return undefined;
  }
  return name;
}

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

export function parseAcceptedGeneration(data: unknown): AcceptedGeneration {
  if (!isPlainObject(data)) {
    throw drift('accept payload is not an object');
  }
  return { sheetId: requireString(data, 'sheet_id') };
}

export function parseCreatedGenerationId(data: unknown): string {
  if (!isPlainObject(data)) {
    throw drift('create generation payload is not an object');
  }
  return requireString(data, 'id');
}
