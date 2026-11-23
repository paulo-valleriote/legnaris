import { parseDiceDefaults, type DiceDefaults } from './dice';
import { SHEET_KINDS } from './plan';
import type { SheetKind } from './plan';
import {
  drift,
  isPlainObject,
  optionalBoolean,
  optionalString,
  requireBoolean,
  requireString,
  requireNumber,
} from '../parse';

export interface FieldSchema {
  readonly type: string;
  readonly label: string;
  readonly formula?: string;
  readonly dependsOn?: readonly (readonly string[])[];
  readonly isEditable: boolean;
  readonly defaultValue: number | string;
  readonly exclusiveGroup?: string;
  readonly optionLabel?: string;
  readonly columnLabel?: string;
  readonly flag?: boolean;
}

export interface LeafNode {
  readonly kind: 'leaf';
  readonly schema: FieldSchema;
}

export interface GroupNode {
  readonly kind: 'group';
  readonly children: Record<string, FieldNode>;
  readonly label?: string;
  readonly columns?: readonly string[];
}

export interface ListSchema {
  readonly label: string;
  readonly columns?: readonly string[];
  readonly capacity: number;
  readonly shape: Record<string, LeafNode>;
  readonly dice?: string;
  readonly notD20?: boolean;
}

export interface ListNode {
  readonly kind: 'list';
  readonly list: ListSchema;
}

export type FieldNode = LeafNode | GroupNode | ListNode;

export interface TemplateDetail {
  readonly id: string;
  readonly name: string;
  readonly kinds: readonly SheetKind[];
  readonly fields: Record<string, FieldNode>;
  readonly default: boolean;
  readonly dice?: DiceDefaults;
}

export type TemplateSummary = TemplateDetail;

function parseDependsOn(value: unknown): readonly (readonly string[])[] | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }

  if (!Array.isArray(value)) {
    throw drift('template field "depends_on" is not a list');
  }

  return value.map((dependency) => {
    if (!Array.isArray(dependency)) {
      throw drift('template field "depends_on" holds a malformed dependency');
    }

    const segments: string[] = [];
    for (const segment of dependency) {
      if (typeof segment !== 'string') {
        throw drift('template field "depends_on" holds a non-string path segment');
      }
      segments.push(segment);
    }
    return segments;
  });
}

function parseFieldSchema(value: unknown): FieldSchema {
  if (!isPlainObject(value)) {
    throw drift('template leaf schema is not an object');
  }

  return {
    type: requireString(value, 'type'),
    label: requireString(value, 'label'),
    isEditable: requireBoolean(value, 'is_editable'),
    defaultValue: requireNumber(value, 'default'),
    formula: optionalString(value, 'formula'),
    dependsOn: parseDependsOn(value['depends_on']),
    exclusiveGroup: optionalString(value, 'exclusive_group'),
    optionLabel: optionalString(value, 'option_label'),
    columnLabel: optionalString(value, 'column_label'),
    flag: optionalBoolean(value, 'flag'),
  };
}

function parseColumns(value: unknown): readonly string[] | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }

  if (!Array.isArray(value)) {
    throw drift('template group "columns" is not a list');
  }

  const columns: string[] = [];
  for (const column of value) {
    if (typeof column !== 'string') {
      throw drift('template group "columns" holds a non-string entry');
    }
    columns.push(column);
  }
  return columns;
}

function parseListShape(value: unknown): Record<string, LeafNode> {
  const group = parseFieldGroup(value);
  const shape: Record<string, LeafNode> = {};
  for (const [key, node] of Object.entries(group)) {
    if (node.kind !== 'leaf') {
      throw drift('template list shape holds a node that is not a leaf');
    }
    shape[key] = node;
  }
  return shape;
}

function parseListSchema(value: unknown): ListSchema {
  if (!isPlainObject(value)) {
    throw drift('template list is not an object');
  }

  const dice = optionalString(value, 'dice');
  return {
    label: requireString(value, 'label'),
    columns: parseColumns(value['columns']),
    capacity: requireNumber(value, 'capacity'),
    shape: parseListShape(value['shape']),
    ...(dice === undefined ? {} : { dice }),
    ...(value['not_d20'] === true ? { notD20: true } : {}),
  };
}

function parseFieldNode(value: unknown): FieldNode {
  if (!isPlainObject(value)) {
    throw drift('template field node is not an object');
  }

  if (value['schema'] !== undefined && value['schema'] !== null) {
    return { kind: 'leaf', schema: parseFieldSchema(value['schema']) };
  }

  if (value['list'] !== undefined && value['list'] !== null) {
    return { kind: 'list', list: parseListSchema(value['list']) };
  }

  if (value['children'] !== undefined && value['children'] !== null) {
    return {
      kind: 'group',
      children: parseFieldGroup(value['children']),
      label: optionalString(value, 'label'),
      columns: parseColumns(value['columns']),
    };
  }

  throw drift('template field node carries neither a schema, children nor a list');
}

function parseFieldGroup(value: unknown): Record<string, FieldNode> {
  if (!isPlainObject(value)) {
    throw drift('template field group "children" is not an object');
  }

  const nodes: Record<string, FieldNode> = {};
  for (const key of Object.keys(value)) {
    nodes[key] = parseFieldNode(value[key]);
  }
  return nodes;
}

function parseKinds(value: unknown): readonly SheetKind[] {
  if (!Array.isArray(value)) {
    throw drift('template "kinds" is not a list');
  }

  if (value.length === 0) {
    throw drift('template "kinds" is empty');
  }

  const kinds: SheetKind[] = [];
  for (const entry of value) {
    if (typeof entry !== 'string') {
      throw drift('template "kinds" holds a non-string entry');
    }
    const match = SHEET_KINDS.find((kind) => kind === entry);
    if (match === undefined) {
      throw drift('template "kinds" holds an unknown kind');
    }
    if (kinds.includes(match)) {
      throw drift('template "kinds" holds a duplicate kind');
    }
    kinds.push(match);
  }
  return kinds;
}

function parseTemplate(value: unknown): TemplateDetail {
  if (!isPlainObject(value)) {
    throw drift('template payload is not an object');
  }

  if (!isPlainObject(value['fields'])) {
    throw drift('template "fields" is not an object');
  }

  return {
    id: requireString(value, 'id'),
    name: requireString(value, 'name'),
    kinds: parseKinds(value['kinds']),
    default: requireBoolean(value, 'default'),
    fields: parseFieldGroup(value['fields']),
    dice: parseDiceDefaults(value['dice']),
  };
}

export function parseTemplateDetail(payload: unknown): TemplateDetail {
  return parseTemplate(payload);
}

export function parseTemplateCatalog(payload: unknown): TemplateSummary[] {
  if (!Array.isArray(payload)) {
    throw drift('template catalog payload is not a list');
  }

  return payload.map((entry) => parseTemplate(entry));
}
