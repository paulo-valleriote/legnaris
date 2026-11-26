import type { PresetConfig, PresetPiece } from '../types';

const abilityScores = [
  {
    score: 'abilities.strength.score',
    modifier: 'abilities.strength.mod',
    row: 'abilities.strength',
  },
  {
    score: 'abilities.dexterity.score',
    modifier: 'abilities.dexterity.mod',
    row: 'abilities.dexterity',
  },
  {
    score: 'abilities.constitution.score',
    modifier: 'abilities.constitution.mod',
    row: 'abilities.constitution',
  },
  {
    score: 'abilities.intelligence.score',
    modifier: 'abilities.intelligence.mod',
    row: 'abilities.intelligence',
  },
  { score: 'abilities.wisdom.score', modifier: 'abilities.wisdom.mod', row: 'abilities.wisdom' },
  {
    score: 'abilities.charisma.score',
    modifier: 'abilities.charisma.mod',
    row: 'abilities.charisma',
  },
];

const savingThrows = [
  { proficiency: 'saves.strength.prof', total: 'saves.strength.total', row: 'saves.strength' },
  { proficiency: 'saves.dexterity.prof', total: 'saves.dexterity.total', row: 'saves.dexterity' },
  {
    proficiency: 'saves.constitution.prof',
    total: 'saves.constitution.total',
    row: 'saves.constitution',
  },
  {
    proficiency: 'saves.intelligence.prof',
    total: 'saves.intelligence.total',
    row: 'saves.intelligence',
  },
  { proficiency: 'saves.wisdom.prof', total: 'saves.wisdom.total', row: 'saves.wisdom' },
  { proficiency: 'saves.charisma.prof', total: 'saves.charisma.total', row: 'saves.charisma' },
];

const skills = [
  {
    rank: 'skills.acrobatics.rank',
    total: 'skills.acrobatics.total',
    row: 'skills.acrobatics',
    ability: 'DEX',
  },
  {
    rank: 'skills.animalHandling.rank',
    total: 'skills.animalHandling.total',
    row: 'skills.animalHandling',
    ability: 'WIS',
  },
  {
    rank: 'skills.arcana.rank',
    total: 'skills.arcana.total',
    row: 'skills.arcana',
    ability: 'INT',
  },
  {
    rank: 'skills.athletics.rank',
    total: 'skills.athletics.total',
    row: 'skills.athletics',
    ability: 'STR',
  },
  {
    rank: 'skills.deception.rank',
    total: 'skills.deception.total',
    row: 'skills.deception',
    ability: 'CHA',
  },
  {
    rank: 'skills.history.rank',
    total: 'skills.history.total',
    row: 'skills.history',
    ability: 'INT',
  },
  {
    rank: 'skills.insight.rank',
    total: 'skills.insight.total',
    row: 'skills.insight',
    ability: 'WIS',
  },
  {
    rank: 'skills.intimidation.rank',
    total: 'skills.intimidation.total',
    row: 'skills.intimidation',
    ability: 'CHA',
  },
  {
    rank: 'skills.investigation.rank',
    total: 'skills.investigation.total',
    row: 'skills.investigation',
    ability: 'INT',
  },
  {
    rank: 'skills.medicine.rank',
    total: 'skills.medicine.total',
    row: 'skills.medicine',
    ability: 'WIS',
  },
  {
    rank: 'skills.nature.rank',
    total: 'skills.nature.total',
    row: 'skills.nature',
    ability: 'INT',
  },
  {
    rank: 'skills.perception.rank',
    total: 'skills.perception.total',
    row: 'skills.perception',
    ability: 'WIS',
  },
  {
    rank: 'skills.performance.rank',
    total: 'skills.performance.total',
    row: 'skills.performance',
    ability: 'CHA',
  },
  {
    rank: 'skills.persuasion.rank',
    total: 'skills.persuasion.total',
    row: 'skills.persuasion',
    ability: 'CHA',
  },
  {
    rank: 'skills.religion.rank',
    total: 'skills.religion.total',
    row: 'skills.religion',
    ability: 'INT',
  },
  {
    rank: 'skills.sleightOfHand.rank',
    total: 'skills.sleightOfHand.total',
    row: 'skills.sleightOfHand',
    ability: 'DEX',
  },
  {
    rank: 'skills.stealth.rank',
    total: 'skills.stealth.total',
    row: 'skills.stealth',
    ability: 'DEX',
  },
  {
    rank: 'skills.survival.rank',
    total: 'skills.survival.total',
    row: 'skills.survival',
    ability: 'WIS',
  },
];

const conditionPaths = [
  'status.conditions.blinded',
  'status.conditions.charmed',
  'status.conditions.deafened',
  'status.conditions.frightened',
  'status.conditions.grappled',
  'status.conditions.incapacitated',
  'status.conditions.invisible',
  'status.conditions.paralyzed',
  'status.conditions.petrified',
  'status.conditions.poisoned',
  'status.conditions.prone',
  'status.conditions.restrained',
  'status.conditions.stunned',
  'status.conditions.unconscious',
];

const hitDicePieces: PresetPiece[] = ['d6', 'd8', 'd10', 'd12'].map((die) => ({
  kind: 'pips',
  key: `hitDice${die.toUpperCase()}`,
  count: `hitDice.${die}.spent`,
  max: `hitDice.${die}.extra`,
}));

const paths = (...list: string[]) => list.map((path) => ({ path }));

const abilityKeys = ['Strength', 'Dexterity', 'Constitution', 'Intelligence', 'Wisdom', 'Charisma'];

const setupPieces: readonly PresetPiece[] = [
  {
    kind: 'fields',
    key: 'armorSetup',
    title: 'preset.fields.armor',
    summary: 'armor.armorClass',
    fields: paths(
      'armor.choiceNone',
      'armor.choiceLight',
      'armor.choiceMedium',
      'armor.choiceHeavy',
      'armor.choiceBarbarian',
      'armor.choiceMonk',
      'armor.base',
      'armor.shield',
      'armor.misc',
    ),
  },
  {
    kind: 'fields',
    key: 'movementSetup',
    title: 'preset.fields.movement',
    fields: paths(
      'combat.initiative.alert',
      'combat.initiative.misc',
      'combat.speed.base',
      'combat.armorStrengthRequirement',
    ),
  },
  {
    kind: 'fields',
    key: 'hitPointsSetup',
    title: 'preset.fields.hitPoints',
    summary: 'combat.hp.max',
    fields: paths(
      'combat.hp.base',
      'combat.hp.perLevel',
      'combat.hp.misc',
      'hitDice.primary.size',
      'hitDice.d6.extra',
      'hitDice.d8.extra',
      'hitDice.d10.extra',
      'hitDice.d12.extra',
    ),
  },
  {
    kind: 'fields',
    key: 'savesSetup',
    title: 'preset.fields.saves',
    fields: savingThrows.map((save) => ({ path: `${save.row}.misc` })),
  },
  {
    kind: 'fields',
    key: 'skillsSetup',
    title: 'preset.fields.skills',
    fields: [
      ...skills.map((skill) => ({ path: `${skill.row}.misc` })),
      ...paths('passive.choiceNone', 'passive.choiceAdvantage', 'passive.choiceDisadvantage'),
    ],
  },
  {
    kind: 'fields',
    key: 'spellcastingSetup',
    title: 'preset.fields.spellcasting',
    fields: paths(
      ...abilityKeys.map((ability) => `spellcasting.ability${ability}`),
      'spellcasting.abilityMod',
      'spellcasting.attack',
      'spellcasting.saveDC',
      'spellcasting.preparedLimit',
      ...[1, 2, 3, 4, 5, 6, 7, 8, 9].map((level) => `spellcasting.slot${level}.total`),
      'spellcasting.pact.count',
      'spellcasting.pact.level',
    ),
  },
  {
    kind: 'fields',
    key: 'coins',
    title: 'preset.fields.coins',
    fields: paths(
      'coins.cp',
      'coins.sp',
      'coins.ep',
      'coins.gp',
      'coins.pp',
      'inventory.totalWeight',
      'combat.carryingCapacity',
      'inventory.attuned',
    ),
  },
  { kind: 'diceFormulas', key: 'diceFormulas' },
];

const pieces: readonly PresetPiece[] = [
  {
    kind: 'fields',
    key: 'identity',
    title: 'preset.fields.character',
    fields: paths(
      'identity.subclass',
      'identity.species',
      'identity.background',
      'identity.level',
      'identity.jackOfAllTrades',
      'status.xp',
      'status.heroicInspiration',
    ),
  },
  {
    kind: 'statPills',
    key: 'vitals',
    pills: [
      { path: 'armor.armorClass' },
      { path: 'combat.initiative.total', row: 'combat.initiative' },
      { path: 'combat.speed.total', row: 'combat.speed' },
      { path: 'identity.proficiencyBonus' },
      { path: 'passive.perception' },
    ],
  },
  {
    kind: 'hitPoints',
    key: 'hitPoints',
    current: 'combat.hp.current',
    max: 'combat.hp.max',
    temp: 'combat.hp.temp',
    downNotice: 'preset.hitPoints.downSaves',
  },
  { kind: 'abilityGrid', key: 'abilities', section: 'abilities', abilities: abilityScores },
  { kind: 'saveList', key: 'saves', section: 'saves', saves: savingThrows },
  { kind: 'skillList', key: 'skills', section: 'skills', max: 2, skills },
  {
    kind: 'conditions',
    key: 'conditions',
    conditions: conditionPaths.map((path) => ({ path })),
    exhaustion: { level: 'exhaustion.level', max: 6, penalty: 'exhaustion.penalty' },
  },
  {
    kind: 'pips',
    key: 'deathSavesSuccesses',
    count: 'status.deathSaves.successes',
    max: 3,
    heading: 'preset.deathSaves.title',
    label: 'preset.deathSaves.successes',
    critical: true,
  },
  {
    kind: 'pips',
    key: 'deathSavesFailures',
    count: 'status.deathSaves.failures',
    max: 3,
    label: 'preset.deathSaves.failures',
    critical: true,
  },
  {
    kind: 'pips',
    key: 'hitDicePrimary',
    count: 'hitDice.primary.spent',
    max: 'hitDice.primary.count',
  },
  ...hitDicePieces,
  {
    kind: 'spellSlots',
    key: 'spellSlots',
    levels: [1, 2, 3, 4, 5, 6, 7, 8, 9].map((level) => ({
      level,
      count: `spellcasting.slot${level}.spent`,
      max: `spellcasting.slot${level}.total`,
    })),
    pact: { count: 'spellcasting.pact.used', max: 'spellcasting.pact.count' },
  },
  { kind: 'recordTable', key: 'attacks', list: 'attacks.list' },
  { kind: 'recordTable', key: 'inventory', list: 'inventory.items' },
  { kind: 'recordTable', key: 'features', list: 'features.list' },
  { kind: 'recordTable', key: 'spells', list: 'spells.list' },
  {
    kind: 'recordTable',
    key: 'resources',
    list: 'resources.list',
    playColumns: ['used'],
  },
  { kind: 'resetBar', key: 'rests', actions: ['long_rest', 'short_rest'] },
  { kind: 'rollMode', key: 'rollMode' },
  ...setupPieces,
];

export const fifthCharacterConfig: PresetConfig = {
  templateId: 'fifth_character_v1',
  header: { name: 'identity.name', subtitle: 'identity.class', trailing: 'rollMode' },
  pieces,
  layout: [
    { sections: ['identity'] },
    {
      columns: [
        ['vitals', 'hitPoints'],
        [
          'deathSavesSuccesses',
          'deathSavesFailures',
          'hitDicePrimary',
          'hitDiceD6',
          'hitDiceD8',
          'hitDiceD10',
          'hitDiceD12',
          'conditions',
        ],
      ],
    },
    { sections: ['armorSetup', 'movementSetup', 'hitPointsSetup'] },
    {
      columns: [
        ['abilities', 'saves', 'savesSetup'],
        ['skills', 'skillsSetup'],
      ],
    },
    {
      sections: [
        'attacks',
        'inventory',
        'coins',
        'features',
        'spellSlots',
        'spellcastingSetup',
        'spells',
        'resources',
      ],
    },
    { sections: ['rests', 'diceFormulas'] },
  ],
  sectionNav: {
    after: 'identity',
    targets: [
      { label: 'preset.nav.abilities', sections: ['abilities'] },
      {
        label: 'preset.nav.combat',
        sections: [
          'attacks',
          'conditions',
          'deathSavesSuccesses',
          'deathSavesFailures',
          'hitPoints',
        ],
      },
      { label: 'preset.nav.items', sections: ['inventory'] },
      { label: 'preset.nav.spells', sections: ['spellSlots', 'spells'] },
      { label: 'preset.nav.resources', sections: ['features', 'resources'] },
    ],
  },
  narrowLayout: [
    {
      sections: [
        'hitPoints',
        'deathSavesSuccesses',
        'deathSavesFailures',
        'hitDicePrimary',
        'hitDiceD6',
        'hitDiceD8',
        'hitDiceD10',
        'hitDiceD12',
        'conditions',
        'vitals',
        'armorSetup',
        'movementSetup',
        'hitPointsSetup',
        'identity',
        'abilities',
        'saves',
        'savesSetup',
        'skills',
        'skillsSetup',
        'attacks',
        'inventory',
        'coins',
        'features',
        'spellSlots',
        'spellcastingSetup',
        'spells',
        'resources',
        'rests',
        'diceFormulas',
      ],
    },
  ],
};
