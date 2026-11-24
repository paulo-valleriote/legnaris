import { describe, expect, it } from 'vitest';
import type { FieldNode, FieldSchema, LeafNode, TemplateDetail } from '../api/models/template';
import { templateDerivation } from '../landing/template_derivation';

function leaf(label: string, options: Partial<FieldSchema> = {}): LeafNode {
  return {
    kind: 'leaf',
    schema: {
      type: 'number',
      label,
      isEditable: true,
      defaultValue: 0,
      ...options,
    },
  };
}

function group(children: Record<string, FieldNode>): FieldNode {
  return { kind: 'group', label: 'Group', children };
}

function list(shape: Record<string, LeafNode>): FieldNode {
  return {
    kind: 'list',
    list: { label: 'List', capacity: 5, shape },
  };
}

function template(fields: Record<string, FieldNode>): TemplateDetail {
  return { id: 'synthetic_v1', name: 'Synthetic', kinds: ['character'], default: false, fields };
}

function derive(fields: Record<string, FieldNode>, data: Record<string, unknown>) {
  return templateDerivation(template(fields))(data);
}

describe('the template derivation', () => {
  it('Happy path: it evaluates the formula grammar the 5E template uses', () => {
    const fields = {
      stats: group({
        score: leaf('Score', { defaultValue: 10 }),
        flag: leaf('Flag', { defaultValue: 1 }),
      }),
      derived: group({
        modifier: leaf('Modifier', {
          isEditable: false,
          formula: 'floor((stats_score - 10) / 2)',
          dependsOn: [['stats', 'score']],
        }),
        ladder: leaf('Ladder', {
          isEditable: false,
          formula: 'lookup(stats_score, 1, 10, 5, 20) + max(0, stats_score - 12) * 2',
          dependsOn: [['stats', 'score']],
        }),
        choice: leaf('Choice', {
          isEditable: false,
          formula: 'stats_score > 10 && stats_flag > 0 ? 5 : -(1 + 1)',
          dependsOn: [
            ['stats', 'score'],
            ['stats', 'flag'],
          ],
        }),
        remainder: leaf('Remainder', {
          isEditable: false,
          formula: 'stats_score % 4 - -1',
          dependsOn: [['stats', 'score']],
        }),
      }),
    };

    const values = derive(fields, { stats: { score: 18, flag: 1 } });

    expect(values.get('derived.modifier')).toBe(4);
    expect(values.get('derived.ladder')).toBe(32);
    expect(values.get('derived.choice')).toBe(5);
    expect(values.get('derived.remainder')).toBe(3);
  });

  it('Edge: missing values fall back to the template defaults and a chain resolves through derived leaves', () => {
    const fields = {
      stats: group({ score: leaf('Score', { defaultValue: 12 }) }),
      derived: group({
        modifier: leaf('Modifier', {
          isEditable: false,
          formula: 'floor((stats_score - 10) / 2)',
          dependsOn: [['stats', 'score']],
        }),
        save: leaf('Save', {
          isEditable: false,
          formula: 'derived_modifier + 2',
          dependsOn: [['derived', 'modifier']],
        }),
      }),
    };

    const values = derive(fields, {});

    expect(values.get('derived.modifier')).toBe(1);
    expect(values.get('derived.save')).toBe(3);
  });

  it('Edge: a formula outside the supported grammar leaves the stored value alone', () => {
    const fields = {
      stats: group({
        score: leaf('Score', { defaultValue: 12 }),
        broken: leaf('Broken', {
          isEditable: false,
          formula: 'unknown_function(stats_score)',
          dependsOn: [['stats', 'score']],
        }),
        text: leaf('Text result', {
          isEditable: false,
          formula: 'stats_score > 10',
          dependsOn: [['stats', 'score']],
        }),
      }),
    };

    const values = derive(fields, { stats: { score: 12 } });

    expect(values.has('stats.broken')).toBe(false);
    expect(values.has('stats.text')).toBe(false);
  });

  it('Happy path: a record derivation reads its own shape and the sheet-level derived values', () => {
    const fields = {
      stats: group({ score: leaf('Score', { defaultValue: 16 }) }),
      derived: group({
        modifier: leaf('Modifier', {
          isEditable: false,
          formula: 'floor((stats_score - 10) / 2)',
          dependsOn: [['stats', 'score']],
        }),
      }),
      attacks: group({
        list: list({
          ability: leaf('Ability', { defaultValue: 1 }),
          quantity: leaf('Quantity', { defaultValue: 1 }),
          weight: leaf('Weight', { defaultValue: 0 }),
          lineWeight: leaf('Line weight', {
            isEditable: false,
            formula: 'record_quantity * record_weight',
            dependsOn: [['quantity'], ['weight']],
          }),
          toHit: leaf('To hit', {
            isEditable: false,
            formula: 'record_ability * derived_modifier + 2',
            dependsOn: [['ability'], ['derived', 'modifier']],
          }),
        }),
      }),
    };

    const values = derive(fields, {
      attacks: {
        list: {
          r1: { quantity: 3, weight: 2, toHit: 99 },
          r2: { ability: 0, quantity: 1, weight: 0.5 },
        },
      },
    });

    expect(values.get('attacks.list.r1.lineWeight')).toBe(6);
    expect(values.get('attacks.list.r1.toHit')).toBe(5);
    expect(values.get('attacks.list.r2.lineWeight')).toBe(0.5);
    expect(values.get('attacks.list.r2.toHit')).toBe(2);
  });
});
