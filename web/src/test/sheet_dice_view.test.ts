import { describe, expect, it } from 'vitest';
import type { FieldNode } from '../api/models/template';
import type { FieldSchema } from '../api/models/template';
import {
  checkExpression,
  checkSectionExpression,
  diceSchemaFromFields,
  diceSectionRows,
  pruneStaleChoices,
  diceErrorMessageKey,
  diceSummary,
} from '../sheet/dice_view';
import { parseDice, type Expression } from '../dice/parse';
import { messages } from '../l10n/strings';
import type { DiceErrorClass } from '../dice/errors';

function leaf(type: string, label = type): FieldNode {
  const schema: FieldSchema = {
    type,
    label,
    isEditable: true,
    defaultValue: 0,
  };
  return { kind: 'leaf', schema };
}

const fields: Record<string, FieldNode> = {
  attrs: {
    kind: 'group',
    children: {
      dex: leaf('number'),
      init: leaf('number'),
      name: leaf('text'),
      alive: leaf('bool'),
    },
  },
};

const schema = diceSchemaFromFields(fields);

describe('diceSchemaFromFields', () => {
  it('Happy path: each leaf answers its kind by dotted path', () => {
    expect(schema('attrs.dex')).toBe('number');
    expect(schema('attrs.name')).toBe('text');
    expect(schema('attrs.alive')).toBe('bool');
  });

  it('Edge: a group, an absent path and an inherited name are not Fields', () => {
    expect(schema('attrs')).toBeUndefined();
    expect(schema('attrs.missing')).toBeUndefined();
    expect(schema('attrs.constructor')).toBeUndefined();
  });

  it('Edge: an empty tree answers no Field at all', () => {
    expect(diceSchemaFromFields({})('attrs.dex')).toBeUndefined();
  });
});

describe('checkExpression', () => {
  it('Happy path: a valid expression over a numeric Field is accepted', () => {
    expect(checkExpression('1d20+attrs.dex', schema, 'attrs.init')).toBeUndefined();
  });

  it('Edge: a blank expression is refused as empty', () => {
    expect(checkExpression('   ', schema, 'attrs.init')?.class).toBe('empty');
  });

  it('Edge: a die with zero faces or zero dice is refused by its class', () => {
    expect(checkExpression('1d0', schema, 'attrs.init')?.class).toBe('dice_faces');
    expect(checkExpression('0d6', schema, 'attrs.init')?.class).toBe('dice_count');
  });

  it('Edge: a fractional term is refused as unreadable', () => {
    expect(checkExpression('1d20 + 1.5', schema, 'attrs.init')?.class).toBe('unreadable_term');
  });

  it('Error: a Field that does not exist is refused as unknown_field', () => {
    expect(checkExpression('1d20+attrs.missing', schema, 'attrs.init')?.class).toBe(
      'unknown_field',
    );
  });

  it('Error: a Field that is not numeric is refused as non_numeric_field', () => {
    expect(checkExpression('1d20+attrs.name', schema, 'attrs.init')?.class).toBe(
      'non_numeric_field',
    );
  });
});

describe('pruneStaleChoices', () => {
  const rowKeys = new Set(['attrs.init']);
  const sectionRows = new Map([['attrs', ['attrs.init']]]);

  it('Happy path: a current override is kept as stored', () => {
    const pruned = pruneStaleChoices(
      { row: { 'attrs.init': '1d20+attrs.dex' }, section: { attrs: '1d6' } },
      { rowKeys, sectionRows, schema },
    );

    expect(pruned).toEqual({ row: { 'attrs.init': '1d20+attrs.dex' }, section: { attrs: '1d6' } });
  });

  it('Edge: an override for a Row or Section the Template no longer has is ignored', () => {
    const pruned = pruneStaleChoices(
      { row: { 'attrs.gone': '1d4' }, section: { gone: '1d4' } },
      { rowKeys, sectionRows, schema },
    );

    expect(pruned).toEqual({ row: {}, section: {} });
  });

  it('Edge: an override whose Field was removed no longer fits and is ignored', () => {
    const pruned = pruneStaleChoices(
      { row: { 'attrs.init': '1d20+attrs.removed' }, section: {} },
      { rowKeys, sectionRows, schema },
    );

    expect(pruned.row).toEqual({});
  });

  it('Edge: an explicit "no dice" is a choice and survives pruning', () => {
    const pruned = pruneStaleChoices(
      { row: { 'attrs.init': null }, section: { attrs: null } },
      { rowKeys, sectionRows, schema },
    );

    expect(pruned).toEqual({ row: { 'attrs.init': null }, section: { attrs: null } });
  });

  it('Edge: a Section expression that fits none of its Rows is ignored (ADR 0006)', () => {
    const pruned = pruneStaleChoices(
      { row: {}, section: { attrs: '1d20+attrs.removed' } },
      { rowKeys, sectionRows, schema },
    );

    expect(pruned.section).toEqual({});
  });

  it('Edge: a Section whose Rows are all gone keeps no expression', () => {
    const pruned = pruneStaleChoices(
      { row: {}, section: { attrs: '1d6' } },
      { rowKeys, sectionRows: new Map([['attrs', []]]), schema },
    );

    expect(pruned.section).toEqual({});
  });

  it('Edge: an explicit "no dice" for a Row or Section the Template no longer has is ignored', () => {
    const pruned = pruneStaleChoices(
      { row: { 'attrs.gone': null }, section: { gone: null } },
      { rowKeys, sectionRows, schema },
    );

    expect(pruned).toEqual({ row: {}, section: {} });
  });

  it('Edge: an unparseable expression is ignored, the rest of the block kept', () => {
    const pruned = pruneStaleChoices(
      { row: { 'attrs.init': '1d20++' }, section: { attrs: '1d6' } },
      { rowKeys, sectionRows, schema },
    );

    expect(pruned).toEqual({ row: {}, section: { attrs: '1d6' } });
  });

  it('Edge: an empty block prunes to an empty block', () => {
    expect(pruneStaleChoices({ row: {}, section: {} }, { rowKeys, sectionRows, schema })).toEqual({
      row: {},
      section: {},
    });
  });
});

describe('diceErrorMessageKey', () => {
  const classes: DiceErrorClass[] = [
    'empty',
    'too_long',
    'too_many_terms',
    'dice_count',
    'dice_faces',
    'number_range',
    'missing_term',
    'unreadable_term',
    'unknown_field',
    'non_numeric_field',
    'no_rows',
    'no_value',
    'overflow',
    'no_dice',
    'turned_off',
  ];

  it('Happy path: every error class resolves to a message that exists in the catalogue', () => {
    for (const errorClass of classes) {
      const key = diceErrorMessageKey(errorClass);
      expect(messages[key], errorClass).toBeTypeOf('string');
    }
  });

  it('Edge: a missing or non-numeric value gets its own explicit message, not the generic one', () => {
    expect(diceErrorMessageKey('no_value')).not.toBe(diceErrorMessageKey('unknown_field'));
    expect(diceErrorMessageKey('non_numeric_field')).not.toBe(diceErrorMessageKey('no_value'));
  });
});

function parsed(text: string): Expression {
  const result = parseDice(text);
  if (!result.ok) {
    throw new Error(`fixture "${text}" does not parse: ${result.error.message}`);
  }
  return result.value;
}

describe('diceSummary', () => {
  const tests: { name: string; expression: string; want: string | undefined }[] = [
    {
      name: 'Edge: an expression with no dice has no summary',
      expression: '3+attrs.dex',
      want: undefined,
    },
    { name: 'Edge: a count of one is left implicit', expression: '1d20', want: 'd20' },
    { name: 'Edge: an implicit count stays implicit', expression: 'd8', want: 'd8' },
    {
      name: 'Edge: numbers and Field references are left out',
      expression: '1d20+attrs.dex+2',
      want: 'd20',
    },
    { name: 'Edge: a leading negative die keeps its sign', expression: '-1d4+5', want: '-d4' },
    { name: 'Edge: a later negative die keeps its sign', expression: '2d6-1d4', want: '2d6-d4' },
    {
      name: 'Edge: past two dice terms the rest collapse into an ellipsis',
      expression: '1d20+1d6+1d4',
      want: 'd20+d6+…',
    },
    {
      name: 'Happy path: two dice terms are both named',
      expression: '2d6+1d8+attrs.dex',
      want: '2d6+d8',
    },
  ];
  for (const tc of tests) {
    it(tc.name, () => {
      expect(diceSummary(parsed(tc.expression))).toBe(tc.want);
    });
  }
});

describe('checkSectionExpression', () => {
  const rows = ['attrs.dex', 'attrs.init'];

  it.each<[string, string, readonly string[], DiceErrorClass | undefined]>([
    ['Edge: an unparseable expression is refused', '1d20++', rows, 'missing_term'],
    ['Edge: a Section with no Rows fits nothing', '1d6', [], 'no_rows'],
    [
      'Error: a Field reference no Row can resolve is refused',
      '1d20+attrs.removed',
      rows,
      'unknown_field',
    ],
    ['Error: a non-numeric Field is refused', '1d20+attrs.name', rows, 'non_numeric_field'],
    ['Happy path: a plain expression fits', '1d6', rows, undefined],
    [
      'Happy path: "@" fits when at least one Row is numeric',
      '1d20+@',
      ['attrs.name', 'attrs.init'],
      undefined,
    ],
  ])('%s', (_name, text, sectionRowKeys, expected) => {
    expect(checkSectionExpression(text, schema, sectionRowKeys)?.class).toBe(expected);
  });
});

describe('diceSectionRows', () => {
  it('Edge: no fields yields no Sections', () => {
    expect(diceSectionRows({}).size).toBe(0);
  });

  it('Happy path: each Section lists the keys of its Rows', () => {
    const rows = diceSectionRows(fields).get('attrs');

    expect(rows).toEqual(expect.arrayContaining(['attrs.dex', 'attrs.init']));
  });
});
