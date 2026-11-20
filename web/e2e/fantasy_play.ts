export const TEMPLATE_ID = 'fantasy_v1';
export const SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1d';

const ABILITIES = [
  { key: 'strength', label: 'Strength' },
  { key: 'dexterity', label: 'Dexterity' },
  { key: 'constitution', label: 'Constitution' },
  { key: 'intelligence', label: 'Intelligence' },
  { key: 'wisdom', label: 'Wisdom' },
  { key: 'charisma', label: 'Charisma' },
];

const SKILLS = [
  'athletics',
  'acrobatics',
  'stealth',
  'perception',
  'insight',
  'survival',
  'persuasion',
  'deception',
  'intimidation',
  'knowledge',
  'analysis',
  'craft',
];

const capitalized = (text: string) => text.charAt(0).toUpperCase() + text.slice(1);

const numberField = (label: string, extra: Record<string, unknown> = {}) => ({
  schema: { type: 'number', label, is_editable: true, default: 0, ...extra },
});

const derivedField = (label: string) => ({
  schema: { type: 'number', label, is_editable: false, default: 0 },
});

const textField = (label: string) => ({
  schema: { type: 'text', label, is_editable: true, default: 0 },
});

const FIELDS = {
  identity: {
    label: 'Identity',
    children: {
      characterName: textField('Character name'),
      playerName: textField('Player name'),
      ancestry: textField('Ancestry'),
      origin: textField('Origin'),
      level: numberField('Level', { min: 1, max: 20 }),
      xp: numberField('Experience points', { min: 0, max: 355000 }),
      proficiencyBonus: numberField('Proficiency bonus', { min: 2, max: 6 }),
    },
  },
  stats: {
    label: 'Stats',
    children: Object.fromEntries(ABILITIES.map((a) => [a.key, numberField(a.label)])),
  },
  modifiers: {
    label: 'Modifiers',
    children: Object.fromEntries(
      ABILITIES.map((a) => [`${a.key}Mod`, derivedField(`${a.label} modifier`)]),
    ),
  },
  saves: {
    label: 'Saves',
    children: Object.fromEntries(ABILITIES.map((a) => [a.key, derivedField(`${a.label} save`)])),
  },
  skills: {
    label: 'Skills',
    children: Object.fromEntries(
      SKILLS.map((key) => [
        key,
        {
          children: {
            bonus: numberField(`${capitalized(key)} bonus`, {
              min: -10,
              max: 20,
              column_label: 'Bonus',
            }),
            total: {
              schema: {
                ...derivedField(`${capitalized(key)} total`).schema,
                column_label: 'Total',
              },
            },
          },
          columns: ['bonus', 'total'],
        },
      ]),
    ),
  },
  attacks: {
    label: 'Attacks',
    children: Object.fromEntries(
      [1, 2, 3, 4].map((slot) => [
        `attack${slot}`,
        {
          label: `Attack ${slot}`,
          children: {
            name: textField(`Attack ${slot} name`),
            bonus: numberField(`Attack ${slot} bonus`, { min: -5, max: 20 }),
            damage: textField(`Attack ${slot} damage`),
          },
          columns: ['name', 'damage', 'bonus'],
        },
      ]),
    ),
  },
  spells: {
    label: 'Spells',
    children: {
      attackBonus: derivedField('Spell attack bonus'),
      saveDC: derivedField('Spell save DC'),
      atWill: textField('At-will spells'),
      list: textField('Spell list'),
      ...Object.fromEntries(
        [1, 2, 3, 4, 5, 6, 7, 8, 9].map((level) => [
          `slot${level}`,
          {
            label: `Level ${level}`,
            children: {
              total: numberField(`Level ${level} total`, {
                min: 0,
                max: 10,
                column_label: 'Total',
              }),
              expended: numberField(`Level ${level} expended`, {
                min: 0,
                max: 10,
                column_label: 'Expended',
              }),
              remaining: derivedField(`Level ${level} remaining`),
            },
            columns: ['total', 'expended', 'remaining'],
          },
        ]),
      ),
    },
  },
  derived: {
    label: 'Derived',
    children: {
      armorClass: derivedField('Armor class'),
      passiveAwareness: derivedField('Passive awareness'),
    },
  },
  combat: {
    label: 'Combat',
    children: {
      baseArmorClass: numberField('Base armor class', { min: 1, max: 30 }),
      initiative: numberField('Initiative', { min: -5, max: 20 }),
      speed: numberField('Speed', { min: 0, max: 120 }),
      hitPoints: numberField('Current hit points', { min: 0, max: 500 }),
      hitPointsMax: numberField('Hit points (maximum)', { min: 1, max: 500 }),
      hitPointsTemp: numberField('Temporary hit points', { min: 0, max: 100 }),
    },
  },
  config: {
    label: 'Template settings',
    children: {
      acUsesDex: numberField('Armor class uses dexterity', {
        min: 0,
        max: 1,
        exclusive_group: 'armorClassAbility',
        option_label: 'Dexterity',
      }),
      acUsesCon: numberField('Armor class uses constitution', {
        min: 0,
        max: 1,
        exclusive_group: 'armorClassAbility',
        option_label: 'Constitution',
      }),
      ...Object.fromEntries(
        ABILITIES.map((a) => [
          `save${capitalized(a.key)}Prof`,
          numberField(`${a.label} save proficiency`, { min: 0, max: 1 }),
        ]),
      ),
      ...Object.fromEntries(
        ABILITIES.map((a) => [
          `spellUses${capitalized(a.key)}`,
          numberField(`Spellcasting uses ${a.label.toLowerCase()}`, {
            min: 0,
            max: 1,
            exclusive_group: 'spellcastingAbility',
            option_label: a.label,
          }),
        ]),
      ),
      ...Object.fromEntries(
        SKILLS.map((key) => [
          `skill${capitalized(key)}Prof`,
          numberField(`${capitalized(key)} proficiency`, { min: 0, max: 1 }),
        ]),
      ),
    },
  },
  rules: {
    label: 'Table rules',
    children: {
      modifierBase: numberField('Ability modifier base'),
      modifierStep: numberField('Ability modifier step'),
      passiveBase: numberField('Passive score base'),
      spellSaveBase: numberField('Spell save DC base'),
    },
  },
};

export const TEMPLATE = {
  id: TEMPLATE_ID,
  name: 'Fantasy',
  default: true,
  fields: FIELDS,
  dice: {
    section: { skills: '1d20 + @.total', saves: '1d20 + @', attacks: '1d20 + @.bonus' },
    row: {
      'combat.initiative': '1d20 + @',
      ...Object.fromEntries(
        ABILITIES.map((a) => [`stats.${a.key}`, `1d20 + modifiers.${a.key}Mod`]),
      ),
    },
  },
};

export const SHEET = {
  id: SHEET_ID,
  template_id: TEMPLATE_ID,
  name: 'Aragorn',
  kind: 'character',
  data: {
    identity: {
      characterName: 'Aria',
      playerName: 'Sam',
      ancestry: 'Elf',
      origin: 'Wanderer',
      level: 3,
      proficiencyBonus: 2,
      xp: 1400,
    },
    stats: {
      strength: 16,
      dexterity: 14,
      constitution: 12,
      intelligence: 10,
      wisdom: 8,
      charisma: 13,
    },
    modifiers: {
      strengthMod: 3,
      dexterityMod: 2,
      constitutionMod: 1,
      intelligenceMod: 0,
      wisdomMod: -1,
      charismaMod: 1,
    },
    saves: {
      strength: 3,
      dexterity: 4,
      constitution: 1,
      intelligence: 0,
      wisdom: -1,
      charisma: 1,
    },
    skills: Object.fromEntries(SKILLS.map((key, index) => [key, { bonus: 0, total: index - 1 }])),
    attacks: {
      attack1: { name: 'Longbow', bonus: 6, damage: '1d8+3' },
      attack2: { name: 'Dagger', bonus: 5, damage: '1d4+3' },
    },
    spells: {
      attackBonus: 6,
      saveDC: 14,
      atWill: 'Light, Mage Hand',
      list: 'Shield, Magic Missile',
      slot1: { total: 4, expended: 1, remaining: 3 },
      slot2: { total: 3, expended: 0, remaining: 3 },
    },
    derived: { armorClass: 12, passiveAwareness: 14 },
    combat: {
      baseArmorClass: 10,
      initiative: 3,
      speed: 30,
      hitPoints: 10,
      hitPointsMax: 20,
      hitPointsTemp: 0,
    },
    config: {
      ...Object.fromEntries(ABILITIES.map((a) => [`save${capitalized(a.key)}Prof`, 0])),
      ...Object.fromEntries(ABILITIES.map((a) => [`spellUses${capitalized(a.key)}`, 0])),
      ...Object.fromEntries(SKILLS.map((key) => [`skill${capitalized(key)}Prof`, 0])),
      acUsesDex: 1,
      acUsesCon: 0,
      saveDexterityProf: 1,
      skillAthleticsProf: 1,
      spellUsesWisdom: 1,
    },
    rules: { modifierBase: 10, modifierStep: 2, passiveBase: 10, spellSaveBase: 8 },
  },
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};
