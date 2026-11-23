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
    kinds: ['character', 'npc', 'enemy'],
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
  it('Happy path: decodes identifier, name, default flag and the field tree', () => {
    const decoded = parseTemplateDetail(template());

    expect(decoded.id).toBe('fantasy_v1');
    expect(decoded.name).toBe('Fantasy RPG');
    expect(decoded.default).toBe(true);
    expect(Object.keys(decoded.fields)).toEqual(['identity', 'stats']);
  });

  it('Happy path: decodes the accepted kinds in their declared order', () => {
    const decoded = parseTemplateDetail(template({ kinds: ['npc', 'enemy', 'character'] }));

    expect(decoded.kinds).toEqual(['npc', 'enemy', 'character']);
  });

  it('Edge: a single-kind template exposes exactly that kind', () => {
    const decoded = parseTemplateDetail(template({ kinds: ['enemy'] }));

    expect(decoded.kinds).toEqual(['enemy']);
  });

  it('Edge: a template without character offers the remaining kinds only', () => {
    const decoded = parseTemplateDetail(template({ kinds: ['npc', 'enemy'] }));

    expect(decoded.kinds).toEqual(['npc', 'enemy']);
    expect(decoded.kinds).not.toContain('character');
  });

  it('Edge: a missing kinds key decodes as no accepted kinds', () => {
    const decoded = parseTemplateDetail(template({ kinds: undefined }));

    expect(decoded.kinds).toEqual([]);
  });

  it('Edge: an empty kinds list decodes as no accepted kinds', () => {
    const decoded = parseTemplateDetail(template({ kinds: [] }));

    expect(decoded.kinds).toEqual([]);
  });

  it('Edge: duplicate kinds keep the declared order', () => {
    const decoded = parseTemplateDetail(template({ kinds: ['npc', 'npc', 'enemy'] }));

    expect(decoded.kinds).toEqual(['npc', 'npc', 'enemy']);
  });

  it('Error: non-list, null, non-string or unknown kinds are contract drift', () => {
    const broken: unknown[] = [null, 'character', ['character', 'dragon'], ['character', 7], [7]];

    for (const kinds of broken) {
      expect(() => parseTemplateDetail(template({ kinds }))).toThrow(DefensiveParseFailure);
    }
  });

  it('Happy path: preserves the payload order of the top-level fields', () => {
    const decoded = parseTemplateDetail(
      template({ fields: { stats: group, identity: { children: {} } } }),
    );

    expect(Object.keys(decoded.fields)).toEqual(['stats', 'identity']);
  });

  it('Happy path: a node carrying `schema` is a leaf and a node carrying `children` is a group', () => {
    const decoded = parseTemplateDetail(template());

    expect(decoded.fields['stats']?.kind).toBe('group');
    const stats = decoded.fields['stats'];
    expect(stats?.kind === 'group' && Object.keys(stats.children)).toEqual([
      'strength',
      'armorClass',
    ]);
  });

  it('Happy path: a leaf exposes its schema, including type, label and editability', () => {
    const decoded = parseTemplateDetail(template());
    const stats = decoded.fields['stats'];

    expect(stats?.kind === 'group' && stats.children['strength']?.kind).toBe('leaf');
    const strength = stats?.kind === 'group' ? stats.children['strength'] : undefined;
    expect(strength?.kind === 'leaf' && strength.schema.type).toBe('number');
    expect(strength?.kind === 'leaf' && strength.schema.label).toBe('Strength');
    expect(strength?.kind === 'leaf' && strength.schema.isEditable).toBe(true);
    expect(strength?.kind === 'leaf' && strength.schema.defaultValue).toBe(10);
  });

  it('Happy path: a leaf formula and its dependencies are carried through', () => {
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

  it('Edge: an omitted formula and dependencies are absent, never an empty string or an empty list', () => {
    const decoded = parseTemplateDetail(template());
    const stats = decoded.fields['stats'];
    const strength = stats?.kind === 'group' ? stats.children['strength'] : undefined;

    expect(strength?.kind === 'leaf' && strength.schema.formula).toBeUndefined();
    expect(strength?.kind === 'leaf' && strength.schema.dependsOn).toBeUndefined();
    expect(strength?.kind === 'leaf' && strength.schema.dependsOn).not.toEqual([]);
  });

  it('Edge: the optional exclusive-group and option metadata are carried through when present', () => {
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

  it('Edge: the optional flag marker is carried through when present and absent otherwise', () => {
    const decoded = parseTemplateDetail(
      template({
        fields: {
          features: {
            children: {
              enabled: {
                schema: {
                  type: 'number',
                  label: 'Enabled',
                  is_editable: true,
                  default: 1,
                  flag: true,
                },
              },
            },
          },
        },
      }),
    );
    const features = decoded.fields['features'];
    const enabled = features?.kind === 'group' ? features.children['enabled'] : undefined;
    const stats = parseTemplateDetail(template()).fields['stats'];
    const strength = stats?.kind === 'group' ? stats.children['strength'] : undefined;

    expect(enabled?.kind === 'leaf' && enabled.schema.flag).toBe(true);
    expect(strength?.kind === 'leaf' && strength.schema.flag).toBeUndefined();
  });

  it('Edge: group table metadata is carried when the payload declares it', () => {
    const decoded = parseTemplateDetail(template());
    const stats = decoded.fields['stats'];

    expect(stats?.kind === 'group' && stats.label).toBe('Stats');
    expect(stats?.kind === 'group' && stats.columns).toEqual(['strength', 'armorClass']);
  });

  it('Edge: `default: false` is read as a value, not treated as absence', () => {
    const decoded = parseTemplateDetail(template({ default: false }));

    expect(decoded.default).toBe(false);
  });

  it('Edge: a leftover suggested_pins key of any shape is ignored, never drift', () => {
    const leftovers: unknown[] = [['combat.hitPoints'], [], null, 7, { a: 1 }];

    for (const leftover of leftovers) {
      const decoded = parseTemplateDetail(template({ suggested_pins: leftover }));
      expect(decoded).not.toHaveProperty('suggestedPins');
    }
  });

  it('Error: a node carrying none of schema, children or list is contract drift', () => {
    expect(() =>
      parseTemplateDetail(template({ fields: { broken: { label: 'Broken' } } })),
    ).toThrow(DefensiveParseFailure);
  });

  it('Happy path: a list node carries its label, columns, capacity, shape, dice and marking', () => {
    const decoded = parseTemplateDetail(
      template({
        fields: {
          inventory: {
            children: {
              items: {
                list: {
                  label: 'Items',
                  columns: ['name', 'quantity'],
                  capacity: 2,
                  dice: '1d20 + @.quantity',
                  not_d20: true,
                  shape: {
                    name: {
                      schema: { type: 'string', label: 'Name', is_editable: true, default: 0 },
                    },
                    quantity: leaf,
                  },
                },
              },
            },
          },
        },
      }),
    );

    const inventory = decoded.fields['inventory'];
    const items = inventory?.kind === 'group' ? inventory.children['items'] : undefined;
    expect(items?.kind).toBe('list');
    expect(items?.kind === 'list' && items.list.label).toBe('Items');
    expect(items?.kind === 'list' && items.list.capacity).toBe(2);
    expect(items?.kind === 'list' && items.list.columns).toEqual(['name', 'quantity']);
    expect(items?.kind === 'list' && items.list.dice).toBe('1d20 + @.quantity');
    expect(items?.kind === 'list' && items.list.notD20).toBe(true);
    expect(items?.kind === 'list' && Object.keys(items.list.shape)).toEqual(['name', 'quantity']);
  });

  it('Error: a list shape holding anything but leaves is contract drift', () => {
    expect(() =>
      parseTemplateDetail(
        template({
          fields: {
            inventory: {
              children: {
                items: {
                  list: { label: 'Items', capacity: 2, shape: { nested: group } },
                },
              },
            },
          },
        }),
      ),
    ).toThrow(DefensiveParseFailure);
  });

  it('Error: a list without a label or capacity is contract drift', () => {
    for (const list of [
      { capacity: 2, shape: {} },
      { label: 'Items', shape: {} },
    ]) {
      expect(() =>
        parseTemplateDetail(template({ fields: { inventory: { children: { items: { list } } } } })),
      ).toThrow(DefensiveParseFailure);
    }
  });

  it('Error: a missing identifier, name, default flag or fields is rejected by name', () => {
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

  it('Error: a non-object payload is rejected rather than coerced', () => {
    for (const payload of [null, undefined, 'fantasy', 42, []]) {
      expect(() => parseTemplateDetail(payload)).toThrow(DefensiveParseFailure);
    }
  });

  it('Happy path: the catalog decodes every entry with the same shape as the detail', () => {
    const decoded = parseTemplateCatalog([
      template(),
      template({ id: 'scifi_v1', default: false }),
    ]);

    expect(decoded).toHaveLength(2);
    expect(decoded.map((entry) => entry.id)).toEqual(['fantasy_v1', 'scifi_v1']);
    expect(decoded[1]?.default).toBe(false);
    expect(Object.keys(decoded[0]?.fields ?? {})).toEqual(['identity', 'stats']);
  });

  it('Edge: an empty catalog decodes to an empty list, not a failure', () => {
    expect(parseTemplateCatalog([])).toEqual([]);
  });

  it('Error: a catalog whose entries are not objects is rejected', () => {
    expect(() => parseTemplateCatalog(['fantasy_v1'])).toThrow(DefensiveParseFailure);
  });

  it('Error: a catalog payload that is not an array is rejected', () => {
    expect(() => parseTemplateCatalog(template())).toThrow(DefensiveParseFailure);
  });
});

const GAME_SYSTEM = {
  id: 'fifth',
  name: '5E',
  compatibility_label: '5E compatible',
  notice: { label: 'SRD 5.2.1', path: '/srd' },
};

describe('template game system', () => {
  it('Happy path: decodes the id, name, compatibility label and notice', () => {
    const decoded = parseTemplateDetail(template({ game_system: GAME_SYSTEM }));

    expect(decoded.gameSystem).toEqual({
      id: 'fifth',
      name: '5E',
      compatibilityLabel: '5E compatible',
      notice: { label: 'SRD 5.2.1', path: '/srd' },
    });
  });

  it('Happy path: the catalog decodes the game system of each entry', () => {
    const decoded = parseTemplateCatalog([
      template({ id: 'fifth_character_v1', game_system: GAME_SYSTEM }),
      template({ id: 'fantasy_v1' }),
    ]);

    expect(decoded[0]?.gameSystem?.compatibilityLabel).toBe('5E compatible');
    expect(decoded[1]?.gameSystem).toBeUndefined();
  });

  it('Edge: a template without a game system omits it, never an empty object', () => {
    expect(parseTemplateDetail(template()).gameSystem).toBeUndefined();
    expect(parseTemplateDetail(template({ game_system: null })).gameSystem).toBeUndefined();
  });

  it('Edge: a root-relative notice path survives decoding', () => {
    for (const path of ['/', '/srd', '/srd/notice?lang=en#top']) {
      const decoded = parseTemplateDetail(
        template({ game_system: { ...GAME_SYSTEM, notice: { label: 'SRD 5.2.1', path } } }),
      );

      expect(decoded.gameSystem?.notice.path).toBe(path);
    }
  });

  it('Error: a malformed game system payload is contract drift', () => {
    const broken: unknown[] = [
      'fifth',
      [],
      {},
      { name: '5E', compatibility_label: '5E compatible', notice: GAME_SYSTEM.notice },
      { id: 'fifth', compatibility_label: '5E compatible', notice: GAME_SYSTEM.notice },
      { id: 'fifth', name: '5E', notice: GAME_SYSTEM.notice },
      { id: 'fifth', name: '5E', compatibility_label: '5E compatible' },
      { id: 'fifth', name: '5E', compatibility_label: '5E compatible', notice: 'SRD 5.2.1' },
      {
        id: 'fifth',
        name: '5E',
        compatibility_label: '5E compatible',
        notice: { path: '/srd' },
      },
      {
        id: 'fifth',
        name: '5E',
        compatibility_label: '5E compatible',
        notice: { label: 'SRD 5.2.1' },
      },
      { id: 5, name: '5E', compatibility_label: '5E compatible', notice: GAME_SYSTEM.notice },
      { id: 'fifth', name: 5, compatibility_label: '5E compatible', notice: GAME_SYSTEM.notice },
      {
        id: 'fifth',
        name: '5E',
        compatibility_label: 5,
        notice: GAME_SYSTEM.notice,
      },
      {
        id: 'fifth',
        name: '5E',
        compatibility_label: '5E compatible',
        notice: { label: 5, path: '/srd' },
      },
    ];

    for (const gameSystem of broken) {
      expect(() => parseTemplateDetail(template({ game_system: gameSystem }))).toThrow(
        DefensiveParseFailure,
      );
    }
  });

  it('Error: an external or protocol-relative notice path is refused', () => {
    const refused = [
      '',
      'srd',
      '//example.com/srd',
      'http:',
      'https://example.com/srd',
      ['javascript', 'alert(1)'].join(':'),
      String.raw`/\example.com/srd`,
      '/srd\nx',
    ];

    for (const path of refused) {
      expect(() =>
        parseTemplateDetail(
          template({ game_system: { ...GAME_SYSTEM, notice: { label: 'SRD 5.2.1', path } } }),
        ),
      ).toThrow(DefensiveParseFailure);
    }
  });
});

describe('template reset actions', () => {
  const rest = {
    id: 'rest',
    label: 'Long rest',
    assignments: [
      { target: 'stats.level', expression: 'max(1, level - 1)' },
      {
        target: 'inventory.items.quantity',
        expression: 'record_quantity + 1',
        filter: 'active',
      },
    ],
  };

  it('Happy path: actions decode in order with their assignments and list filter', () => {
    const decoded = parseTemplateDetail(template({ actions: [rest] }));

    expect(decoded.actions).toEqual([
      {
        id: 'rest',
        label: 'Long rest',
        assignments: [
          { target: 'stats.level', expression: 'max(1, level - 1)' },
          {
            target: 'inventory.items.quantity',
            expression: 'record_quantity + 1',
            filter: 'active',
          },
        ],
      },
    ]);
  });

  it('Edge: a static assignment carries no filter key', () => {
    const decoded = parseTemplateDetail(
      template({ actions: [{ id: 'rest', label: 'Rest', assignments: [rest.assignments[0]] }] }),
    );

    expect(decoded.actions?.[0]?.assignments[0]).not.toHaveProperty('filter');
  });

  it('Edge: absent actions stay absent and an empty list stays empty', () => {
    expect(parseTemplateDetail(template()).actions).toBeUndefined();
    expect(parseTemplateDetail(template({ actions: [] })).actions).toEqual([]);
  });

  it('Error: malformed actions and assignments are contract drift', () => {
    const broken: unknown[] = [
      'rest',
      {},
      [7],
      [{ label: 'Rest', assignments: [] }],
      [{ id: 'rest', assignments: [] }],
      [{ id: 'rest', label: 'Rest' }],
      [{ id: 'rest', label: 'Rest', assignments: {} }],
      [{ id: 'rest', label: 'Rest', assignments: [7] }],
      [{ id: 'rest', label: 'Rest', assignments: [{ target: 'stats.level' }] }],
      [{ id: 'rest', label: 'Rest', assignments: [{ expression: 'level' }] }],
      [
        {
          id: 'rest',
          label: 'Rest',
          assignments: [{ target: 'stats.level', expression: 'level', filter: 7 }],
        },
      ],
    ];

    for (const actions of broken) {
      expect(() => parseTemplateDetail(template({ actions }))).toThrow(DefensiveParseFailure);
    }
  });
});
