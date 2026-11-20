import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { isPlainObject } from '../api/parse';
import { setPathValue, seedFromSheet } from '../engine/sheet_defaults';
import {
  ABILITY_KEYS,
  abilityModifierPath,
  abilityScorePath,
  type AbilityKey,
} from '../sheet/preset/fantasy/abilities_data';
import {
  ARMOR_ABILITIES,
  ARMOR_CLASS_PATH,
  BASE_ARMOR_PATH,
  armorAbilityFromValues,
  type ArmorAbilityKey,
} from '../sheet/preset/fantasy/armor_class';
import {
  skillAbility,
  skillBonusPath,
  skillProficiencyPath,
  skillTotalPath,
  SKILL_KEYS,
  saveProficiencyPath,
  saveTotalPath,
  type SkillKey,
} from '../sheet/preset/fantasy/saves_skills_data';
import type { DiceTarget, SheetEditState, SheetEditStore } from '../sheet/sheet_edit';
import {
  canPreview,
  canSave,
  createRollingWindow,
  editableLeafPaths,
  isDicePending,
} from '../sheet/sheet_edit';
import { castingConfigPath } from './demo_character';

const EMPTY_DICE = { section: {}, row: {} };

const DEFAULT_MODIFIER_BASE = 10;
const DEFAULT_MODIFIER_STEP = 2;
const DEFAULT_PASSIVE_BASE = 10;
const DEFAULT_SAVE_BASE = 8;
const DEFAULT_ATTRIBUTE = 10;
const DEFAULT_DEFENSE = 10;

function readPath(data: Record<string, unknown>, path: string): unknown {
  try {
    return valueAtPath(data, path);
  } catch {
    return undefined;
  }
}

function numberAt(data: Record<string, unknown>, path: string, fallback: number): number {
  const value = readPath(data, path);
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function isOn(data: Record<string, unknown>, path: string): boolean {
  return numberAt(data, path, 0) !== 0;
}

const ARMOR_ABILITY: Readonly<Record<ArmorAbilityKey, AbilityKey>> = {
  dex: 'dexterity',
  con: 'constitution',
};

function cloneTree(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(cloneTree);
  }
  if (isPlainObject(value)) {
    const copy: Record<string, unknown> = {};
    for (const [key, child] of Object.entries(value)) {
      copy[key] = cloneTree(child);
    }
    return copy;
  }
  return value;
}

function modifiersFrom(data: Record<string, unknown>): Record<AbilityKey, number> {
  const base = numberAt(data, 'rules.modifierBase', DEFAULT_MODIFIER_BASE);
  const rawStep = numberAt(data, 'rules.modifierStep', DEFAULT_MODIFIER_STEP);
  const step = rawStep === 0 ? DEFAULT_MODIFIER_STEP : rawStep;
  const modifiers = {} as Record<AbilityKey, number>;
  for (const ability of ABILITY_KEYS) {
    const score = numberAt(data, abilityScorePath(ability), DEFAULT_ATTRIBUTE);
    modifiers[ability] = Math.floor((score - base) / step);
  }
  return modifiers;
}

function skillTotal(
  data: Record<string, unknown>,
  skill: SkillKey,
  modifiers: Readonly<Record<AbilityKey, number>>,
  proficiency: number,
): number {
  const bonus = numberAt(data, skillBonusPath(skill), 0);
  const proficient = isOn(data, skillProficiencyPath(skill)) ? proficiency : 0;
  return modifiers[skillAbility(skill)] + bonus + proficient;
}

export function deriveDemoValues(data: Record<string, unknown>): Map<string, number> {
  const derived = new Map<string, number>();
  const modifiers = modifiersFrom(data);
  const proficiency = numberAt(data, 'identity.proficiencyBonus', 0);

  for (const ability of ABILITY_KEYS) {
    derived.set(abilityModifierPath(ability), modifiers[ability]);
    const proficient = isOn(data, saveProficiencyPath(ability)) ? proficiency : 0;
    derived.set(saveTotalPath(ability), modifiers[ability] + proficient);
  }

  for (const skill of SKILL_KEYS) {
    derived.set(skillTotalPath(skill), skillTotal(data, skill, modifiers, proficiency));
  }

  const active = armorAbilityFromValues({
    dex: numberAt(data, ARMOR_ABILITIES.dex.configPath, 0),
    con: numberAt(data, ARMOR_ABILITIES.con.configPath, 0),
  });
  const defense = numberAt(data, BASE_ARMOR_PATH, DEFAULT_DEFENSE);
  const armor = active === undefined ? 0 : modifiers[ARMOR_ABILITY[active]];
  derived.set(ARMOR_CLASS_PATH, defense + armor);

  const passiveBase = numberAt(data, 'rules.passiveBase', DEFAULT_PASSIVE_BASE);
  const perception = skillTotal(data, 'perception', modifiers, proficiency);
  derived.set('derived.passiveAwareness', passiveBase + perception);

  const casting = ABILITY_KEYS.find((ability) => isOn(data, castingConfigPath(ability)));
  if (casting !== undefined) {
    derived.set('spells.attackBonus', proficiency + modifiers[casting]);
    derived.set(
      'spells.saveDC',
      numberAt(data, 'rules.spellSaveBase', DEFAULT_SAVE_BASE) + proficiency + modifiers[casting],
    );
  }

  return derived;
}

function applyDerived(data: Record<string, unknown>): void {
  for (const [path, value] of deriveDemoValues(data)) {
    setPathValue(data, path, value);
  }
}

function withDerived(seed: Record<string, unknown>): Record<string, unknown> {
  const derived = cloneTree(seed) as Record<string, unknown>;
  applyDerived(derived);
  return derived;
}

export function createDemoEditStore(template: TemplateDetail, sheet: Sheet): SheetEditStore {
  const editable = editableLeafPaths(template.fields);
  const seeded = withDerived(seedFromSheet(template.fields, sheet.data));
  const seededDice = sheet.dice ?? EMPTY_DICE;
  const saveWindow = createRollingWindow(() => Date.now(), Number.POSITIVE_INFINITY);
  const previewWindow = createRollingWindow(() => Date.now(), Number.POSITIVE_INFINITY);

  let state: SheetEditState = {
    defaults: seeded,
    data: cloneTree(seeded) as Record<string, unknown>,
    editedPaths: new Set<string>(),
    savedDice: seededDice,
    dice: seededDice,
    isSaving: false,
    saveError: undefined,
    revision: 0,
    previewedRevision: undefined,
    previewMarks: undefined,
    isPreviewing: false,
    previewError: undefined,
    saveWindow,
    previewWindow,
  };

  const listeners = new Set<() => void>();
  const pathListeners = new Map<string, Set<() => void>>();
  const marksListeners = new Set<() => void>();

  const commit = (next: SheetEditState): void => {
    const previous = state;
    state = next;
    for (const [path, pathSubscribers] of pathListeners) {
      if (!Object.is(readPath(previous.data, path), readPath(next.data, path))) {
        for (const listener of [...pathSubscribers]) {
          listener();
        }
      }
    }
    if (previous.previewMarks !== next.previewMarks) {
      for (const listener of [...marksListeners]) {
        listener();
      }
    }
    for (const listener of [...listeners]) {
      listener();
    }
  };

  const applyWrites = (writes: ReadonlyMap<string, unknown>): void => {
    const effective = new Map<string, unknown>();
    for (const [path, value] of writes) {
      if (!editable.has(path) || Object.is(readPath(state.data, path), value)) {
        continue;
      }
      effective.set(path, value);
    }
    if (effective.size === 0) {
      return;
    }

    const data = cloneTree(state.data) as Record<string, unknown>;
    const editedPaths = new Set(state.editedPaths);
    for (const [path, value] of effective) {
      setPathValue(data, path, value);
      editedPaths.add(path);
    }
    applyDerived(data);

    commit({
      ...state,
      data,
      editedPaths,
      revision: state.revision + 1,
      previewMarks: undefined,
    });
  };

  const pendingDelta = (current: SheetEditState): Record<string, unknown> => {
    const delta: Record<string, unknown> = {};
    for (const path of current.editedPaths) {
      if (
        editable.has(path) &&
        !Object.is(readPath(current.data, path), readPath(current.defaults, path))
      ) {
        setPathValue(delta, path, readPath(current.data, path));
      }
    }
    return delta;
  };

  const pendingDice = (current: SheetEditState) =>
    isDicePending(current) ? current.dice : undefined;

  const withDiceEntry = (
    overrides: Readonly<Record<string, string | null>>,
    key: string,
    expression: string | null | undefined,
  ): Record<string, string | null> => {
    const next = { ...overrides };
    if (expression === undefined) {
      delete next[key];
    } else {
      next[key] = expression;
    }
    return next;
  };

  const sameDiceEntry = (
    overrides: Readonly<Record<string, string | null>>,
    key: string,
    expression: string | null | undefined,
  ): boolean =>
    expression === undefined
      ? !Object.hasOwn(overrides, key)
      : Object.hasOwn(overrides, key) && overrides[key] === expression;

  return {
    getState: () => state,
    getPendingDelta: () => pendingDelta(state),
    getPendingDice: () => pendingDice(state),

    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },

    subscribePath: (path, listener) => {
      let subscribers = pathListeners.get(path);
      if (subscribers === undefined) {
        subscribers = new Set();
        pathListeners.set(path, subscribers);
      }
      subscribers.add(listener);
      return () => {
        subscribers.delete(listener);
        if (subscribers.size === 0) {
          pathListeners.delete(path);
        }
      };
    },

    subscribeMarks: (listener) => {
      marksListeners.add(listener);
      return () => {
        marksListeners.delete(listener);
      };
    },

    setPath: (path, value) => {
      applyWrites(new Map([[path, value]]));
    },

    setExclusive: (selected, siblings) => {
      const writes = new Map<string, unknown>();
      for (const sibling of siblings) {
        writes.set(sibling, sibling === selected ? 1 : 0);
      }
      applyWrites(writes);
    },

    setDice: (target: DiceTarget, key, expression) => {
      const current = target === 'row' ? state.dice.row : state.dice.section;
      if (sameDiceEntry(current, key, expression)) {
        return;
      }
      const next = withDiceEntry(current, key, expression);
      commit({
        ...state,
        dice: target === 'row' ? { ...state.dice, row: next } : { ...state.dice, section: next },
      });
    },

    save: async () => {
      if (!canSave(state)) {
        return;
      }
      saveWindow.record();
      commit({
        ...state,
        defaults: cloneTree(state.data) as Record<string, unknown>,
        editedPaths: new Set<string>(),
        savedDice: state.dice,
        isSaving: false,
        saveError: undefined,
        saveErrorKind: undefined,
        previewMarks: undefined,
      });
    },

    preview: async () => {
      if (!canPreview(state)) {
        return;
      }
      previewWindow.record();
      commit({
        ...state,
        isPreviewing: false,
        previewedRevision: state.revision,
        previewError: undefined,
        previewErrorKind: undefined,
      });
    },

    restore: async () => undefined,

    discard: () => {
      commit({
        ...state,
        data: cloneTree(state.defaults) as Record<string, unknown>,
        editedPaths: new Set<string>(),
        dice: state.savedDice,
        revision: state.revision + 1,
        previewedRevision: undefined,
        previewMarks: undefined,
        saveError: undefined,
        saveErrorKind: undefined,
        previewError: undefined,
        previewErrorKind: undefined,
      });
    },

    dispose: () => undefined,
  };
}
