import type { Sheet } from '../api/models/sheet';
import type { FieldNode, LeafNode, ResetAction, TemplateDetail } from '../api/models/template';

export const FIFTH_DEMO_TEMPLATE_ID = 'fifth_character_v1';

export const FIFTH_DEMO_SHEET_ID = '00000000-0000-7000-8000-000000000421';

export const FIFTH_DEMO_NOTE = 'An invented character on the 5E Character template.';

const ABILITY_KEYS = [
  'strength',
  'dexterity',
  'constitution',
  'intelligence',
  'wisdom',
  'charisma',
] as const;

type AbilityKey = (typeof ABILITY_KEYS)[number];

const ABILITY_LABELS: Readonly<Record<AbilityKey, string>> = {
  strength: 'Strength',
  dexterity: 'Dexterity',
  constitution: 'Constitution',
  intelligence: 'Intelligence',
  wisdom: 'Wisdom',
  charisma: 'Charisma',
};

const SKILL_KEYS = [
  'acrobatics',
  'animalHandling',
  'arcana',
  'athletics',
  'deception',
  'history',
  'insight',
  'intimidation',
  'investigation',
  'medicine',
  'nature',
  'perception',
  'performance',
  'persuasion',
  'religion',
  'sleightOfHand',
  'stealth',
  'survival',
] as const;

type SkillKey = (typeof SKILL_KEYS)[number];

const SKILL_LABELS: Readonly<Record<SkillKey, string>> = {
  acrobatics: 'Acrobatics',
  animalHandling: 'Animal Handling',
  arcana: 'Arcana',
  athletics: 'Athletics',
  deception: 'Deception',
  history: 'History',
  insight: 'Insight',
  intimidation: 'Intimidation',
  investigation: 'Investigation',
  medicine: 'Medicine',
  nature: 'Nature',
  perception: 'Perception',
  performance: 'Performance',
  persuasion: 'Persuasion',
  religion: 'Religion',
  sleightOfHand: 'Sleight of Hand',
  stealth: 'Stealth',
  survival: 'Survival',
};

const CONDITION_KEYS = [
  'blinded',
  'charmed',
  'deafened',
  'frightened',
  'grappled',
  'incapacitated',
  'invisible',
  'paralyzed',
  'petrified',
  'poisoned',
  'prone',
  'restrained',
  'stunned',
  'unconscious',
] as const;

const CONDITION_LABELS: Readonly<Record<(typeof CONDITION_KEYS)[number], string>> = {
  blinded: 'Blinded',
  charmed: 'Charmed',
  deafened: 'Deafened',
  frightened: 'Frightened',
  grappled: 'Grappled',
  incapacitated: 'Incapacitated',
  invisible: 'Invisible',
  paralyzed: 'Paralyzed',
  petrified: 'Petrified',
  poisoned: 'Poisoned',
  prone: 'Prone',
  restrained: 'Restrained',
  stunned: 'Stunned',
  unconscious: 'Unconscious',
};

const SLOT_LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

const HIT_DIE_KEYS = ['d6', 'd8', 'd10', 'd12'] as const;

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function numberLeaf(
  label: string,
  defaultValue: number,
  options: {
    readonly editable?: boolean;
    readonly exclusiveGroup?: string;
    readonly optionLabel?: string;
    readonly columnLabel?: string;
    readonly flag?: boolean;
  } = {},
): LeafNode {
  const { editable = true, ...rest } = options;
  return {
    kind: 'leaf',
    schema: {
      type: 'number',
      label,
      isEditable: editable,
      defaultValue,
      ...rest,
    },
  };
}

function textLeaf(label: string, options: { readonly columnLabel?: string } = {}): LeafNode {
  return {
    kind: 'leaf',
    schema: {
      type: 'text',
      label,
      isEditable: true,
      defaultValue: '',
      ...options,
    },
  };
}

function group(
  label: string,
  children: Record<string, FieldNode>,
  columns?: readonly string[],
): FieldNode {
  return {
    kind: 'group',
    label,
    children,
    ...(columns === undefined ? {} : { columns }),
  };
}

function list(
  label: string,
  columns: readonly string[],
  capacity: number,
  shape: Record<string, LeafNode>,
  dice?: string,
): FieldNode {
  return {
    kind: 'list',
    list: {
      label,
      columns,
      capacity,
      shape,
      ...(dice === undefined ? {} : { dice }),
    },
  };
}

function abilityGroups(): Record<string, FieldNode> {
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      ability,
      group(
        ABILITY_LABELS[ability],
        {
          score: numberLeaf(ABILITY_LABELS[ability], 10, { columnLabel: 'Score' }),
          mod: numberLeaf(`${ABILITY_LABELS[ability]} modifier`, 0, {
            editable: false,
            columnLabel: 'Modifier',
          }),
        },
        ['score', 'mod'],
      ),
    ]),
  );
}

function saveGroups(): Record<string, FieldNode> {
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      ability,
      group(
        ABILITY_LABELS[ability],
        {
          prof: numberLeaf(`${ABILITY_LABELS[ability]} save proficiency`, 0, {
            columnLabel: 'Proficient',
            flag: true,
          }),
          misc: numberLeaf(`${ABILITY_LABELS[ability]} save misc`, 0, { columnLabel: 'Misc' }),
          total: numberLeaf(`${ABILITY_LABELS[ability]} save total`, 0, {
            editable: false,
            columnLabel: 'Total',
          }),
        },
        ['prof', 'misc', 'total'],
      ),
    ]),
  );
}

function skillGroups(): Record<string, FieldNode> {
  return Object.fromEntries(
    SKILL_KEYS.map((skill) => [
      skill,
      group(
        SKILL_LABELS[skill],
        {
          rank: numberLeaf(`${SKILL_LABELS[skill]} rank`, 0, { columnLabel: 'Rank' }),
          misc: numberLeaf(`${SKILL_LABELS[skill]} misc`, 0, { columnLabel: 'Misc' }),
          total: numberLeaf(`${SKILL_LABELS[skill]} total`, 0, {
            editable: false,
            columnLabel: 'Total',
          }),
        },
        ['rank', 'misc', 'total'],
      ),
    ]),
  );
}

function slotGroups(): Record<string, FieldNode> {
  return Object.fromEntries(
    SLOT_LEVELS.map((level) => [
      `slot${level}`,
      group(
        `Level ${level}`,
        {
          total: numberLeaf(`Level ${level} spell slots`, 0, { columnLabel: 'Total' }),
          spent: numberLeaf(`Level ${level} spell slots spent`, 0, { columnLabel: 'Spent' }),
          remaining: numberLeaf(`Level ${level} spell slots remaining`, 0, {
            editable: false,
            columnLabel: 'Remaining',
          }),
        },
        ['total', 'spent', 'remaining'],
      ),
    ]),
  );
}

function hitDieGroups(): Record<string, FieldNode> {
  return Object.fromEntries(
    HIT_DIE_KEYS.map((die) => [
      die,
      group(
        `${die} Hit Dice`,
        {
          extra: numberLeaf(`${die} Hit Dice extra`, 0, { columnLabel: 'Extra' }),
          spent: numberLeaf(`${die} Hit Dice spent`, 0, { columnLabel: 'Spent' }),
          remaining: numberLeaf(`${die} Hit Dice remaining`, 0, {
            editable: false,
            columnLabel: 'Remaining',
          }),
        },
        ['extra', 'spent', 'remaining'],
      ),
    ]),
  );
}

function attackAbilityLeaves(): Record<string, LeafNode> {
  return Object.fromEntries(
    ABILITY_KEYS.map((ability, index) => [
      `ability${capitalize(ability)}`,
      numberLeaf('Ability', index === 0 ? 1 : 0, {
        exclusiveGroup: 'attackAbility',
        optionLabel: ABILITY_LABELS[ability],
      }),
    ]),
  );
}

function attackShape(): Record<string, LeafNode> {
  return {
    ...attackAbilityLeaves(),
    damage: textLeaf('Damage'),
    damageBonus: numberLeaf('Damage bonus', 0, { editable: false }),
    misc: numberLeaf('Misc', 0),
    name: textLeaf('Name'),
    notes: textLeaf('Notes'),
    proficient: numberLeaf('Proficient', 0, { flag: true }),
    toHit: numberLeaf('To hit', 0, { editable: false }),
  };
}

function inventoryShape(): Record<string, LeafNode> {
  return {
    attuned: numberLeaf('Attuned', 0, { flag: true }),
    lineWeight: numberLeaf('Total weight', 0, { editable: false }),
    name: textLeaf('Name'),
    notes: textLeaf('Notes'),
    quantity: numberLeaf('Quantity', 0),
    weight: numberLeaf('Weight', 0),
  };
}

function featureShape(): Record<string, LeafNode> {
  return {
    name: textLeaf('Name'),
    source: textLeaf('Source'),
    text: textLeaf('Text'),
  };
}

function spellShape(): Record<string, LeafNode> {
  return {
    concentration: numberLeaf('Concentration', 0, { flag: true }),
    level: numberLeaf('Level', 0),
    name: textLeaf('Name'),
    notes: textLeaf('Notes'),
    prepared: numberLeaf('Prepared', 0, { flag: true }),
    ritual: numberLeaf('Ritual', 0, { flag: true }),
  };
}

function resourceShape(): Record<string, LeafNode> {
  return {
    max: numberLeaf('Max', 0),
    name: textLeaf('Name'),
    rechargeLong: numberLeaf('Recharge', 0, {
      exclusiveGroup: 'resourceRecharge',
      optionLabel: 'Long rest',
    }),
    rechargeNone: numberLeaf('Recharge', 1, {
      exclusiveGroup: 'resourceRecharge',
      optionLabel: 'None',
    }),
    rechargeShort: numberLeaf('Recharge', 0, {
      exclusiveGroup: 'resourceRecharge',
      optionLabel: 'Short rest',
    }),
    used: numberLeaf('Used', 0),
  };
}

const ACTIONS: readonly ResetAction[] = [
  {
    id: 'long_rest',
    label: 'Long rest',
    assignments: [
      { target: 'combat.hp.current', expression: 'combat_hp_max' },
      { target: 'exhaustion.level', expression: 'max(0, exhaustion_level - 1)' },
      { target: 'status.deathSaves.successes', expression: '0' },
      { target: 'status.deathSaves.failures', expression: '0' },
      { target: 'spellcasting.pact.used', expression: '0' },
      { target: 'hitDice.primary.spent', expression: '0' },
      { target: 'hitDice.d6.spent', expression: '0' },
      { target: 'hitDice.d8.spent', expression: '0' },
      { target: 'hitDice.d10.spent', expression: '0' },
      { target: 'hitDice.d12.spent', expression: '0' },
      { target: 'spellcasting.slot1.spent', expression: '0' },
      { target: 'spellcasting.slot2.spent', expression: '0' },
      { target: 'spellcasting.slot3.spent', expression: '0' },
      { target: 'spellcasting.slot4.spent', expression: '0' },
      { target: 'spellcasting.slot5.spent', expression: '0' },
      { target: 'spellcasting.slot6.spent', expression: '0' },
      { target: 'spellcasting.slot7.spent', expression: '0' },
      { target: 'spellcasting.slot8.spent', expression: '0' },
      { target: 'spellcasting.slot9.spent', expression: '0' },
      { target: 'resources.list.used', expression: 'record_used * record_rechargeNone' },
    ],
  },
  {
    id: 'short_rest',
    label: 'Short rest',
    assignments: [
      { target: 'spellcasting.pact.used', expression: '0' },
      { target: 'resources.list.used', expression: 'record_used * (1 - record_rechargeShort)' },
    ],
  },
];

export const FIFTH_DEMO_TEMPLATE: TemplateDetail = {
  id: FIFTH_DEMO_TEMPLATE_ID,
  name: '5E Character',
  default: false,
  kinds: ['character'],
  gameSystem: {
    id: 'fifth',
    name: '5E',
    compatibilityLabel: '5E compatible',
    notice: { label: 'SRD 5.2.1', path: '/srd' },
  },
  fields: {
    identity: group('Identity', {
      name: textLeaf('Name'),
      class: textLeaf('Class'),
      subclass: textLeaf('Subclass'),
      species: textLeaf('Species'),
      background: textLeaf('Background'),
      level: numberLeaf('Level', 1),
      jackOfAllTrades: numberLeaf('Jack of All Trades', 0, { flag: true }),
      proficiencyBonus: numberLeaf('Proficiency bonus', 0, { editable: false }),
    }),
    abilities: group('Abilities', abilityGroups()),
    saves: group('Saves', saveGroups()),
    skills: group('Skills', skillGroups()),
    exhaustion: group('Exhaustion', {
      level: numberLeaf('Exhaustion level', 0),
      penalty: numberLeaf('Exhaustion penalty', 0, { editable: false }),
    }),
    passive: group('Passive Perception', {
      choiceNone: numberLeaf('Passive Perception', 1, {
        exclusiveGroup: 'perceptionChoice',
        optionLabel: 'None',
      }),
      choiceAdvantage: numberLeaf('Passive Perception with advantage', 0, {
        exclusiveGroup: 'perceptionChoice',
        optionLabel: '+5 (Advantage)',
      }),
      choiceDisadvantage: numberLeaf('Passive Perception with disadvantage', 0, {
        exclusiveGroup: 'perceptionChoice',
        optionLabel: '-5 (Disadvantage)',
      }),
      perception: numberLeaf('Passive Perception', 0, { editable: false }),
    }),
    armor: group('Armor', {
      choiceNone: numberLeaf('Armor', 1, {
        exclusiveGroup: 'armorChoice',
        optionLabel: 'None',
      }),
      choiceLight: numberLeaf('Armor', 0, {
        exclusiveGroup: 'armorChoice',
        optionLabel: 'Light armor',
      }),
      choiceMedium: numberLeaf('Armor', 0, {
        exclusiveGroup: 'armorChoice',
        optionLabel: 'Medium armor',
      }),
      choiceHeavy: numberLeaf('Armor', 0, {
        exclusiveGroup: 'armorChoice',
        optionLabel: 'Heavy armor',
      }),
      choiceBarbarian: numberLeaf('Unarmored defense', 0, {
        exclusiveGroup: 'armorChoice',
        optionLabel: 'Barbarian',
      }),
      choiceMonk: numberLeaf('Unarmored defense', 0, {
        exclusiveGroup: 'armorChoice',
        optionLabel: 'Monk',
      }),
      base: numberLeaf('Armor class base', 10),
      shield: numberLeaf('Shield', 0, { flag: true }),
      misc: numberLeaf('Armor class misc', 0),
      armorClass: numberLeaf('Armor class', 0, { editable: false }),
    }),
    hitDice: group('Hit Dice', {
      primary: group(
        'Primary Hit Dice',
        {
          size: numberLeaf('Hit Die size', 8, { columnLabel: 'Size' }),
          count: numberLeaf('Primary Hit Dice', 0, {
            editable: false,
            columnLabel: 'Count',
          }),
          spent: numberLeaf('Primary Hit Dice spent', 0, { columnLabel: 'Spent' }),
          remaining: numberLeaf('Primary Hit Dice remaining', 0, {
            editable: false,
            columnLabel: 'Remaining',
          }),
        },
        ['size', 'count', 'spent', 'remaining'],
      ),
      ...hitDieGroups(),
    }),
    spellcasting: group('Spellcasting', {
      abilityStrength: numberLeaf('Spellcasting ability', 0, {
        exclusiveGroup: 'spellcastingAbility',
        optionLabel: 'Strength',
      }),
      abilityDexterity: numberLeaf('Spellcasting ability', 0, {
        exclusiveGroup: 'spellcastingAbility',
        optionLabel: 'Dexterity',
      }),
      abilityConstitution: numberLeaf('Spellcasting ability', 0, {
        exclusiveGroup: 'spellcastingAbility',
        optionLabel: 'Constitution',
      }),
      abilityIntelligence: numberLeaf('Spellcasting ability', 0, {
        exclusiveGroup: 'spellcastingAbility',
        optionLabel: 'Intelligence',
      }),
      abilityWisdom: numberLeaf('Spellcasting ability', 0, {
        exclusiveGroup: 'spellcastingAbility',
        optionLabel: 'Wisdom',
      }),
      abilityCharisma: numberLeaf('Spellcasting ability', 0, {
        exclusiveGroup: 'spellcastingAbility',
        optionLabel: 'Charisma',
      }),
      abilityMod: numberLeaf('Spellcasting ability modifier', 0, { editable: false }),
      attack: numberLeaf('Spell attack bonus', 0, { editable: false }),
      saveDC: numberLeaf('Spell save DC', 0, { editable: false }),
      preparedLimit: numberLeaf('Prepared spell limit', 0),
      ...slotGroups(),
      pact: group(
        'Pact Magic',
        {
          count: numberLeaf('Pact Magic slots', 0, { columnLabel: 'Count' }),
          level: numberLeaf('Pact Magic slot level', 0, { columnLabel: 'Level' }),
          used: numberLeaf('Pact Magic slots used', 0, { columnLabel: 'Used' }),
          remaining: numberLeaf('Pact Magic slots remaining', 0, {
            editable: false,
            columnLabel: 'Remaining',
          }),
        },
        ['count', 'level', 'used', 'remaining'],
      ),
    }),
    combat: group('Combat', {
      initiative: group(
        'Initiative',
        {
          alert: numberLeaf('Alert', 0, { columnLabel: 'Alert', flag: true }),
          misc: numberLeaf('Initiative misc', 0, { columnLabel: 'Misc' }),
          total: numberLeaf('Initiative', 0, { editable: false, columnLabel: 'Total' }),
        },
        ['alert', 'misc', 'total'],
      ),
      speed: group(
        'Speed',
        {
          base: numberLeaf('Speed base', 30, { columnLabel: 'Base' }),
          total: numberLeaf('Speed', 0, { editable: false, columnLabel: 'Speed' }),
        },
        ['base', 'total'],
      ),
      armorStrengthRequirement: numberLeaf('Heavy armor Strength requirement', 0),
      hp: group(
        'Hit Points',
        {
          base: numberLeaf('Hit point base', 0, { columnLabel: 'Base' }),
          perLevel: numberLeaf('Hit points per level', 0, { columnLabel: 'Per level' }),
          misc: numberLeaf('Hit point misc', 0, { columnLabel: 'Misc' }),
          max: numberLeaf('Hit point maximum', 0, { editable: false, columnLabel: 'Maximum' }),
          current: numberLeaf('Current hit points', 0, { columnLabel: 'Current' }),
          temp: numberLeaf('Temporary hit points', 0, { columnLabel: 'Temporary' }),
        },
        ['base', 'perLevel', 'misc', 'max', 'current', 'temp'],
      ),
      carryingCapacity: numberLeaf('Carrying capacity (lb)', 0, { editable: false }),
    }),
    status: group('Status', {
      xp: numberLeaf('Experience points', 0, { columnLabel: 'XP' }),
      heroicInspiration: numberLeaf('Heroic Inspiration', 0, { flag: true }),
      conditions: group(
        'Conditions',
        Object.fromEntries(
          CONDITION_KEYS.map((condition) => [
            condition,
            numberLeaf(CONDITION_LABELS[condition], 0, { flag: true }),
          ]),
        ),
      ),
      deathSaves: group(
        'Death saves (10 or higher succeeds; a natural 1 is two failures; a natural 20 regains 1 hit point and resets both)',
        {
          successes: numberLeaf('Death save successes', 0, { columnLabel: 'Successes' }),
          failures: numberLeaf('Death save failures', 0, { columnLabel: 'Failures' }),
        },
        ['successes', 'failures'],
      ),
    }),
    coins: group('Coins', {
      cp: numberLeaf('Copper pieces', 0),
      sp: numberLeaf('Silver pieces', 0),
      ep: numberLeaf('Electrum pieces', 0),
      gp: numberLeaf('Gold pieces', 0),
      pp: numberLeaf('Platinum pieces', 0),
    }),
    attacks: group('Attacks', {
      list: list(
        'Attacks',
        ['name', 'toHit', 'damageBonus', 'damage', 'notes'],
        10,
        attackShape(),
        '1d20 + @.toHit + exhaustion.penalty',
      ),
    }),
    inventory: group('Inventory', {
      items: list(
        'Items',
        ['name', 'quantity', 'weight', 'lineWeight', 'attuned', 'notes'],
        100,
        inventoryShape(),
      ),
      totalWeight: numberLeaf('Total weight', 0, { editable: false }),
      attuned: numberLeaf('Attuned items', 0, { editable: false }),
    }),
    features: group('Features', {
      list: list('Features and feats', ['name', 'source', 'text'], 50, featureShape()),
    }),
    spells: group('Spells', {
      list: list(
        'Spells',
        ['name', 'level', 'prepared', 'concentration', 'ritual', 'notes'],
        100,
        spellShape(),
      ),
    }),
    resources: group('Resources', {
      list: list(
        'Resources',
        ['name', 'max', 'used', 'rechargeShort', 'rechargeLong', 'rechargeNone'],
        20,
        resourceShape(),
      ),
    }),
  },
  dice: {
    section: {
      abilities: '1d20 + @.mod + exhaustion.penalty',
      saves: '1d20 + @.total + exhaustion.penalty',
      skills: '1d20 + @.total + exhaustion.penalty',
    },
    row: {
      'combat.initiative': '1d20 + @.total + exhaustion.penalty',
      'hitDice.d10': '1d10 + abilities.constitution.mod min 1',
      'hitDice.d12': '1d12 + abilities.constitution.mod min 1',
      'hitDice.d6': '1d6 + abilities.constitution.mod min 1',
      'hitDice.d8': '1d8 + abilities.constitution.mod min 1',
      'hitDice.primary': '1d{hitDice.primary.size} + abilities.constitution.mod min 1',
      'spellcasting.attack': '1d20 + @ + exhaustion.penalty',
      'status.deathSaves': '1d20 + exhaustion.penalty',
    },
    notD20: { section: ['hitDice'], row: [] },
  },
  actions: ACTIONS,
};

function abilityData(): Record<string, unknown> {
  const scores: Readonly<Record<AbilityKey, number>> = {
    strength: 12,
    dexterity: 10,
    constitution: 14,
    intelligence: 11,
    wisdom: 16,
    charisma: 13,
  };
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      ability,
      { score: scores[ability], mod: Math.floor((scores[ability] - 10) / 2) },
    ]),
  );
}

function saveData(): Record<string, unknown> {
  const totals: Readonly<Record<AbilityKey, number>> = {
    strength: 1,
    dexterity: 0,
    constitution: 2,
    intelligence: 0,
    wisdom: 5,
    charisma: 3,
  };
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      ability,
      {
        prof: ability === 'wisdom' || ability === 'charisma' ? 1 : 0,
        misc: 0,
        total: totals[ability],
      },
    ]),
  );
}

function skillData(): Record<string, unknown> {
  const totals: Readonly<Record<SkillKey, number>> = {
    acrobatics: 0,
    animalHandling: 3,
    arcana: 0,
    athletics: 1,
    deception: 1,
    history: 0,
    insight: 5,
    intimidation: 1,
    investigation: 0,
    medicine: 5,
    nature: 0,
    perception: 3,
    performance: 1,
    persuasion: 3,
    religion: 2,
    sleightOfHand: 0,
    stealth: 0,
    survival: 3,
  };
  const ranks: Readonly<Partial<Record<SkillKey, number>>> = {
    insight: 1,
    medicine: 1,
    persuasion: 1,
    religion: 1,
  };
  return Object.fromEntries(
    SKILL_KEYS.map((skill) => [skill, { rank: ranks[skill] ?? 0, misc: 0, total: totals[skill] }]),
  );
}

function conditionData(): Record<string, number> {
  return Object.fromEntries(CONDITION_KEYS.map((condition) => [condition, 0]));
}

function spellcastingData(): Record<string, unknown> {
  const slots = Object.fromEntries(
    SLOT_LEVELS.map((level) => {
      const total = level === 1 ? 4 : level === 2 ? 2 : 0;
      const spent = level === 1 ? 1 : 0;
      return [`slot${level}`, { total, spent, remaining: total - spent }];
    }),
  );
  return {
    abilityStrength: 0,
    abilityDexterity: 0,
    abilityConstitution: 0,
    abilityIntelligence: 0,
    abilityWisdom: 1,
    abilityCharisma: 0,
    abilityMod: 3,
    attack: 5,
    saveDC: 13,
    preparedLimit: 6,
    ...slots,
    pact: { count: 0, level: 0, used: 0, remaining: 0 },
  };
}

function attackRecord(fields: Record<string, unknown>, position: number): Record<string, unknown> {
  return {
    abilityStrength: 0,
    abilityDexterity: 0,
    abilityConstitution: 0,
    abilityIntelligence: 0,
    abilityWisdom: 0,
    abilityCharisma: 0,
    damageBonus: 0,
    misc: 0,
    proficient: 0,
    ...fields,
    position,
  };
}

function inventoryRecord(
  fields: Record<string, unknown>,
  position: number,
): Record<string, unknown> {
  return { attuned: 0, lineWeight: 0, notes: '', ...fields, position };
}

function recordID(index: number): string {
  return `r${index.toString(16).padStart(32, '0')}`;
}

export const FIFTH_DEMO_SHEET: Sheet = {
  id: FIFTH_DEMO_SHEET_ID,
  templateId: FIFTH_DEMO_TEMPLATE_ID,
  kind: 'character',
  name: 'Ysolde Ashgrave',
  data: {
    identity: {
      name: 'Ysolde Ashgrave',
      class: 'Cleric',
      subclass: 'Life Domain',
      species: 'Human',
      background: 'Acolyte',
      level: 3,
      jackOfAllTrades: 0,
      proficiencyBonus: 2,
    },
    abilities: abilityData(),
    saves: saveData(),
    skills: skillData(),
    exhaustion: { level: 0, penalty: 0 },
    passive: { choiceNone: 1, choiceAdvantage: 0, choiceDisadvantage: 0, perception: 13 },
    armor: {
      choiceNone: 0,
      choiceLight: 0,
      choiceMedium: 1,
      choiceHeavy: 0,
      choiceBarbarian: 0,
      choiceMonk: 0,
      base: 13,
      shield: 1,
      misc: 0,
      armorClass: 15,
    },
    hitDice: {
      primary: { size: 8, count: 3, spent: 0, remaining: 3 },
      d6: { extra: 0, spent: 0, remaining: 0 },
      d8: { extra: 0, spent: 0, remaining: 0 },
      d10: { extra: 0, spent: 0, remaining: 0 },
      d12: { extra: 0, spent: 0, remaining: 0 },
    },
    spellcasting: spellcastingData(),
    combat: {
      initiative: { alert: 0, misc: 0, total: 0 },
      speed: { base: 30, total: 30 },
      armorStrengthRequirement: 0,
      hp: { base: 8, perLevel: 5, misc: 0, max: 29, current: 29, temp: 0 },
      carryingCapacity: 180,
    },
    status: {
      xp: 900,
      heroicInspiration: 0,
      conditions: conditionData(),
      deathSaves: { successes: 0, failures: 0 },
    },
    coins: { cp: 0, sp: 10, ep: 0, gp: 15, pp: 0 },
    attacks: {
      list: {
        [recordID(1)]: attackRecord(
          {
            abilityStrength: 1,
            damage: '1d6 + 1',
            damageBonus: 1,
            name: 'Mace',
            notes: 'Melee, 5 ft.',
            proficient: 1,
            toHit: 3,
          },
          0,
        ),
        [recordID(2)]: attackRecord(
          {
            abilityDexterity: 1,
            damage: '1d8',
            name: 'Light Crossbow',
            notes: 'Ranged, 80/320 ft.',
            proficient: 1,
            toHit: 2,
          },
          1,
        ),
      },
    },
    inventory: {
      items: {
        [recordID(1)]: inventoryRecord(
          {
            name: 'Chain Shirt',
            quantity: 1,
            weight: 10,
            lineWeight: 10,
            notes: 'Medium armor',
          },
          0,
        ),
        [recordID(2)]: inventoryRecord(
          { name: 'Shield', quantity: 1, weight: 6, lineWeight: 6, notes: '+2 Armor Class' },
          1,
        ),
        [recordID(3)]: inventoryRecord(
          {
            name: 'Holy Symbol',
            quantity: 1,
            weight: 1,
            lineWeight: 1,
            notes: 'Focus for cleric spells',
          },
          2,
        ),
        [recordID(4)]: inventoryRecord({ name: 'Mace', quantity: 1, weight: 4, lineWeight: 4 }, 3),
        [recordID(5)]: inventoryRecord(
          { name: 'Light Crossbow', quantity: 1, weight: 5, lineWeight: 5 },
          4,
        ),
        [recordID(6)]: inventoryRecord(
          {
            name: 'Crossbow Bolts',
            quantity: 20,
            weight: 0.05,
            lineWeight: 1,
            notes: 'Ammunition',
          },
          5,
        ),
        [recordID(7)]: inventoryRecord(
          { name: "Explorer's Pack", quantity: 1, weight: 30, lineWeight: 30 },
          6,
        ),
      },
      totalWeight: 57,
      attuned: 0,
    },
    features: {
      list: {
        [recordID(1)]: {
          name: 'Life Domain',
          source: 'Cleric 3',
          text: 'Domain spells: Bless, Cure Wounds, Lesser Restoration, Spiritual Weapon.',
          position: 0,
        },
        [recordID(2)]: {
          name: 'Channel Divinity',
          source: 'Cleric 2',
          text: 'Turn Undead or Preserve Life; one use returns on a short rest.',
          position: 1,
        },
        [recordID(3)]: {
          name: 'Disciple of Life',
          source: 'Life Domain 3',
          text: 'Healing spells restore extra hit points equal to 2 plus the spell level.',
          position: 2,
        },
      },
    },
    spells: {
      list: {
        [recordID(1)]: {
          name: 'Guidance',
          level: 0,
          prepared: 1,
          concentration: 1,
          ritual: 0,
          notes: 'Cantrip; adds 1d4 to one ability check.',
          position: 0,
        },
        [recordID(2)]: {
          name: 'Sacred Flame',
          level: 0,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Cantrip; Dexterity save or 1d8 radiant damage.',
          position: 1,
        },
        [recordID(3)]: {
          name: 'Thaumaturgy',
          level: 0,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Cantrip; minor divine omen.',
          position: 2,
        },
        [recordID(4)]: {
          name: 'Bless',
          level: 1,
          prepared: 1,
          concentration: 1,
          ritual: 0,
          notes: 'Three creatures add 1d4 to attack rolls and saves.',
          position: 3,
        },
        [recordID(5)]: {
          name: 'Cure Wounds',
          level: 1,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Touch restores 2d8 + Wisdom modifier hit points.',
          position: 4,
        },
        [recordID(6)]: {
          name: 'Healing Word',
          level: 1,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Bonus action; restores 2d4 + Wisdom modifier hit points at range.',
          position: 5,
        },
        [recordID(7)]: {
          name: 'Guiding Bolt',
          level: 1,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: '4d6 radiant damage; the next attack against the target has Advantage.',
          position: 6,
        },
        [recordID(8)]: {
          name: 'Lesser Restoration',
          level: 2,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Ends one disease or condition on a creature.',
          position: 7,
        },
        [recordID(9)]: {
          name: 'Spiritual Weapon',
          level: 2,
          prepared: 1,
          concentration: 0,
          ritual: 0,
          notes: 'Bonus action; a spectral weapon attacks for force damage.',
          position: 8,
        },
      },
    },
    resources: {
      list: {
        [recordID(1)]: {
          name: 'Channel Divinity',
          max: 2,
          used: 0,
          rechargeShort: 0,
          rechargeLong: 1,
          rechargeNone: 0,
          position: 0,
        },
      },
    },
  },
  dice: { section: {}, row: {} },
  createdAt: '2026-11-24T00:00:00.000Z',
  updatedAt: '2026-11-24T00:00:00.000Z',
};
