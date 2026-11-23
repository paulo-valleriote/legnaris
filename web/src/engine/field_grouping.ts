import type { FieldNode, FieldSchema } from '../api/models/template';

export interface SheetField {
  readonly path: string;
  readonly schema: FieldSchema;
}

export interface SheetExclusiveGroup {
  readonly id: string;
  readonly options: readonly SheetField[];
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

export type SheetRow = NormalRow | ExclusiveRow | TableRow;

export interface SheetSection {
  readonly key: string;
  readonly label: string;
  readonly leaves: readonly SheetField[];
  readonly rows: readonly SheetRow[];
}

export function buildSections(fields: Record<string, FieldNode>): SheetSection[] {
  const sections: SheetSection[] = [];
  for (const [key, node] of Object.entries(fields)) {
    const leaves = flatten(key, node);
    if (leaves.length === 0) {
      continue;
    }
    sections.push({
      key,
      label:
        node.kind === 'group'
          ? groupDisplayName(key, node.label)
          : node.kind === 'list'
            ? node.list.label.trim() === ''
              ? humanize(key)
              : node.list.label
            : fieldDisplayName(key, node.schema),
      leaves,
      rows: buildRows(key, node),
    });
  }
  return sections;
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
  }
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

export function lastSegment(path: string): string {
  const index = path.lastIndexOf('.');
  return index < 0 ? path : path.slice(index + 1);
}

function parentPath(path: string): string {
  const index = path.lastIndexOf('.');
  return index < 0 ? path : path.slice(0, index);
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

function buildRows(prefix: string, root: FieldNode): SheetRow[] {
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
