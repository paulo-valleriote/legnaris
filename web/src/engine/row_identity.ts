import type { FieldNode, FieldSchema } from '../api/models/template';
import { isPlainObject } from '../api/parse';
import { buildSections, RECORD_ID_PATTERN, recordRowKey, rowKey } from './field_grouping';

export { RECORD_ID_PATTERN, recordRowKey };

export const SYNTHETIC_RECORD_ID = 'r00000000000000000000000000000000';

export interface ListInfo {
  readonly path: string;
  readonly section: string;
  readonly label: string;
  readonly capacity: number;
  readonly shape: Readonly<Record<string, FieldSchema>>;
  readonly dice?: string;
  readonly notD20: boolean;
}

export interface RecordRow {
  readonly listPath: string;
  readonly recordID: string;
  readonly section: string;
}

export interface RecordOrderEntry {
  readonly id: string;
  readonly position: number;
}

function collectLists(fields: Readonly<Record<string, FieldNode>>): ListInfo[] {
  const lists: ListInfo[] = [];

  const walk = (
    nodes: Readonly<Record<string, FieldNode>>,
    prefix: string,
    section: string,
  ): void => {
    for (const [key, node] of Object.entries(nodes)) {
      const path = prefix === '' ? key : `${prefix}.${key}`;
      const owner = prefix === '' ? key : section;
      if (node.kind === 'list') {
        const shape: Record<string, FieldSchema> = {};
        for (const [name, leaf] of Object.entries(node.list.shape)) {
          shape[name] = leaf.schema;
        }
        lists.push({
          path,
          section: owner,
          label: node.list.label,
          capacity: node.list.capacity,
          shape,
          ...(node.list.dice === undefined ? {} : { dice: node.list.dice }),
          notD20: node.list.notD20 === true,
        });
        continue;
      }
      if (node.kind === 'group') {
        walk(node.children, path, owner);
      }
    }
  };

  walk(fields, '', '');
  return lists.sort((left, right) => right.path.length - left.path.length);
}

export function listInfos(fields: Readonly<Record<string, FieldNode>>): readonly ListInfo[] {
  return collectLists(fields);
}

export const RECORD_POSITION_FIELD = 'position';

export function isRecordPositionPath(
  fields: Readonly<Record<string, FieldNode>>,
  path: string,
): boolean {
  const suffix = `.${RECORD_POSITION_FIELD}`;
  if (!path.endsWith(suffix)) {
    return false;
  }
  return parseRecordRowKey(fields, path.slice(0, -suffix.length)) !== undefined;
}

export function recordRowID(listPath: string, key: string): string | undefined {
  const prefix = `${listPath}.`;
  if (!key.startsWith(prefix)) {
    return undefined;
  }
  const recordID = key.slice(prefix.length);
  return RECORD_ID_PATTERN.test(recordID) ? recordID : undefined;
}

export function parseRecordRowKey(
  fields: Readonly<Record<string, FieldNode>>,
  key: string,
): RecordRow | undefined {
  for (const list of collectLists(fields)) {
    const recordID = recordRowID(list.path, key);
    if (recordID !== undefined) {
      return { listPath: list.path, recordID, section: list.section };
    }
  }
  return undefined;
}

export function recordFieldSchema(
  fields: Readonly<Record<string, FieldNode>>,
  path: string,
): FieldSchema | undefined {
  for (const list of collectLists(fields)) {
    const prefix = `${list.path}.`;
    if (!path.startsWith(prefix)) {
      continue;
    }
    const remainder = path.slice(prefix.length);
    const dot = remainder.indexOf('.');
    if (dot < 0) {
      continue;
    }
    const recordID = remainder.slice(0, dot);
    const field = remainder.slice(dot + 1);
    if (!RECORD_ID_PATTERN.test(recordID) || field.includes('.')) {
      continue;
    }
    return list.shape[field];
  }
  return undefined;
}

function readPath(data: Record<string, unknown>, path: string): unknown {
  let current: unknown = data;
  for (const segment of path.split('.')) {
    if (!isPlainObject(current)) {
      return undefined;
    }
    current = current[segment];
  }
  return current;
}

export function orderedRecords(
  listPath: string,
  data: Readonly<Record<string, unknown>>,
): readonly RecordOrderEntry[] {
  const stored = readPath(data as Record<string, unknown>, listPath);
  if (!isPlainObject(stored)) {
    return [];
  }
  const entries: RecordOrderEntry[] = [];
  for (const [id, value] of Object.entries(stored)) {
    if (!RECORD_ID_PATTERN.test(id) || !isPlainObject(value) || value === null) {
      continue;
    }
    const raw = value.position;
    const position =
      typeof raw === 'number' && Number.isFinite(raw) ? raw : Number.MAX_SAFE_INTEGER;
    entries.push({ id, position });
  }
  return entries.sort((left, right) =>
    left.position === right.position
      ? left.id.localeCompare(right.id)
      : left.position - right.position,
  );
}

export function orderedRecordIDs(
  fields: Readonly<Record<string, FieldNode>>,
  listPath: string,
  data: Readonly<Record<string, unknown>>,
): readonly string[] {
  if (listInfoAtPath(fields, listPath) === undefined) {
    return [];
  }
  return orderedRecords(listPath, data).map((entry) => entry.id);
}

export function recordsByList(
  fields: Readonly<Record<string, FieldNode>>,
  data: Readonly<Record<string, unknown>>,
): Record<string, readonly string[]> {
  const rows: Record<string, readonly string[]> = {};
  for (const list of collectLists(fields)) {
    rows[list.path] = orderedRecords(list.path, data).map((entry) => entry.id);
  }
  return rows;
}

export function listInfoAtPath(
  fields: Readonly<Record<string, FieldNode>>,
  listPath: string,
): ListInfo | undefined {
  return collectLists(fields).find((list) => list.path === listPath);
}

export function newRecordID(random: (bytes: Uint8Array) => void = defaultRandomBytes): string {
  const bytes = new Uint8Array(16);
  random(bytes);
  let hex = '';
  for (const byte of bytes) {
    hex += byte.toString(16).padStart(2, '0');
  }
  return `r${hex}`;
}

function defaultRandomBytes(bytes: Uint8Array): void {
  const buffer = new Uint8Array(new ArrayBuffer(bytes.length));
  globalThis.crypto.getRandomValues(buffer);
  bytes.set(buffer);
}

function staticRows(fields: Readonly<Record<string, FieldNode>>): Map<string, string> {
  const rows = new Map<string, string>();
  for (const section of buildSections({ ...fields })) {
    for (const row of section.rows) {
      rows.set(rowKey(row), section.key);
    }
  }
  return rows;
}

export function isRowKey(fields: Readonly<Record<string, FieldNode>>, key: string): boolean {
  return staticRows(fields).has(key) || parseRecordRowKey(fields, key) !== undefined;
}

export function rowSectionKey(
  fields: Readonly<Record<string, FieldNode>>,
  key: string,
): string | undefined {
  const staticSection = staticRows(fields).get(key);
  if (staticSection !== undefined) {
    return staticSection;
  }
  return parseRecordRowKey(fields, key)?.section;
}
