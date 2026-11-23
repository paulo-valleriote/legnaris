import { describe, expect, it } from 'vitest';
import type { FieldNode, FieldSchema, GroupNode, LeafNode } from '../api/models/template';
import { validateRow, validateSection } from '../dice/fit';
import type { DiceSchema } from '../dice/fit';
import { diceSchemaFromFields, diceSectionRows } from '../sheet/dice_view';
import {
  isRowKey,
  recordRowKey,
  parseRecordRowKey,
  rowSectionKey,
  SYNTHETIC_RECORD_ID,
} from '../engine/row_identity';
import vectorsJson from '../../../testdata/records/row_identity.json';

interface RowIdentityVectors {
  readonly version: number;
  readonly fields: Readonly<Record<string, string>>;
  readonly lists: Readonly<
    Record<
      string,
      {
        readonly section: string;
        readonly shape: Readonly<Record<string, string>>;
      }
    >
  >;
  readonly rowKeys: readonly {
    readonly name: string;
    readonly key: string;
    readonly valid: boolean;
    readonly section?: string;
  }[];
  readonly fit: readonly {
    readonly name: string;
    readonly row: string;
    readonly expr: string;
    readonly fits: boolean;
    readonly class?: string;
  }[];
  readonly listDefaults: readonly {
    readonly name: string;
    readonly list: string;
    readonly expr: string;
    readonly fits: boolean;
    readonly class?: string;
  }[];
  readonly sections: readonly {
    readonly name: string;
    readonly section: string;
    readonly expr: string;
    readonly fits: boolean;
    readonly class?: string;
  }[];
}

const vectors = vectorsJson as unknown as RowIdentityVectors;

function schemaOf(kind: string): FieldSchema {
  return { type: kind, label: kind, isEditable: true, defaultValue: 0 };
}

function leaf(kind: string): FieldNode {
  return { kind: 'leaf', schema: schemaOf(kind) };
}

function insert(nodes: Record<string, FieldNode>, path: string, node: FieldNode): void {
  const segments = path.split('.');
  let cursor = nodes;
  for (const segment of segments.slice(0, -1)) {
    const existing = cursor[segment];
    if (existing?.kind === 'group') {
      cursor = existing.children;
      continue;
    }
    const created: GroupNode = { kind: 'group', children: {} };
    cursor[segment] = created;
    cursor = created.children;
  }
  cursor[segments[segments.length - 1]] = node;
}

function buildFields(): Record<string, FieldNode> {
  const fields: Record<string, FieldNode> = {};
  for (const [path, kind] of Object.entries(vectors.fields)) {
    insert(fields, path, leaf(kind));
  }
  for (const [path, list] of Object.entries(vectors.lists)) {
    const shape: Record<string, LeafNode> = {};
    for (const [name, kind] of Object.entries(list.shape)) {
      shape[name] = leaf(kind) as LeafNode;
    }
    insert(fields, path, {
      kind: 'list',
      list: { label: path, capacity: 10, shape },
    });
  }
  return fields;
}

const fields = buildFields();
const schema: DiceSchema = diceSchemaFromFields(fields);

function expectFit(
  fits: boolean,
  expectedClass: string | undefined,
  result: { ok: boolean },
): void {
  if (fits) {
    expect(result.ok).toBe(true);
    return;
  }
  expect(result.ok).toBe(false);
  const error = (result as { ok: false; error: { class: string } }).error;
  expect(error.class).toBe(expectedClass);
}

describe('shared Record row identity vectors', () => {
  it('Happy path: the vector file is the versioned contract', () => {
    expect(vectors.version).toBe(1);
  });

  it('row keys agree with the backend', () => {
    for (const vector of vectors.rowKeys) {
      expect(isRowKey(fields, vector.key), vector.name).toBe(vector.valid);
      const section = rowSectionKey(fields, vector.key);
      if (vector.valid) {
        expect(section, vector.name).toBe(vector.section);
        const row = parseRecordRowKey(fields, vector.key);
        if (row !== undefined) {
          expect(recordRowKey(row.listPath, row.recordID), vector.name).toBe(vector.key);
        }
      } else {
        expect(section, vector.name).toBeUndefined();
      }
    }
  });

  it('row fit agrees with the backend', () => {
    for (const vector of vectors.fit) {
      expectFit(vector.fits, vector.class, validateRow(schema, vector.row, vector.expr));
    }
  });

  it('list defaults agree with the backend', () => {
    for (const vector of vectors.listDefaults) {
      expectFit(
        vector.fits,
        vector.class,
        validateRow(schema, recordRowKey(vector.list, SYNTHETIC_RECORD_ID), vector.expr),
      );
    }
  });

  it('Section fit agrees with the backend', () => {
    const rows = diceSectionRows(fields);
    for (const vector of vectors.sections) {
      expectFit(
        vector.fits,
        vector.class,
        validateSection(schema, rows.get(vector.section) ?? [], vector.expr),
      );
    }
  });
});
