/**
 * Template payload models (API-08, API-09).
 *
 * The backend reuses one shape for the catalog and the detail, so a single
 * decoder serves both: the field tree is rebuilt level by level, preserving the
 * order the backend emitted, because the interface renders sections in the
 * declared order and never sorts them.
 */

import {
  drift,
  isPlainObject,
  optionalString,
  requireBoolean,
  requireString,
  requireNumber,
} from '../parse';

/** A leaf's declared contract (API-09). */
export interface FieldSchema {
  /** Opaque here; the engine feature maps it to a control. */
  readonly type: string;
  /** Display label. */
  readonly label: string;
  /** Present only when the backend declares a formula; never an empty string (API-09). */
  readonly formula?: string;
  /** Present only when the backend declares dependencies; never an empty list (API-09). */
  readonly dependsOn?: readonly (readonly string[])[];
  /** Whether the value is user-editable. */
  readonly isEditable: boolean;
  /** The declared default value. */
  readonly defaultValue: number;
  /** Optional exclusive-group membership, when the payload carries it (API-09). */
  readonly exclusiveGroup?: string;
  /** Optional option label, when the payload carries it (API-09). */
  readonly optionLabel?: string;
  /** Optional column label, when the payload carries it (API-09). */
  readonly columnLabel?: string;
}

/** A field-tree node that carries a schema (API-08). */
export interface LeafNode {
  readonly kind: 'leaf';
  readonly schema: FieldSchema;
}

/** A field-tree node that carries children, plus the optional table metadata (API-08). */
export interface GroupNode {
  readonly kind: 'group';
  readonly children: Record<string, FieldNode>;
  /** Table/section label, when the payload declares it. */
  readonly label?: string;
  /** Declared column order, when the payload declares it. */
  readonly columns?: readonly string[];
}

/** A node of the template field tree (API-08). */
export type FieldNode = LeafNode | GroupNode;

/** A decoded template; the catalog entry and the detail share this shape (API-08). */
export interface TemplateDetail {
  /** Template slug, code-defined on the backend. */
  readonly id: string;
  /** Display name. */
  readonly name: string;
  /** Top-level groups, in the payload's order. */
  readonly fields: Record<string, FieldNode>;
  /** Whether this is the default template; read by key, so `false` is a value (API-08). */
  readonly default: boolean;
}

/** A catalog entry is a decoded template (API-08). */
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

/**
 * Parses one field-tree node, classifying it by the key it carries (API-08).
 *
 * A node with a `schema` is a leaf and a node with `children` is a group; a node
 * carrying neither is contract drift rather than an empty node.
 */
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
  };
}

/** Parses one template detail payload (API-08). */
export function parseTemplateDetail(payload: unknown): TemplateDetail {
  return parseTemplate(payload);
}

/** Parses the template catalog payload into decoded entries (API-08). */
export function parseTemplateCatalog(payload: unknown): TemplateSummary[] {
  if (!Array.isArray(payload)) {
    throw drift('template catalog payload is not a list');
  }

  return payload.map((entry) => parseTemplate(entry));
}
