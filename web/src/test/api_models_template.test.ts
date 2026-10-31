import { describe, expect, it } from 'vitest';
import { parseTemplateCatalog, parseTemplateDetail } from '../api/models/template';
import { DefensiveParseFailure } from '../api/errors';

const leaf = {
  schema: {
    type: 'number',
    label: 'Strength',
    is_editable: true,
    default: 10,
  },
};

const derivedLeaf = {
  schema: {
    type: 'number',
    label: 'Armor class',
    formula: '10 + dexterityMod',
    depends_on: [['modifiers', 'dexterityMod']],
    is_editable: false,
    default: 0,
    column_label: 'AC',
  },
};

const group = {
  children: {
    strength: leaf,
    armorClass: derivedLeaf,
  },
  label: 'Stats',
  columns: ['strength', 'armorClass'],
};

function template(overrides: Record<string, unknown> = {}) {
  return {
    id: 'fantasy_v1',
    name: 'Fantasy RPG',
    default: true,
    fields: {
      identity: {
        children: {
          characterName: {
            schema: { type: 'string', label: 'Name', is_editable: true, default: 0 },
          },
        },
      },
      stats: group,
    },
    ...overrides,
  };
}

describe('template models', () => {
  it('Happy path: decodes identifier, name, default flag and the field tree (API-08)', () => {
    const decoded = parseTemplateDetail(template());

    expect(decoded.id).toBe('fantasy_v1');
    expect(decoded.name).toBe('Fantasy RPG');
    expect(decoded.default).toBe(true);
    expect(Object.keys(decoded.fields)).toEqual(['identity', 'stats']);
  });

  it('Happy path: preserves the payload order of the top-level fields (API-08)', () => {
    const decoded = parseTemplateDetail(
      template({ fields: { stats: group, identity: { children: {} } } }),
    );

    expect(Object.keys(decoded.fields)).toEqual(['stats', 'identity']);
  });

  it('Happy path: a node carrying `schema` is a leaf and a node carrying `children` is a group (API-08)', () => {
    const decoded = parseTemplateDetail(template());

    expect(decoded.fields['stats']?.kind).toBe('group');
    const stats = decoded.fields['stats'];
    expect(stats?.kind === 'group' && Object.keys(stats.children)).toEqual([
      'strength',
      'armorClass',
    ]);
  });

  it('Happy path: a leaf exposes its schema, including type, label and editability (API-09)', () => {
    const decoded = parseTemplateDetail(template());
    const stats = decoded.fields['stats'];

    expect(stats?.kind === 'group' && stats.children['strength']?.kind).toBe('leaf');
    const strength = stats?.kind === 'group' ? stats.children['strength'] : undefined;
    expect(strength?.kind === 'leaf' && strength.schema.type).toBe('number');
    expect(strength?.kind === 'leaf' && strength.schema.label).toBe('Strength');
    expect(strength?.kind === 'leaf' && strength.schema.isEditable).toBe(true);
    expect(strength?.kind === 'leaf' && strength.schema.defaultValue).toBe(10);
  });

  it('Happy path: a leaf formula and its dependencies are carried through (API-09)', () => {
    const decoded = parseTemplateDetail(template());
    const stats = decoded.fields['stats'];
    const armorClass = stats?.kind === 'group' ? stats.children['armorClass'] : undefined;

    expect(armorClass?.kind === 'leaf' && armorClass.schema.formula).toBe('10 + dexterityMod');
    expect(armorClass?.kind === 'leaf' && armorClass.schema.dependsOn).toEqual([
      ['modifiers', 'dexterityMod'],
    ]);
    expect(armorClass?.kind === 'leaf' && armorClass.schema.isEditable).toBe(false);
    expect(armorClass?.kind === 'leaf' && armorClass.schema.columnLabel).toBe('AC');
  });

  it('Edge: an omitted formula and dependencies are absent, never an empty string or an empty list (API-09)', () => {
    const decoded = parseTemplateDetail(template());
    const stats = decoded.fields['stats'];
    const strength = stats?.kind === 'group' ? stats.children['strength'] : undefined;

    expect(strength?.kind === 'leaf' && strength.schema.formula).toBeUndefined();
    expect(strength?.kind === 'leaf' && strength.schema.dependsOn).toBeUndefined();
    expect(strength?.kind === 'leaf' && strength.schema.dependsOn).not.toEqual([]);
  });

  it('Edge: the optional exclusive-group and option metadata are carried through when present (API-09)', () => {
    const decoded = parseTemplateDetail(
      template({
        fields: {
          config: {
            children: {
              acUsesDex: {
                schema: {
                  type: 'boolean',
                  label: 'Use dexterity',
                  is_editable: true,
                  default: 0,
                  exclusive_group: 'acSource',
                  option_label: 'Dexterity',
                },
              },
            },
          },
        },
      }),
    );
    const config = decoded.fields['config'];
    const node = config?.kind === 'group' ? config.children['acUsesDex'] : undefined;

    expect(node?.kind === 'leaf' && node.schema.exclusiveGroup).toBe('acSource');
    expect(node?.kind === 'leaf' && node.schema.optionLabel).toBe('Dexterity');
  });

  it('Edge: group table metadata is carried when the payload declares it (API-08)', () => {
    const decoded = parseTemplateDetail(template());
    const stats = decoded.fields['stats'];

    expect(stats?.kind === 'group' && stats.label).toBe('Stats');
    expect(stats?.kind === 'group' && stats.columns).toEqual(['strength', 'armorClass']);
  });

  it('Edge: `default: false` is read as a value, not treated as absence (API-08)', () => {
    const decoded = parseTemplateDetail(template({ default: false }));

    expect(decoded.default).toBe(false);
  });

  it('Happy path: suggested pins are decoded in the order the payload declares (ADR 0002)', () => {
    const decoded = parseTemplateDetail(
      template({ suggested_pins: ['combat.hitPoints', 'derived.armorClass', 'combat.speed'] }),
    );

    expect(decoded.suggestedPins).toEqual([
      'combat.hitPoints',
      'derived.armorClass',
      'combat.speed',
    ]);
  });

  it('Edge: omitted or null suggested pins are absent, never coerced to a list (ADR 0002)', () => {
    expect(parseTemplateDetail(template()).suggestedPins).toBeUndefined();
    expect(parseTemplateDetail(template({ suggested_pins: null })).suggestedPins).toBeUndefined();
  });

  it('Edge: an empty suggested-pins list is decoded as empty, not dropped (ADR 0002)', () => {
    expect(parseTemplateDetail(template({ suggested_pins: [] })).suggestedPins).toEqual([]);
  });

  it('Edge: the catalog decodes suggested pins per entry (ADR 0002)', () => {
    const decoded = parseTemplateCatalog([
      template({ suggested_pins: ['stats.strength'] }),
      template({ id: 'scifi_v1' }),
    ]);

    expect(decoded[0]?.suggestedPins).toEqual(['stats.strength']);
    expect(decoded[1]?.suggestedPins).toBeUndefined();
  });

  it('Error: suggested pins of the wrong shape are contract drift (ADR 0002)', () => {
    const broken: unknown[] = ['combat.hitPoints', 7, {}, true, [1], ['ok', null], [['nested']]];

    for (const suggested of broken) {
      expect(() => parseTemplateDetail(template({ suggested_pins: suggested }))).toThrow(
        DefensiveParseFailure,
      );
    }
  });

  it('Error: a node with neither a schema nor children is contract drift (API-08)', () => {
    expect(() =>
      parseTemplateDetail(template({ fields: { broken: { label: 'Broken' } } })),
    ).toThrow(DefensiveParseFailure);
  });

  it('Error: a missing identifier, name, default flag or fields is rejected by name (API-08)', () => {
    const broken = [
      template({ id: undefined }),
      template({ name: undefined }),
      template({ default: undefined }),
      template({ fields: undefined }),
      template({ fields: [] }),
    ];

    for (const payload of broken) {
      expect(() => parseTemplateDetail(payload)).toThrow(DefensiveParseFailure);
    }
  });

  it('Error: a non-object payload is rejected rather than coerced (API-08)', () => {
    for (const payload of [null, undefined, 'fantasy', 42, []]) {
      expect(() => parseTemplateDetail(payload)).toThrow(DefensiveParseFailure);
    }
  });

  it('Happy path: the catalog decodes every entry with the same shape as the detail (API-08)', () => {
    const decoded = parseTemplateCatalog([
      template(),
      template({ id: 'scifi_v1', default: false }),
    ]);

    expect(decoded).toHaveLength(2);
    expect(decoded.map((entry) => entry.id)).toEqual(['fantasy_v1', 'scifi_v1']);
    expect(decoded[1]?.default).toBe(false);
    expect(Object.keys(decoded[0]?.fields ?? {})).toEqual(['identity', 'stats']);
  });

  it('Edge: an empty catalog decodes to an empty list, not a failure (API-08)', () => {
    expect(parseTemplateCatalog([])).toEqual([]);
  });

  it('Error: a catalog whose entries are not objects is rejected (API-08)', () => {
    expect(() => parseTemplateCatalog(['fantasy_v1'])).toThrow(DefensiveParseFailure);
  });

  it('Error: a catalog payload that is not an array is rejected (API-08)', () => {
    expect(() => parseTemplateCatalog(template())).toThrow(DefensiveParseFailure);
  });
});
