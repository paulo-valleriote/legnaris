import type { FieldNode, FieldSchema } from '../api/models/template';
import { buildSections, rowKey } from './field_grouping';

export const RECORD_ID_PATTERN = /^r[0-9a-f]{32}$/;

export const SYNTHETIC_RECORD_ID = 'r00000000000000000000000000000000';

export interface ListInfo {
  readonly path: string;
  readonly section: string;
  readonly shape: Readonly<Record<string, FieldSchema>>;
  readonly dice?: string;
  readonly notD20: boolean;
}

export interface RecordRow {
  readonly listPath: string;
  readonly recordID: string;
  readonly section: string;
}

export function recordRowKey(listPath: string, recordID: string): string {
  return `${listPath}.${recordID}`;
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

export function parseRecordRowKey(
  fields: Readonly<Record<string, FieldNode>>,
  key: string,
): RecordRow | undefined {
  for (const list of collectLists(fields)) {
    const prefix = `${list.path}.`;
    if (!key.startsWith(prefix)) {
      continue;
    }
    const recordID = key.slice(prefix.length);
    if (RECORD_ID_PATTERN.test(recordID)) {
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
