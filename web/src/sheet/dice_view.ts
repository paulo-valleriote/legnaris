import type { DiceError, DiceErrorClass } from '../dice/errors';
import { referenceLabel, type Expression } from '../dice/parse';
import {
  schemaFromKinds,
  validateRow,
  validateSection,
  type DiceSchema,
  type FieldKind,
} from '../dice/fit';
import {
  resolveDice,
  type DiceChoices,
  type DiceDefaults as ResolveDiceDefaults,
  type Resolution,
} from '../dice/resolve';
import type { DiceBlock, DiceDefaults as TemplateDiceDefaults } from '../api/models/dice';
import type { FieldNode } from '../api/models/template';
import { buildSections, rowKey } from '../engine/field_grouping';
import {
  listInfos,
  recordFieldSchema,
  recordRowID,
  recordRowKey,
  SYNTHETIC_RECORD_ID,
} from '../engine/row_identity';
import type { MessageKey } from '../l10n/strings';

function kindOf(type: string): FieldKind {
  if (type === 'number') {
    return 'number';
  }
  return type === 'bool' ? 'bool' : 'text';
}

function collectKinds(node: FieldNode, path: string, into: Record<string, FieldKind>): void {
  if (node.kind === 'leaf') {
    into[path] = kindOf(node.schema.type);
    return;
  }
  if (node.kind === 'list') {
    return;
  }
  for (const [key, child] of Object.entries(node.children)) {
    collectKinds(child, `${path}.${key}`, into);
  }
}

export function diceSchemaFromFields(fields: Readonly<Record<string, FieldNode>>): DiceSchema {
  const kinds: Record<string, FieldKind> = {};
  for (const [key, node] of Object.entries(fields)) {
    collectKinds(node, key, kinds);
  }
  const staticSchema = schemaFromKinds(kinds);
  return (path) => {
    const kind = staticSchema(path);
    if (kind !== undefined) {
      return kind;
    }
    const schema = recordFieldSchema(fields, path);
    return schema === undefined ? undefined : kindOf(schema.type);
  };
}

export function checkExpression(
  text: string,
  schema: DiceSchema,
  rowKey: string,
): DiceError | undefined {
  const validated = validateRow(schema, rowKey, text);
  return validated.ok ? undefined : validated.error;
}

export function checkSectionExpression(
  text: string,
  schema: DiceSchema,
  rows: readonly string[],
): DiceError | undefined {
  const validated = validateSection(schema, rows, text);
  return validated.ok ? undefined : validated.error;
}

export function diceSectionRows(
  fields: Readonly<Record<string, FieldNode>>,
): ReadonlyMap<string, readonly string[]> {
  const rows = new Map(
    buildSections({ ...fields }).map((section) => [section.key, section.rows.map(rowKey)]),
  );
  for (const list of listInfos(fields)) {
    rows.set(list.section, [
      ...(rows.get(list.section) ?? []),
      recordRowKey(list.path, SYNTHETIC_RECORD_ID),
    ]);
  }
  return rows;
}

export interface PruneContext {
  readonly rowKeys: ReadonlySet<string>;
  readonly sectionRows: ReadonlyMap<string, readonly string[]>;
  readonly schema: DiceSchema;
  readonly isRowKey?: (key: string) => boolean;
}

function keepEntries(
  record: Readonly<Record<string, string | null>>,
  isKnown: (key: string) => boolean,
  fits: (key: string, expression: string) => boolean,
): Record<string, string | null> {
  return Object.fromEntries(
    Object.entries(record).filter(
      ([key, expression]) => isKnown(key) && (expression === null || fits(key, expression)),
    ),
  );
}

export function pruneStaleChoices(choices: DiceChoices, context: PruneContext): DiceBlock {
  const { rowKeys, sectionRows, schema, isRowKey } = context;
  return {
    row: keepEntries(
      choices.row ?? {},
      (key) => rowKeys.has(key) || isRowKey?.(key) === true,
      (key, expression) => validateRow(schema, key, expression).ok,
    ),
    section: keepEntries(
      choices.section ?? {},
      (key) => sectionRows.has(key),
      (key, expression) => validateSection(schema, sectionRows.get(key) ?? [], expression).ok,
    ),
  };
}

const ERROR_KEYS: Record<DiceErrorClass, MessageKey> = {
  empty: 'dice.error.empty',
  too_long: 'dice.error.tooLong',
  too_many_terms: 'dice.error.tooManyTerms',
  dice_count: 'dice.error.diceCount',
  dice_faces: 'dice.error.diceFaces',
  number_range: 'dice.error.numberRange',
  missing_term: 'dice.error.missingTerm',
  unreadable_term: 'dice.error.unreadableTerm',
  unknown_field: 'dice.error.unknownField',
  non_numeric_field: 'dice.error.nonNumericField',
  no_rows: 'dice.error.noRows',
  no_value: 'dice.error.noValue',
  overflow: 'dice.error.overflow',
  no_dice: 'dice.error.noDice',
  turned_off: 'dice.error.turnedOff',
};

export function diceErrorMessageKey(errorClass: DiceErrorClass): MessageKey {
  return ERROR_KEYS[errorClass];
}

export interface RowResolveInput {
  readonly rowKey: string;
  readonly sectionKey: string;
  readonly schema: DiceSchema;
  readonly rowKeys: ReadonlySet<string>;
  readonly sectionRows: ReadonlyMap<string, readonly string[]>;
  readonly isRowKey?: (key: string) => boolean;
  readonly defaults: ResolveDiceDefaults;
  readonly block: DiceBlock;
}

export function resolveRowForSheet(input: RowResolveInput): Resolution {
  const choices = pruneStaleChoices(input.block, {
    rowKeys: input.rowKeys,
    sectionRows: input.sectionRows,
    schema: input.schema,
    isRowKey: input.isRowKey,
  });
  return resolveDice({
    rowKey: input.rowKey,
    sectionKey: input.sectionKey,
    schema: input.schema,
    template: input.defaults,
    choices,
  });
}

export function templateDiceDefaults(
  dice: TemplateDiceDefaults | undefined,
  fields?: Readonly<Record<string, FieldNode>>,
): ResolveDiceDefaults {
  const listDice: Record<string, string> = {};
  if (fields !== undefined) {
    for (const list of listInfos(fields)) {
      if (list.dice !== undefined) {
        listDice[list.path] = list.dice;
      }
    }
  }
  return {
    sectionDice: dice?.section,
    rowDice: dice?.row,
    ...(Object.keys(listDice).length === 0 ? {} : { listDice }),
  };
}

export interface DiceMarking {
  readonly section: ReadonlySet<string>;
  readonly row: ReadonlySet<string>;
  readonly lists: ReadonlySet<string>;
}

const EMPTY_MARKING: DiceMarking = { section: new Set(), row: new Set(), lists: new Set() };

export function templateDiceMarking(
  dice: TemplateDiceDefaults | undefined,
  fields?: Readonly<Record<string, FieldNode>>,
): DiceMarking {
  const lists = new Set(
    (fields === undefined ? [] : listInfos(fields))
      .filter((list) => list.notD20)
      .map((list) => list.path),
  );
  if (dice?.notD20 === undefined && lists.size === 0) {
    return EMPTY_MARKING;
  }
  return {
    section: new Set(dice?.notD20?.section ?? []),
    row: new Set(dice?.notD20?.row ?? []),
    lists,
  };
}

function isMarkedListRow(marking: DiceMarking, rowKey: string): boolean {
  for (const listPath of marking.lists) {
    if (recordRowID(listPath, rowKey) !== undefined) {
      return true;
    }
  }
  return false;
}

export function isNotD20Test(marking: DiceMarking, rowKey: string, sectionKey: string): boolean {
  return (
    marking.row.has(rowKey) || marking.section.has(sectionKey) || isMarkedListRow(marking, rowKey)
  );
}

const SUMMARY_MAX_DICE = 2;

export function diceSummary(expression: Expression): string | undefined {
  const dice = expression.terms.filter((term) => term.kind === 'dice');
  if (dice.length === 0) {
    return undefined;
  }
  const named = dice.slice(0, SUMMARY_MAX_DICE).map((term, index) => {
    const sign = term.sign === -1 ? '-' : index === 0 ? '' : '+';
    const count =
      term.countRef !== undefined
        ? `{${referenceLabel(term.countRef)}}`
        : term.count === 1
          ? ''
          : String(term.count);
    const faces =
      term.facesRef !== undefined ? `{${referenceLabel(term.facesRef)}}` : String(term.faces);
    return `${sign}${count}d${faces}`;
  });
  return dice.length > SUMMARY_MAX_DICE ? `${named.join('')}+…` : named.join('');
}
