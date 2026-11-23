import type { FieldNode, FieldSchema, LeafNode, ListSchema } from '../api/models/template';
import { messageWith } from '../l10n/strings';

export interface SheetField {
  readonly path: string;
  readonly schema: FieldSchema;
}

export interface SheetExclusiveGroup {
  readonly id: string;
  readonly options: readonly SheetField[];
}

export interface ListColumnField {
  readonly kind: 'field';
  readonly name: string;
  readonly schema: FieldSchema;
}

export interface ListColumnGroup {
  readonly kind: 'exclusive';
  readonly id: string;
  readonly label: string;
  readonly options: readonly { readonly name: string; readonly schema: FieldSchema }[];
}

export type ListColumn = ListColumnField | ListColumnGroup;

export interface SheetList {
  readonly path: string;
  readonly label: string;
  readonly capacity: number;
  readonly columns: readonly ListColumn[];
}

export interface NormalRow {
  readonly kind: 'normal';
  readonly field: SheetField;
}

export interface ExclusiveRow {
  readonly kind: 'exclusive';
  readonly group: SheetExclusiveGroup;
}

export interface TableRow {
  readonly kind: 'table';
  readonly fields: readonly SheetField[];
  readonly label: string;
}

export interface RecordCellField {
  readonly kind: 'field';
  readonly field: SheetField;
}

export interface RecordCellGroup {
  readonly kind: 'exclusive';
  readonly label: string;
  readonly group: SheetExclusiveGroup;
}

export type RecordCell = RecordCellField | RecordCellGroup;

export interface RecordRow {
  readonly kind: 'record';
  readonly listPath: string;
  readonly listLabel: string;
  readonly capacity: number;
  readonly recordID: string;
  readonly position: number;
  readonly columns: readonly RecordCell[];
}

export type SheetRow = NormalRow | ExclusiveRow | TableRow | RecordRow;

export interface SheetSection {
  readonly key: string;
  readonly label: string;
  readonly leaves: readonly SheetField[];
  readonly rows: readonly SheetRow[];
  readonly lists: readonly SheetList[];
}

export type RecordRows = Readonly<Record<string, readonly string[]>>;

export const RECORD_ID_PATTERN = /^r[0-9a-f]{32}$/;

export function recordRowKey(listPath: string, recordID: string): string {
  return `${listPath}.${recordID}`;
}

export function buildSections(
  fields: Record<string, FieldNode>,
  records?: RecordRows,
): SheetSection[] {
  const sections: SheetSection[] = [];
  for (const [key, node] of Object.entries(fields)) {
    const leaves = flatten(key, node);
    const lists = listAt(key, node);
    if (leaves.length === 0 && lists.length === 0) {
      continue;
    }
    sections.push({
      key,
      label:
        node.kind === 'group'
          ? groupDisplayName(key, node.label)
          : node.kind === 'list'
            ? listLabel(key, node.list)
            : fieldDisplayName(key, node.schema),
      leaves,
      rows: buildRows(key, node, records),
      lists,
    });
  }
  return sections;
}

export function listColumns(
  shape: Readonly<Record<string, LeafNode>>,
  declared?: readonly string[],
): ListColumn[] {
  const order: string[] = [];
  for (const name of declared ?? []) {
    if (Object.hasOwn(shape, name) && !order.includes(name)) {
      order.push(name);
    }
  }
  for (const name of Object.keys(shape)) {
    if (!order.includes(name)) {
      order.push(name);
    }
  }

  const emitted = new Set<string>();
  const columns: ListColumn[] = [];
  for (const name of order) {
    const schema = shape[name]?.schema;
    if (schema === undefined) {
      continue;
    }
    const group = schema.exclusiveGroup?.trim();
    if (group !== undefined && group !== '') {
      if (emitted.has(group)) {
        continue;
      }
      emitted.add(group);
      const options = order
        .filter((option) => shape[option]?.schema.exclusiveGroup === group)
        .map((option) => ({ name: option, schema: shape[option].schema }));
      if (options.length > 1) {
        columns.push({
          kind: 'exclusive',
          id: group,
          label: groupDisplayName(group),
          options,
        });
        continue;
      }
    }
    columns.push({ kind: 'field', name, schema });
  }
  return columns;
}

export function fieldDisplayName(key: string, schema?: FieldSchema): string {
  const label = schema?.label.trim();
  if (label !== undefined && label !== '') {
    return label;
  }
  return humanize(key);
}

export function groupDisplayName(key: string, label?: string): string {
  const trimmed = label?.trim();
  if (trimmed !== undefined && trimmed !== '') {
    return trimmed;
  }
  return humanize(key);
}

export function fieldColumnName(key: string, schema: FieldSchema): string {
  const column = schema.columnLabel?.trim();
  if (column !== undefined && column !== '') {
    return column;
  }
  return fieldDisplayName(key, schema);
}

export function fieldColumnAlign(schema: FieldSchema): 'left' | 'right' {
  return schema.type === 'number' ? 'right' : 'left';
}

export function rowKey(row: SheetRow): string {
  switch (row.kind) {
    case 'normal':
      return row.field.path;
    case 'exclusive':
      return `${parentPath(row.group.options[0].path)}#${row.group.id}`;
    case 'table':
      return parentPath(row.fields[0].path);
    case 'record':
      return recordRowKey(row.listPath, row.recordID);
  }
}

export function rowFieldPaths(row: SheetRow): string[] {
  switch (row.kind) {
    case 'normal':
      return [row.field.path];
    case 'exclusive':
      return row.group.options.map((option) => option.path);
    case 'table':
      return row.fields.map((field) => field.path);
    case 'record':
      return recordCellFields(row).map((field) => field.path);
  }
}

export function recordCellFields(row: RecordRow): readonly SheetField[] {
  return row.columns.flatMap((cell) => (cell.kind === 'field' ? [cell.field] : cell.group.options));
}

export function sectionKeyOfRow(key: string): string {
  const end = key.search(/[.#]/);
  return end < 0 ? key : key.slice(0, end);
}

export function rowLabelText(row: SheetRow): string {
  switch (row.kind) {
    case 'normal':
      return fieldDisplayName(lastSegment(row.field.path), row.field.schema);
    case 'exclusive':
      return groupDisplayName(row.group.id);
    case 'table':
      return row.label;
    case 'record':
      return messageWith('record.rowLabel', { list: row.listLabel, position: row.position });
  }
}

export function tableColumnsKey(row: TableRow): string {
  return row.fields
    .map((field) => fieldColumnName(lastSegment(field.path), field.schema))
    .join('\u0000');
}

export function needsTableHeader(previous: SheetRow | undefined, row: TableRow): boolean {
  if (previous === undefined || previous.kind !== 'table') {
    return true;
  }
  return tableColumnsKey(previous) !== tableColumnsKey(row);
}

export function needsRecordHeader(previous: SheetRow | undefined, row: RecordRow): boolean {
  return previous === undefined || previous.kind !== 'record' || previous.listPath !== row.listPath;
}

export function lastSegment(path: string): string {
  const index = path.lastIndexOf('.');
  return index < 0 ? path : path.slice(index + 1);
}

function parentPath(path: string): string {
  const index = path.lastIndexOf('.');
  return index < 0 ? path : path.slice(0, index);
}

function listLabel(path: string, list: ListSchema): string {
  return list.label.trim() === '' ? humanize(lastSegment(path)) : list.label;
}

function listAt(prefix: string, node: FieldNode): SheetList[] {
  if (node.kind === 'list') {
    return [
      {
        path: prefix,
        label: listLabel(prefix, node.list),
        capacity: node.list.capacity,
        columns: listColumns(node.list.shape, node.list.columns),
      },
    ];
  }
  if (node.kind === 'leaf') {
    return [];
  }
  return Object.entries(node.children).flatMap(([key, child]) => listAt(`${prefix}.${key}`, child));
}

function flatten(prefix: string, node: FieldNode): SheetField[] {
  if (node.kind === 'leaf') {
    return [{ path: prefix, schema: node.schema }];
  }
  if (node.kind === 'list') {
    return [];
  }
  return Object.entries(node.children).flatMap(([key, child]) =>
    flatten(`${prefix}.${key}`, child),
  );
}

function buildRows(prefix: string, root: FieldNode, records?: RecordRows): SheetRow[] {
  const rows: SheetRow[] = [];
  const groupRowIndex = new Map<string, number>();
  const groupOptions = new Map<string, SheetField[]>();

  const addLeaf = (leaf: SheetField) => {
    const id = leaf.schema.exclusiveGroup;
    if (id === undefined || id === '') {
      rows.push({ kind: 'normal', field: leaf });
      return;
    }
    const existing = groupRowIndex.get(id);
    if (existing === undefined) {
      groupRowIndex.set(id, rows.length);
      groupOptions.set(id, [leaf]);
      rows.push({ kind: 'normal', field: leaf });
      return;
    }
    groupOptions.get(id)?.push(leaf);
  };

  const walk = (path: string, node: FieldNode) => {
    if (node.kind === 'leaf') {
      addLeaf({ path, schema: node.schema });
      return;
    }
    if (node.kind === 'list') {
      const list = listAt(path, node)[0];
      const ids = records?.[path] ?? [];
      ids.forEach((recordID, index) => {
        if (!RECORD_ID_PATTERN.test(recordID)) {
          return;
        }
        rows.push({
          kind: 'record',
          listPath: path,
          listLabel: list.label,
          capacity: list.capacity,
          recordID,
          position: index + 1,
          columns: list.columns.map((column) =>
            column.kind === 'field'
              ? {
                  kind: 'field',
                  field: { path: `${path}.${recordID}.${column.name}`, schema: column.schema },
                }
              : {
                  kind: 'exclusive',
                  label: column.label,
                  group: {
                    id: column.id,
                    options: column.options.map((option) => ({
                      path: `${path}.${recordID}.${option.name}`,
                      schema: option.schema,
                    })),
                  },
                },
          ),
        });
      });
      return;
    }

    const columns = node.columns;
    if (columns !== undefined && columns.length > 0) {
      const tableFields: SheetField[] = [];
      for (const key of columns) {
        const child = node.children[key];
        if (child === undefined || child.kind !== 'leaf') {
          continue;
        }
        tableFields.push({ path: `${path}.${key}`, schema: child.schema });
      }
      if (tableFields.length > 0) {
        rows.push({
          kind: 'table',
          fields: tableFields,
          label: groupDisplayName(lastSegment(path), node.label),
        });
        return;
      }
    }

    for (const [key, child] of Object.entries(node.children)) {
      walk(`${path}.${key}`, child);
    }
  };

  walk(prefix, root);

  for (const [id, options] of groupOptions) {
    if (options.length < 2) {
      continue;
    }
    const index = groupRowIndex.get(id);
    if (index !== undefined) {
      rows[index] = { kind: 'exclusive', group: { id, options } };
    }
  }

  return rows;
}

function humanize(key: string): string {
  const spaced = key
    .replace(/[_-]+/g, ' ')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2')
    .trim();
  if (spaced === '') {
    return key;
  }
  const lower = spaced.toLowerCase();
  return lower.charAt(0).toUpperCase() + lower.slice(1);
}
