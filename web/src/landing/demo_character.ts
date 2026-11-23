import type { Sheet } from '../api/models/sheet';
import type { FieldNode, TemplateDetail } from '../api/models/template';
import {
  ABILITY_KEYS,
  abilityModifierPath,
  abilityScorePath,
  type AbilityKey,
} from '../sheet/preset/fantasy/abilities_data';
import { INITIATIVE_PATH } from '../sheet/preset/fantasy/armor_class';
import {
  skillProficiencyPath,
  SKILL_KEYS,
  saveProficiencyPath,
  type SkillKey,
} from '../sheet/preset/fantasy/saves_skills_data';

export const DEMO_TEMPLATE_ID = 'demo_v1';

export const DEMO_SHEET_ID = '00000000-0000-7000-8000-000000000347';

const ABILITY_LABELS: Readonly<Record<AbilityKey, string>> = {
  strength: 'Might',
  dexterity: 'Agility',
  constitution: 'Vigor',
  intelligence: 'Wits',
  wisdom: 'Instinct',
  charisma: 'Presence',
};

const SKILL_LABELS: Readonly<Record<SkillKey, string>> = {
  athletics: 'Climbing',
  acrobatics: 'Balance',
  stealth: 'Concealment',
  perception: 'Awareness',
  insight: 'Empathy',
  survival: 'Fieldcraft',
  persuasion: 'Rapport',
  deception: 'Guile',
  intimidation: 'Menace',
  knowledge: 'Lore',
  analysis: 'Inquiry',
  craft: 'Crafting',
};

const ATTACK_SLOTS = [1, 2, 3, 4] as const;

const SLOT_LEVELS = [1, 2, 3] as const;

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function keyOf(path: string): string {
  return path.slice(path.lastIndexOf('.') + 1);
}

export function spellUsesConfigPath(ability: AbilityKey): string {
  return `config.spellUses${capitalize(ability)}`;
}

function numberLeaf(
  label: string,
  defaultValue: number,
  options: {
    readonly editable?: boolean;
    readonly exclusiveGroup?: string;
    readonly optionLabel?: string;
    readonly columnLabel?: string;
  } = {},
): FieldNode {
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

function textLeaf(label: string, options: { readonly columnLabel?: string } = {}): FieldNode {
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

function group(label: string, children: Record<string, FieldNode>): FieldNode {
  return { kind: 'group', label, children };
}

function tableGroup(
  label: string,
  columns: readonly string[],
  children: Record<string, FieldNode>,
): FieldNode {
  return { kind: 'group', label, columns, children };
}

function abilityNodes(editable: boolean): Record<string, FieldNode> {
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [ability, numberLeaf(ABILITY_LABELS[ability], 10, { editable })]),
  );
}

function modifierNodes(): Record<string, FieldNode> {
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      keyOf(abilityModifierPath(ability)),
      numberLeaf(`${ABILITY_LABELS[ability]} modifier`, 0, { editable: false }),
    ]),
  );
}

function saveNodes(): Record<string, FieldNode> {
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      ability,
      numberLeaf(`${ABILITY_LABELS[ability]} save`, 0, { editable: false }),
    ]),
  );
}

function saveProficiencyNodes(): Record<string, FieldNode> {
  return Object.fromEntries(
    ABILITY_KEYS.map((ability) => [
      keyOf(saveProficiencyPath(ability)),
      numberLeaf(`${ABILITY_LABELS[ability]} save proficiency`, 0),
    ]),
  );
}

function skillNodes(): Record<string, FieldNode> {
  return Object.fromEntries(
    SKILL_KEYS.map((skill) => [
      skill,
      tableGroup(SKILL_LABELS[skill], ['bonus', 'total'], {
        bonus: numberLeaf(`${SKILL_LABELS[skill]} bonus`, 0, { columnLabel: 'Bonus' }),
        total: numberLeaf(`${SKILL_LABELS[skill]} total`, 0, {
          editable: false,
          columnLabel: 'Total',
        }),
      }),
    ]),
  );
}

function skillProficiencyNodes(): Record<string, FieldNode> {
  return Object.fromEntries(
    SKILL_KEYS.map((skill) => [
      keyOf(skillProficiencyPath(skill)),
      numberLeaf(`${SKILL_LABELS[skill]} proficiency`, 0),
    ]),
  );
}

function attackNodes(): Record<string, FieldNode> {
  return Object.fromEntries(
    ATTACK_SLOTS.map((slot) => [
      `attack${slot}`,
      tableGroup(`Attack ${slot}`, ['name', 'damage', 'bonus', 'notes'], {
        name: textLeaf('Attack name', { columnLabel: 'Name' }),
        damage: textLeaf('Damage', { columnLabel: 'Damage' }),
        bonus: numberLeaf('Attack bonus', 0, { columnLabel: 'Bonus' }),
        notes: textLeaf('Attack notes', { columnLabel: 'Notes' }),
      }),
    ]),
  );
}

function slotNodes(): Record<string, FieldNode> {
  return Object.fromEntries(
    SLOT_LEVELS.map((level) => [
      `slot${level}`,
      tableGroup(`Focus slots ${level}`, ['total', 'expended'], {
        total: numberLeaf('Focus slots total', 0, { columnLabel: 'Total' }),
        expended: numberLeaf('Focus slots spent', 0, { columnLabel: 'Spent' }),
      }),
    ]),
  );
}

function configNodes(): Record<string, FieldNode> {
  return {
    ...saveProficiencyNodes(),
    ...skillProficiencyNodes(),
    acUsesDex: numberLeaf('Defense uses agility', 1, {
      exclusiveGroup: 'armorClassAbility',
      optionLabel: ABILITY_LABELS.dexterity,
    }),
    acUsesCon: numberLeaf('Defense uses vigor', 0, {
      exclusiveGroup: 'armorClassAbility',
      optionLabel: ABILITY_LABELS.constitution,
    }),
    ...Object.fromEntries(
      ABILITY_KEYS.map((ability) => [
        keyOf(spellUsesConfigPath(ability)),
        numberLeaf(`Focus uses ${ABILITY_LABELS[ability]}`, 0, {
          exclusiveGroup: 'spellcastingAbility',
          optionLabel: ABILITY_LABELS[ability],
        }),
      ]),
    ),
  };
}

function rowDice(): Record<string, string> {
  return {
    [INITIATIVE_PATH]: '1d20 + @',
    ...Object.fromEntries(
      ABILITY_KEYS.map((ability) => [
        abilityScorePath(ability),
        `1d20 + ${abilityModifierPath(ability)}`,
      ]),
    ),
  };
}

export const DEMO_TEMPLATE: TemplateDetail = {
  id: DEMO_TEMPLATE_ID,
  name: 'Living sheet demo',
  default: false,
  kinds: ['character', 'npc', 'enemy'],
  fields: {
    identity: group('Identity', {
      characterName: textLeaf('Character name'),
      playerName: textLeaf('Player name'),
      ancestry: textLeaf('Heritage'),
      origin: textLeaf('Origin'),
      level: numberLeaf('Level', 1),
      proficiencyBonus: numberLeaf('Training bonus', 2),
      xp: numberLeaf('Experience', 0),
    }),
    stats: group('Attributes', abilityNodes(true)),
    modifiers: group('Attribute modifiers', modifierNodes()),
    derived: group('Derived', {
      armorClass: numberLeaf('Defense', 0, { editable: false }),
      passiveAwareness: numberLeaf('Passive awareness', 0, { editable: false }),
    }),
    config: group('Demo settings', configNodes()),
    combat: group('Combat', {
      baseArmorClass: numberLeaf('Base defense', 10),
      initiative: numberLeaf('Reaction', 0),
      speed: numberLeaf('Movement', 30),
      hitPoints: numberLeaf('Current health', 10),
      hitPointsMax: numberLeaf('Health maximum', 10),
      hitPointsTemp: numberLeaf('Temporary health', 0),
    }),
    saves: group('Saves', saveNodes()),
    skills: group('Skills', skillNodes()),
    attacks: group('Attacks', attackNodes()),
    spells: group('Focus', {
      attackBonus: numberLeaf('Focus attack', 0, { editable: false }),
      saveDC: numberLeaf('Focus difficulty', 0, { editable: false }),
      atWill: textLeaf('At will'),
      list: textLeaf('Known'),
      ...slotNodes(),
    }),
    rules: group('Rules', {
      modifierBase: numberLeaf('Modifier base', 10),
      modifierStep: numberLeaf('Modifier step', 2),
      passiveBase: numberLeaf('Passive base', 10),
      spellSaveBase: numberLeaf('Focus base', 8),
    }),
  },
  dice: {
    section: {
      saves: '1d20 + @',
      skills: '1d20 + @.total',
      attacks: '1d20 + @.bonus',
    },
    row: rowDice(),
  },
};

const SAMPLE_SKILLS: Readonly<Record<SkillKey, number>> = {
  athletics: 1,
  acrobatics: 0,
  stealth: 1,
  perception: 2,
  insight: 0,
  survival: 0,
  persuasion: 0,
  deception: 0,
  intimidation: 0,
  knowledge: 0,
  analysis: 0,
  craft: 0,
};

const SAMPLE_CONFIG: Readonly<Record<string, number>> = {
  ...Object.fromEntries(ABILITY_KEYS.map((ability) => [keyOf(saveProficiencyPath(ability)), 0])),
  ...Object.fromEntries(ABILITY_KEYS.map((ability) => [keyOf(spellUsesConfigPath(ability)), 0])),
  ...Object.fromEntries(SKILL_KEYS.map((skill) => [keyOf(skillProficiencyPath(skill)), 0])),
  saveDexterityProf: 1,
  skillStealthProf: 1,
  skillPerceptionProf: 1,
  acUsesDex: 1,
  acUsesCon: 0,
  spellUsesCharisma: 1,
};

export const DEMO_SHEET: Sheet = {
  id: DEMO_SHEET_ID,
  templateId: DEMO_TEMPLATE_ID,
  name: 'Rowan Vale',
  data: {
    identity: {
      characterName: 'Rowan Vale',
      playerName: 'Demo',
      ancestry: 'Wayfarer',
      origin: 'Hearthvale',
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
    combat: {
      baseArmorClass: 10,
      initiative: 3,
      speed: 30,
      hitPoints: 24,
      hitPointsMax: 30,
      hitPointsTemp: 0,
    },
    skills: Object.fromEntries(SKILL_KEYS.map((skill) => [skill, { bonus: SAMPLE_SKILLS[skill] }])),
    config: SAMPLE_CONFIG,
    attacks: {
      attack1: { name: 'Oak staff', bonus: 5, damage: '1d6 + 3', notes: 'Reach 5 ft' },
      attack2: { name: 'Throwing stone', bonus: 4, damage: '1d4 + 2', notes: '' },
      attack3: { name: '', bonus: 0, damage: '', notes: '' },
      attack4: { name: '', bonus: 0, damage: '', notes: '' },
    },
    spells: {
      atWill: 'Ember',
      list: 'Veil, Hush',
      slot1: { total: 2, expended: 0 },
      slot2: { total: 1, expended: 0 },
      slot3: { total: 0, expended: 0 },
    },
    rules: {
      modifierBase: 10,
      modifierStep: 2,
      passiveBase: 10,
      spellSaveBase: 8,
    },
  },
  dice: { section: {}, row: {} },
  createdAt: '2026-11-20T00:00:00.000Z',
  updatedAt: '2026-11-20T00:00:00.000Z',
};
