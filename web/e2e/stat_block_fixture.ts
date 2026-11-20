const ABILITIES = [
  { key: 'strength', label: 'Strength' },
  { key: 'dexterity', label: 'Dexterity' },
  { key: 'constitution', label: 'Constitution' },
  { key: 'intelligence', label: 'Intelligence' },
  { key: 'wisdom', label: 'Wisdom' },
  { key: 'charisma', label: 'Charisma' },
] as const;

const SKILLS = ['athletics', 'acrobatics', 'stealth', 'perception', 'insight', 'survival'] as const;

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function numberField(label: string, editable: boolean, extra: Record<string, unknown> = {}) {
  return { schema: { type: 'number', label, is_editable: editable, default: 0, ...extra } };
}

function textField(label: string, maxLength = 40) {
  return {
    schema: { type: 'text', label, is_editable: true, default: 0, max_length: maxLength },
  };
}

export const STAT_BLOCK_TEMPLATE_ID = 'fantasy_v1';

export const STAT_BLOCK_SHEET_ID = '01a14236-21c9-7546-945e-40c46d1fcd1b';

function savesFields(): Record<string, unknown> {
  return Object.fromEntries(
    ABILITIES.map((ability) => [ability.key, numberField(`${ability.label} save`, false)]),
  );
}

function skillsFields(): Record<string, unknown> {
  return Object.fromEntries(
    SKILLS.map((skill) => [
      skill,
      {
        columns: ['bonus', 'total'],
        children: {
          bonus: numberField(`${capitalize(skill)} bonus`, true),
          total: numberField(`${capitalize(skill)} total`, false),
        },
      },
    ]),
  );
}

function configFields(): Record<string, unknown> {
  return Object.fromEntries([
    ...ABILITIES.map((ability) => [
      `save${capitalize(ability.key)}Prof`,
      numberField(`${ability.label} save proficiency`, true, { min: 0, max: 1 }),
    ]),
    ...SKILLS.map((skill) => [
      `skill${capitalize(skill)}Prof`,
      numberField(`${capitalize(skill)} proficiency`, true, { min: 0, max: 1 }),
    ]),
    [
      'spellUsesWisdom',
      numberField('Spellcasting uses wisdom', true, {
        exclusive_group: 'spellcastingAbility',
        option_label: 'Wisdom',
      }),
    ],
    [
      'spellUsesIntelligence',
      numberField('Spellcasting uses intelligence', true, {
        exclusive_group: 'spellcastingAbility',
        option_label: 'Intelligence',
      }),
    ],
  ]);
}

function attackFields(slot: number): Record<string, unknown> {
  return {
    label: `Attack ${slot}`,
    columns: ['name', 'damage', 'bonus'],
    children: {
      name: {
        schema: {
          type: 'text',
          label: `Attack ${slot} name`,
          is_editable: true,
          default: 0,
          max_length: 40,
          column_label: 'Name',
        },
      },
      damage: {
        schema: {
          type: 'text',
          label: `Attack ${slot} damage`,
          is_editable: true,
          default: 0,
          max_length: 60,
          column_label: 'Damage',
        },
      },
      bonus: numberField(`Attack ${slot} bonus`, true, {
        min: -5,
        max: 20,
        column_label: 'Bonus',
      }),
    },
  };
}

function slotFields(level: number): Record<string, unknown> {
  return {
    label: `Level ${level}`,
    columns: ['total', 'expended', 'remaining'],
    children: {
      total: numberField(`Level ${level} total`, true, { column_label: 'Total' }),
      expended: numberField(`Level ${level} expended`, true, { column_label: 'Expended' }),
      remaining: numberField(`Level ${level} remaining`, false, {
        formula: `spells.slot${level}.total - spells.slot${level}.expended`,
        column_label: 'Remaining',
      }),
    },
  };
}

export const STAT_BLOCK_DETAIL = {
  id: STAT_BLOCK_TEMPLATE_ID,
  name: 'Fantasy',
  default: true,
  fields: {
    identity: {
      label: 'Identity',
      children: {
        characterName: textField('Character name', 60),
        ancestry: textField('Ancestry'),
        origin: textField('Origin'),
        level: numberField('Level', true, { default: 1, min: 1, max: 20 }),
      },
    },
    stats: {
      label: 'Stats',
      children: Object.fromEntries(
        ABILITIES.map((ability) => [
          ability.key,
          numberField(ability.label, true, { default: 10, min: 3, max: 20 }),
        ]),
      ),
    },
    modifiers: {
      label: 'Modifiers',
      children: Object.fromEntries(
        ABILITIES.map((ability) => [
          `${ability.key}Mod`,
          numberField(`${ability.label} modifier`, false),
        ]),
      ),
    },
    derived: {
      label: 'Derived',
      children: {
        armorClass: numberField('Armor class', false),
        passiveAwareness: numberField('Passive awareness', false),
      },
    },
    config: { label: 'Template settings', children: configFields() },
    combat: {
      label: 'Combat',
      children: {
        baseArmorClass: numberField('Base armor class', true, { default: 10, min: 1, max: 30 }),
        initiative: numberField('Initiative', true, { min: -5, max: 20 }),
        speed: numberField('Speed', true, { default: 30, min: 0, max: 120 }),
        hitPoints: numberField('Current hit points', true, { default: 10, max: 500 }),
        hitPointsMax: numberField('Hit points (maximum)', true, { default: 10, min: 1, max: 500 }),
        hitPointsTemp: numberField('Temporary hit points', true, { max: 100 }),
      },
    },
    saves: { label: 'Saves', children: savesFields() },
    skills: { label: 'Skills', children: skillsFields() },
    attacks: {
      label: 'Attacks',
      children: { attack1: attackFields(1), attack2: attackFields(2) },
    },
    spells: {
      label: 'Spells',
      children: {
        attackBonus: numberField('Spell attack bonus', false),
        saveDC: numberField('Spell save DC', false),
        slot1: slotFields(1),
        atWill: textField('At-will spells', 200),
        list: textField('Spell list', 200),
      },
    },
  },
};

const STAT_BLOCK_SKILLS = {
  athletics: { bonus: 0, total: 5 },
  acrobatics: { bonus: 0, total: 2 },
  stealth: { bonus: 1, total: 3 },
  perception: { bonus: 2, total: 5 },
  insight: { bonus: 0, total: -1 },
  survival: { bonus: 0, total: -1 },
};

export const STAT_BLOCK_SHEET = {
  id: STAT_BLOCK_SHEET_ID,
  template_id: STAT_BLOCK_TEMPLATE_ID,
  name: 'Ashclaw',
  kind: 'enemy',
  data: {
    identity: { characterName: 'Ashclaw', ancestry: 'Fiend', origin: 'Ravager', level: 4 },
    stats: {
      strength: 17,
      dexterity: 14,
      constitution: 15,
      intelligence: 8,
      wisdom: 12,
      charisma: 10,
    },
    modifiers: {
      strengthMod: 3,
      dexterityMod: 2,
      constitutionMod: 2,
      intelligenceMod: -1,
      wisdomMod: 1,
      charismaMod: 0,
    },
    derived: { armorClass: 15, passiveAwareness: 13 },
    config: {
      saveStrengthProf: 1,
      saveDexterityProf: 1,
      saveConstitutionProf: 0,
      saveIntelligenceProf: 0,
      saveWisdomProf: 1,
      saveCharismaProf: 0,
      skillAthleticsProf: 1,
      skillAcrobaticsProf: 0,
      skillStealthProf: 1,
      skillPerceptionProf: 1,
      skillInsightProf: 0,
      skillSurvivalProf: 0,
      spellUsesWisdom: 1,
      spellUsesIntelligence: 0,
      acUsesDex: 1,
      acUsesCon: 0,
    },
    combat: {
      baseArmorClass: 13,
      initiative: 2,
      speed: 30,
      hitPoints: 26,
      hitPointsMax: 38,
      hitPointsTemp: 0,
    },
    saves: { strength: 6, dexterity: 5, constitution: 5, intelligence: 2, wisdom: 4, charisma: 3 },
    skills: STAT_BLOCK_SKILLS,
    attacks: {
      attack1: { name: 'Claw', damage: '2d6 + 3', bonus: 6 },
      attack2: { name: 'Bite', damage: '1d8 + 3', bonus: 6 },
    },
    spells: {
      attackBonus: 5,
      saveDC: 13,
      slot1: { total: 3, expended: 1, remaining: 2 },
      atWill: 'Minor illusion',
      list: 'Fear, Darkness',
    },
  },
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};
