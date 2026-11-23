import type { DiceError } from './errors';
import { validateRow, type DiceSchema } from './fit';
import type { Expression } from './parse';

export interface DiceDefaults {
  readonly sectionDice?: Readonly<Record<string, string>>;
  readonly rowDice?: Readonly<Record<string, string>>;
  readonly listDice?: Readonly<Record<string, string>>;
}

export interface DiceChoices {
  readonly row?: Readonly<Record<string, string | null>>;
  readonly section?: Readonly<Record<string, string | null>>;
}

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

function ownEntry<T>(record: Readonly<Record<string, T>> | undefined, key: string): T | undefined {
  return record !== undefined && Object.hasOwn(record, key) ? record[key] : undefined;
}

const RECORD_ID = /^r[0-9a-f]{32}$/;

function listDefault(
  listDice: Readonly<Record<string, string>> | undefined,
  rowKey: string,
): string | undefined {
  if (listDice === undefined) {
    return undefined;
  }
  for (const [listPath, expression] of Object.entries(listDice)) {
    const prefix = `${listPath}.`;
    if (rowKey.startsWith(prefix) && RECORD_ID.test(rowKey.slice(prefix.length))) {
      return expression;
    }
  }
  return undefined;
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
  const templateList = listDefault(template.listDice, rowKey);
  if (templateList !== undefined) {
    return judgeAgainstRow('template-row', templateList, input);
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
