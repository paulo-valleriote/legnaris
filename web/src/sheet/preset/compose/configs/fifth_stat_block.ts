import type { PresetConfig } from '../types';

const abilities = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'];

const skills = [
  { name: 'acrobatics', ability: 'DEX' },
  { name: 'animalHandling', ability: 'WIS' },
  { name: 'arcana', ability: 'INT' },
  { name: 'athletics', ability: 'STR' },
  { name: 'deception', ability: 'CHA' },
  { name: 'history', ability: 'INT' },
  { name: 'insight', ability: 'WIS' },
  { name: 'intimidation', ability: 'CHA' },
  { name: 'investigation', ability: 'INT' },
  { name: 'medicine', ability: 'WIS' },
  { name: 'nature', ability: 'INT' },
  { name: 'perception', ability: 'WIS' },
  { name: 'performance', ability: 'CHA' },
  { name: 'persuasion', ability: 'CHA' },
  { name: 'religion', ability: 'INT' },
  { name: 'sleightOfHand', ability: 'DEX' },
  { name: 'stealth', ability: 'DEX' },
  { name: 'survival', ability: 'WIS' },
];

export const fifthStatBlockConfig: PresetConfig = {
  templateId: 'fifth_stat_block_v1',
  header: { name: 'identity.name' },
  pieces: [
    {
      kind: 'statPills',
      key: 'identity',
      pills: [{ path: 'identity.challengeRating' }, { path: 'experience.xp' }],
    },
    {
      kind: 'statPills',
      key: 'combat',
      pills: [
        { path: 'armor.armorClass' },
        { path: 'initiative.score' },
        { path: 'speed.walkTotal' },
        { path: 'speed.flyTotal' },
        { path: 'speed.swimTotal' },
        { path: 'speed.climbTotal' },
        { path: 'speed.burrowTotal' },
        { path: 'identity.proficiencyBonus' },
        { path: 'passive.perception' },
      ],
    },
    {
      kind: 'hitPoints',
      key: 'hitPoints',
      current: 'hitPoints.current',
      max: 'hitPoints.maximum',
      temp: 'hitPoints.temp',
    },
    {
      kind: 'abilityGrid',
      key: 'abilities',
      section: 'abilities',
      abilities: abilities.map((ability) => ({
        score: `abilities.${ability}.score`,
        modifier: `abilities.${ability}.mod`,
      })),
    },
    {
      kind: 'saveList',
      key: 'saves',
      section: 'saves',
      saves: abilities.map((ability) => ({
        proficiency: `saves.${ability}.prof`,
        total: `saves.${ability}.total`,
      })),
    },
    {
      kind: 'skillList',
      key: 'skills',
      section: 'skills',
      max: 2,
      skills: skills.map((skill) => ({
        rank: `skills.${skill.name}.rank`,
        total: `skills.${skill.name}.total`,
        ability: skill.ability,
      })),
    },
    {
      kind: 'conditions',
      key: 'conditions',
      conditions: [],
      exhaustion: {
        level: 'exhaustion.level',
        max: 6,
        penalty: 'exhaustion.penalty',
      },
    },
    {
      kind: 'textFields',
      key: 'details',
      fields: [
        { path: 'details.resistances' },
        { path: 'details.immunities' },
        { path: 'details.senses' },
        { path: 'details.languages' },
      ],
    },
    { kind: 'recordTable', key: 'traits', list: 'traits', columns: ['name', 'text'] },
    {
      kind: 'recordTable',
      key: 'actions',
      list: 'actions',
      columns: ['name', 'toHit', 'damage', 'notes'],
    },
    {
      kind: 'recordTable',
      key: 'bonusActions',
      list: 'bonusActions',
      columns: ['name', 'text'],
    },
    { kind: 'recordTable', key: 'reactions', list: 'reactions', columns: ['name', 'text'] },
    {
      kind: 'recordTable',
      key: 'legendaryActions',
      list: 'legendaryActions',
      columns: ['name', 'text'],
    },
    {
      kind: 'recordTable',
      key: 'resources',
      list: 'resources',
      columns: ['name', 'max', 'remaining', 'recharge'],
    },
    { kind: 'rollMode', key: 'rollMode' },
  ],
  layout: [
    { sections: ['identity', 'combat', 'hitPoints', 'conditions'] },
    { columns: [['abilities', 'saves'], ['skills']] },
    {
      sections: [
        'details',
        'traits',
        'actions',
        'bonusActions',
        'reactions',
        'legendaryActions',
        'resources',
        'rollMode',
      ],
    },
  ],
  narrowLayout: [
    {
      sections: [
        'identity',
        'combat',
        'hitPoints',
        'abilities',
        'saves',
        'skills',
        'conditions',
        'details',
        'traits',
        'actions',
        'bonusActions',
        'reactions',
        'legendaryActions',
        'resources',
        'rollMode',
      ],
    },
  ],
};
