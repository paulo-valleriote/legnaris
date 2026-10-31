/**
 * Section derivation for the schema-driven sheet renderer (ENG-01 … ENG-06):
 * one section per top-level field key, with every leaf beneath it flattened
 * into dotted paths and grouped into render rows — stacked leaves, exclusive
 * groups, or table rows declared by the template. Pure derivation: no widgets,
 * no state, no per-system literals.
 */

import type { FieldNode, FieldSchema } from '../api/models/template';

/** One leaf of a section: its dotted path and the schema that decides how it renders (ENG-02, ENG-24). */
export interface SheetField {
  readonly path: string;
  readonly schema: FieldSchema;
}

/** One mutually exclusive set inside a section: the group id plus its member leaves in API order (ENG-05, ENG-14). */
export interface SheetExclusiveGroup {
  readonly id: string;
  readonly options: readonly SheetField[];
}

/** One render line carrying a single untagged leaf (ENG-02). */
export interface NormalRow {
  readonly kind: 'normal';
  readonly field: SheetField;
}

/** One render line carrying every member of a mutually exclusive group (ENG-05, ENG-14). */
export interface ExclusiveRow {
  readonly kind: 'exclusive';
  readonly group: SheetExclusiveGroup;
}

/** One render line carrying a group label and one leaf per declared column, in declared order (ENG-04). */
export interface TableRow {
  readonly kind: 'table';
  readonly fields: readonly SheetField[];
  readonly label: string;
}

/** One render line of a section; exactly one kind is active (ENG-02, ENG-04, ENG-05). */
export type SheetRow = NormalRow | ExclusiveRow | TableRow;

/** One top-level key rendered as a block (ENG-01, ENG-02). */
export interface SheetSection {
  readonly key: string;
  readonly label: string;
  readonly leaves: readonly SheetField[];
  readonly rows: readonly SheetRow[];
}

/**
 * Derives the sections of a field tree: one per top-level key, in the order the
 * service returned the keys (ENG-01). A group with no leaf is omitted; a
 * top-level leaf becomes its own single-field section (ENG-02).
 */
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
          : fieldDisplayName(key, node.schema),
      leaves,
      rows: buildRows(key, node),
    });
  }
  return sections;
}

/** The display name of a field: the schema label when non-empty, else a humanized key (ENG-03). */
export function fieldDisplayName(key: string, schema?: FieldSchema): string {
  const label = schema?.label.trim();
  if (label !== undefined && label !== '') {
    return label;
  }
  return humanize(key);
}

/** The display name of a group or section: its declared label when non-empty, else a humanized key (ENG-03). */
export function groupDisplayName(key: string, label?: string): string {
  const trimmed = label?.trim();
  if (trimmed !== undefined && trimmed !== '') {
    return trimmed;
  }
  return humanize(key);
}

/** The short header of a leaf rendered as a table column (ENG-06). */
export function fieldColumnName(key: string, schema: FieldSchema): string {
  const column = schema.columnLabel?.trim();
  if (column !== undefined && column !== '') {
    return column;
  }
  return fieldDisplayName(key, schema);
}

/** The alignment of a table cell: a numeric column right, every other column left (ENG-06). */
export function fieldColumnAlign(schema: FieldSchema): 'left' | 'right' {
  return schema.type === 'number' ? 'right' : 'left';
}

/**
 * The stable identity of one render row inside its section: a lone field by its
 * dotted path, an exclusive group by its parent path and group id
 * (`<parent>#<id>`), a table row by the path of the group that declares its
 * columns. It depends only on the template, never on position, and matches the
 * keys the backend validates a layout against (ADR 0001).
 */
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

/** The dotted paths of every leaf a Row draws, in order. */
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

/** The key of the Section a Row key belongs to: its first path segment (before any `.` or `#`). */
export function sectionKeyOfRow(key: string): string {
  const end = key.search(/[.#]/);
  return end < 0 ? key : key.slice(0, end);
}

/** The text a Row is called wherever it is named: its label, a group's humanized id, or a table's group label. */
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

/** The column headers of a table Row joined into one comparable identity. */
export function tableColumnsKey(row: TableRow): string {
  return row.fields
    .map((field) => fieldColumnName(lastSegment(field.path), field.schema))
    .join('\u0000');
}

/**
 * Whether a table Row must draw its own column header. A run of adjacent table Rows
 * with the same columns shares one header on its first Row; a Row with any other
 * neighbour (or none) starts a run of its own. Evaluated on the Rows as drawn, so a
 * pin that changes who neighbours whom still leaves exactly one header per run.
 */
export function needsTableHeader(previous: SheetRow | undefined, row: TableRow): boolean {
  if (previous === undefined || previous.kind !== 'table') {
    return true;
  }
  return tableColumnsKey(previous) !== tableColumnsKey(row);
}

/** The segment after the last dot of a dotted field path (ENG-03, ENG-06). */
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
  return Object.entries(node.children).flatMap(([key, child]) =>
    flatten(`${prefix}.${key}`, child),
  );
}

/**
 * Collapses one section's tree into render rows: a leaf child is a normal row,
 * a declared-column group is one table row (unknown/absent columns skipped, a
 * stacked fallback when none resolves), and every non-adjacent leaf sharing a
 * non-empty exclusive id collapses into one row at the first member's position
 * — a single-member group is not collapsed (ENG-02, ENG-04, ENG-05).
 */
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
