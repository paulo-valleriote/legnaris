/**
 * Which Dice expression applies to a Row (ADR 0005). Resolution is
 * specificity-first with "no dice" as the one exception:
 *
 * 1. the player's expression for the Row;
 * 2. the player's "no dice" for the Row's Section (a kill switch that also
 *    silences Template Row defaults);
 * 3. the Template's expression for the Row;
 * 4. the player's expression for the Section;
 * 5. the Template's expression for the Section.
 *
 * A player's `null` for the Row is "no dice" and wins over everything. A key
 * that is absent falls through to the next step. The chosen expression is then
 * judged against the Row: if it does not parse or fit, the Row is not rollable
 * with it and no later step is tried. Pure; mirrors `DiceModel.effective` of
 * the LGN-161 prototype.
 */

import type { DiceError } from './errors';
import { validateRow, type DiceSchema } from './fit';
import type { Expression } from './parse';

/** Dice defaults the Template gives (`SectionDice` / `RowDice`), keyed by Section key / Row key. */
export interface DiceDefaults {
  readonly sectionDice?: Readonly<Record<string, string>>;
  readonly rowDice?: Readonly<Record<string, string>>;
}

/** The player's dice choices stored on the Sheet: an expression replaces or adds, `null` is "no dice". */
export interface DiceChoices {
  readonly row?: Readonly<Record<string, string | null>>;
  readonly section?: Readonly<Record<string, string | null>>;
}

/** Where the applied expression came from, so the UI can say why a Row rolls (or does not). */
export type DiceSource =
  | 'sheet-row'
  | 'sheet-row-off'
  | 'sheet-section-off'
  | 'template-row'
  | 'sheet-section'
  | 'template-section'
  | 'none';

export type Resolution =
  | {
      readonly source: DiceSource;
      readonly expression: string;
      readonly rollable: true;
      readonly parsed: Expression;
    }
  | {
      readonly source: DiceSource;
      readonly expression: string | null;
      readonly rollable: false;
      readonly reason: DiceError;
    };

export interface ResolveInput {
  readonly rowKey: string;
  readonly sectionKey: string;
  readonly schema: DiceSchema;
  readonly template: DiceDefaults;
  readonly choices: DiceChoices;
}

/** Reads an own entry; an absent key and a key holding `undefined` are both "not chosen". */
function ownEntry<T>(record: Readonly<Record<string, T>> | undefined, key: string): T | undefined {
  return record !== undefined && Object.hasOwn(record, key) ? record[key] : undefined;
}

function judgeAgainstRow(source: DiceSource, expression: string, input: ResolveInput): Resolution {
  const validated = validateRow(input.schema, input.rowKey, expression);
  if (!validated.ok) {
    return { source, expression, rollable: false, reason: validated.error };
  }
  return { source, expression, rollable: true, parsed: validated.value };
}

function turnedOff(source: DiceSource, where: string): Resolution {
  return {
    source,
    expression: null,
    rollable: false,
    reason: { class: 'turned_off', message: `rolling is turned off for this ${where}` },
  };
}

/** Resolves the expression that applies to [input.rowKey] and whether it can be rolled there. */
export function resolveDice(input: ResolveInput): Resolution {
  const { rowKey, sectionKey, template, choices } = input;

  const playerRow = ownEntry(choices.row, rowKey);
  if (playerRow === null) {
    return turnedOff('sheet-row-off', 'Row');
  }
  if (playerRow !== undefined) {
    return judgeAgainstRow('sheet-row', playerRow, input);
  }

  const playerSection = ownEntry(choices.section, sectionKey);
  if (playerSection === null) {
    return turnedOff('sheet-section-off', 'Section');
  }

  const templateRow = ownEntry(template.rowDice, rowKey);
  if (templateRow !== undefined) {
    return judgeAgainstRow('template-row', templateRow, input);
  }
  if (playerSection !== undefined) {
    return judgeAgainstRow('sheet-section', playerSection, input);
  }
  const templateSection = ownEntry(template.sectionDice, sectionKey);
  if (templateSection !== undefined) {
    return judgeAgainstRow('template-section', templateSection, input);
  }

  return {
    source: 'none',
    expression: null,
    rollable: false,
    reason: { class: 'no_dice', message: 'no dice are defined for this Row' },
  };
}
