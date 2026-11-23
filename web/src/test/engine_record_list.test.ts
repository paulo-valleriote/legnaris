import { describe, expect, it } from 'vitest';
import type { FieldNode, ListSchema } from '../api/models/template';
import { buildSections, rowFieldPaths, rowKey, rowLabelText } from '../engine/field_grouping';
import { messageWith } from '../l10n/strings';
import { LIST_CAPACITY, LIST_PATH, LIST_RECORD_A, LIST_RECORD_B, listFields } from './list_fixture';

function listNode(fields: Record<string, FieldNode>): ListSchema {
  const inventory = fields.inventory;
  if (inventory?.kind !== 'group') {
    throw new Error('the fixture lost its inventory group');
  }
  const items = inventory.children.items;
  if (items?.kind !== 'list') {
    throw new Error('the fixture lost its items list');
  }
  return items.list;
}

function columnNames(rows: ReturnType<typeof buildSections>): string[] {
  const section = rows.find((entry) => entry.key === 'inventory');
  const row = section?.rows[0];
  if (row?.kind !== 'record') {
    return [];
  }
  return row.columns.map((column) =>
    column.kind === 'field' ? column.field.schema.label : column.label,
  );
}

describe('record rows in the section model', () => {
  it('Happy path: a list is a section of one record row per id, in the order given', () => {
    const sections = buildSections(listFields(), {
      [LIST_PATH]: [LIST_RECORD_A, LIST_RECORD_B],
    });
    const section = sections.find((entry) => entry.key === 'inventory');

    expect(section).toBeDefined();
    expect(section?.label).toBe('Inventory');
    expect(section?.lists.map((list) => list.path)).toEqual([LIST_PATH]);
    expect(section?.lists[0]?.capacity).toBe(LIST_CAPACITY);
    expect(section?.rows.map(rowKey)).toEqual([
      `${LIST_PATH}.${LIST_RECORD_A}`,
      `${LIST_PATH}.${LIST_RECORD_B}`,
    ]);
    expect(section?.rows.map((row) => row.kind)).toEqual(['record', 'record']);
    const first = section?.rows[0];
    expect(first?.kind).toBe('record');
    if (first?.kind !== 'record') {
      return;
    }
    expect(first.listPath).toBe(LIST_PATH);
    expect(first.recordID).toBe(LIST_RECORD_A);
    expect(first.listLabel).toBe('Items');
    expect(rowLabelText(first)).toBe(
      messageWith('record.rowLabel', { list: 'Items', position: 1 }),
    );
    expect(section?.rows[1] === undefined ? '' : rowLabelText(section.rows[1])).toBe(
      messageWith('record.rowLabel', { list: 'Items', position: 2 }),
    );
  });

  it('Happy path: a record row carries every column cell with its full dotted path', () => {
    const sections = buildSections(listFields(), { [LIST_PATH]: [LIST_RECORD_A] });
    const row = sections.find((entry) => entry.key === 'inventory')?.rows[0];

    expect(row?.kind).toBe('record');
    if (row?.kind !== 'record') {
      return;
    }
    expect(columnNames(sections)).toEqual([
      'Name',
      'Quantity',
      'Weight',
      'Active',
      'Pick',
      'Derived total',
      'Scaled',
    ]);
    const fieldPaths = row.columns.flatMap((column) =>
      column.kind === 'field' ? [column.field.path] : column.group.options.map((o) => o.path),
    );
    expect(fieldPaths).toEqual([
      `${LIST_PATH}.${LIST_RECORD_A}.name`,
      `${LIST_PATH}.${LIST_RECORD_A}.quantity`,
      `${LIST_PATH}.${LIST_RECORD_A}.weight`,
      `${LIST_PATH}.${LIST_RECORD_A}.active`,
      `${LIST_PATH}.${LIST_RECORD_A}.optionA`,
      `${LIST_PATH}.${LIST_RECORD_A}.optionB`,
      `${LIST_PATH}.${LIST_RECORD_A}.derivedTotal`,
      `${LIST_PATH}.${LIST_RECORD_A}.scaled`,
    ]);
    expect(rowFieldPaths(row)).toEqual(fieldPaths);
  });

  it('Happy path: the exclusive members of a shape share one choice column', () => {
    const sections = buildSections(listFields(), { [LIST_PATH]: [LIST_RECORD_A] });
    const row = sections.find((entry) => entry.key === 'inventory')?.rows[0];

    expect(row?.kind).toBe('record');
    if (row?.kind !== 'record') {
      return;
    }
    const choices = row.columns.filter((column) => column.kind === 'exclusive');
    expect(choices).toHaveLength(1);
    const choice = choices[0];
    expect(choice?.kind === 'exclusive' && choice.group.id).toBe('pick');
    expect(
      choice?.kind === 'exclusive'
        ? choice.group.options.map((option) => option.schema.optionLabel)
        : [],
    ).toEqual(['A', 'B']);
  });

  it('Edge: the declared columns lead and the rest of the shape follows them', () => {
    const fields = listFields();
    const list = listNode(fields);
    const shaped: FieldNode = {
      kind: 'list',
      list: { ...list, columns: ['weight', 'name'] },
    };
    const sections = buildSections(
      { inventory: { kind: 'group', children: { items: shaped } } },
      {
        [LIST_PATH]: [LIST_RECORD_A],
      },
    );

    expect(columnNames(sections)).toEqual([
      'Weight',
      'Name',
      'Quantity',
      'Active',
      'Pick',
      'Derived total',
      'Scaled',
    ]);
  });

  it('Edge: no declared columns draws the shape in its declared order', () => {
    const fields = listFields();
    const list = listNode(fields);
    const shaped: FieldNode = { kind: 'list', list: { ...list, columns: undefined } };
    const sections = buildSections(
      { inventory: { kind: 'group', children: { items: shaped } } },
      {
        [LIST_PATH]: [LIST_RECORD_A],
      },
    );

    expect(columnNames(sections)).toEqual([
      'Name',
      'Quantity',
      'Weight',
      'Active',
      'Pick',
      'Derived total',
      'Scaled',
    ]);
  });

  it('Edge: an unknown or repeated column is skipped', () => {
    const fields = listFields();
    const list = listNode(fields);
    const shaped: FieldNode = {
      kind: 'list',
      list: { ...list, columns: ['ghost', 'quantity', 'quantity'] },
    };
    const sections = buildSections(
      { inventory: { kind: 'group', children: { items: shaped } } },
      {
        [LIST_PATH]: [LIST_RECORD_A],
      },
    );

    expect(columnNames(sections)).toEqual([
      'Quantity',
      'Name',
      'Weight',
      'Active',
      'Pick',
      'Derived total',
      'Scaled',
    ]);
  });

  it('Edge: an empty list keeps its section and its add target but draws no row', () => {
    const sections = buildSections(listFields(), { [LIST_PATH]: [] });
    const section = sections.find((entry) => entry.key === 'inventory');

    expect(section?.rows).toEqual([]);
    expect(section?.lists.map((list) => list.path)).toEqual([LIST_PATH]);
    expect(section?.lists[0]?.capacity).toBe(LIST_CAPACITY);
  });

  it('Edge: a malformed id or an id of an unknown list is ignored', () => {
    const sections = buildSections(listFields(), {
      [LIST_PATH]: ['not-a-record', LIST_RECORD_A],
      'nope.missing': [LIST_RECORD_B],
    });
    const section = sections.find((entry) => entry.key === 'inventory');

    expect(section?.rows.map(rowKey)).toEqual([`${LIST_PATH}.${LIST_RECORD_A}`]);
  });

  it('Edge: without a record source a list draws no row but still owns its section', () => {
    const sections = buildSections(listFields());
    const section = sections.find((entry) => entry.key === 'inventory');

    expect(section?.rows).toEqual([]);
    expect(section?.lists).toHaveLength(1);
  });
});
