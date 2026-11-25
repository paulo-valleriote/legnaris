import type { FieldNode } from '../api/models/template';
import { fieldDisplayName, lastSegment } from '../engine/field_grouping';
import { recordFieldSchema } from '../engine/row_identity';

export type FieldLabelLookup = (path: string) => string | undefined;

const GENERIC_LEAF_KEYS: ReadonlySet<string> = new Set([
  'mod',
  'modifier',
  'bonus',
  'penalty',
  'total',
  'value',
  'base',
  'score',
  'max',
  'min',
]);

function parentPath(path: string): string {
  const index = path.lastIndexOf('.');
  return index < 0 ? '' : path.slice(0, index);
}

function clean(label: string | undefined): string | undefined {
  const trimmed = label?.trim();
  return trimmed === undefined || trimmed === '' ? undefined : trimmed;
}

export function resolveRollPartLabel(path: string, lookup?: FieldLabelLookup): string {
  const own = clean(lookup?.(path));
  if (own !== undefined) {
    return own;
  }
  const leaf = lastSegment(path);
  const parent = parentPath(path);
  if (parent !== '' && GENERIC_LEAF_KEYS.has(leaf)) {
    const parentLabel = clean(lookup?.(parent)) ?? fieldDisplayName(lastSegment(parent));
    return `${parentLabel} ${fieldDisplayName(leaf).toLowerCase()}`;
  }
  return fieldDisplayName(leaf);
}

export function fieldLabelFromFields(
  fields: Readonly<Record<string, FieldNode>>,
): FieldLabelLookup {
  const labels = new Map<string, string>();
  const walk = (nodes: Readonly<Record<string, FieldNode>>, prefix: string): void => {
    for (const [key, node] of Object.entries(nodes)) {
      const path = prefix === '' ? key : `${prefix}.${key}`;
      if (node.kind === 'leaf') {
        labels.set(path, node.schema.label);
        continue;
      }
      if (node.kind === 'group') {
        if (node.label !== undefined) {
          labels.set(path, node.label);
        }
        walk(node.children, path);
        continue;
      }
      labels.set(path, node.list.label);
    }
  };
  walk(fields, '');
  return (path) => clean(labels.get(path)) ?? clean(recordFieldSchema(fields, path)?.label);
}
