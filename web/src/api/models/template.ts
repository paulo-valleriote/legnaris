import { parseDiceDefaults, type DiceDefaults } from './dice';
import {
  drift,
  isPlainObject,
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

export type FieldNode = LeafNode | GroupNode;

export interface TemplateDetail {
  readonly id: string;
  readonly name: string;
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

function parseFieldNode(value: unknown): FieldNode {
  if (!isPlainObject(value)) {
    throw drift('template field node is not an object');
  }

  if (value['schema'] !== undefined && value['schema'] !== null) {
    return { kind: 'leaf', schema: parseFieldSchema(value['schema']) };
  }

  if (value['children'] !== undefined && value['children'] !== null) {
    return {
      kind: 'group',
      children: parseFieldGroup(value['children']),
      label: optionalString(value, 'label'),
      columns: parseColumns(value['columns']),
    };
  }

  throw drift('template field node carries neither a schema nor children');
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
