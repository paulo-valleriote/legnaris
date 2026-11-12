import { describe, expect, it } from 'vitest';
import type { FieldNode, FieldSchema } from '../api/models/template';
import type { DiceBlock } from '../api/models/dice';
import { diceSchemaFromFields, resolveRowForSheet, templateDiceDefaults } from '../sheet/dice_view';

function leaf(type: string): FieldNode {
  const schema: FieldSchema = { type, label: type, isEditable: true, defaultValue: 0 };
  return { kind: 'leaf', schema };
}

const schema = diceSchemaFromFields({
  attrs: {
    kind: 'group',
    children: { dex: leaf('number'), init: leaf('number'), name: leaf('text') },
  },
});

const rowKeys = new Set(['attrs.init']);
const sectionRows = new Map([['attrs', ['attrs.init']]]);
const templateDefaults = templateDiceDefaults({
  section: { attrs: '1d6' },
  row: { 'attrs.init': '1d20+attrs.dex' },
});

function resolve(block: DiceBlock) {
  return resolveRowForSheet({
    rowKey: 'attrs.init',
    sectionKey: 'attrs',
    schema,
    rowKeys,
    sectionRows,
    defaults: templateDefaults,
    block,
  });
}

describe('resolveRowForSheet', () => {
  it('Happy path: with no player choice the Template Row default applies', () => {
    const resolution = resolve({ section: {}, row: {} });

    expect(resolution.rollable).toBe(true);
    expect(resolution.source).toBe('template-row');
    expect(resolution.expression).toBe('1d20+attrs.dex');
  });

  it('Happy path: a player override for the Row replaces the Template default', () => {
    const resolution = resolve({ section: {}, row: { 'attrs.init': '1d12+attrs.dex' } });

    expect(resolution.source).toBe('sheet-row');
    expect(resolution.expression).toBe('1d12+attrs.dex');
  });

  it('Edge: a stale override for a Row the Template no longer has is ignored, not fatal', () => {
    const resolution = resolve({ section: {}, row: { 'attrs.gone': '1d4' } });

    expect(resolution.rollable).toBe(true);
    expect(resolution.source).toBe('template-row');
  });

  it('Edge: a stale override whose Field was removed falls back to the Template default', () => {
    const resolution = resolve({ section: {}, row: { 'attrs.init': '1d4+attrs.removed' } });

    expect(resolution.source).toBe('template-row');
    expect(resolution.expression).toBe('1d20+attrs.dex');
  });

  it('Edge: a player "no dice" on the Section silences the Template Row default', () => {
    const resolution = resolve({ section: { attrs: null }, row: {} });

    expect(resolution.rollable).toBe(false);
    expect(resolution.source).toBe('sheet-section-off');
  });

  it('Edge: an absent Template dice block resolves to no rollable expression', () => {
    const resolution = resolveRowForSheet({
      rowKey: 'attrs.init',
      sectionKey: 'attrs',
      schema,
      rowKeys,
      sectionRows,
      defaults: templateDiceDefaults(undefined),
      block: { section: {}, row: {} },
    });

    expect(resolution.rollable).toBe(false);
    expect(resolution.source).toBe('none');
  });
});
