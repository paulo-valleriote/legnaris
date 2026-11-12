/**
 * The sheet's view of Dice expressions (ADR 0005): the Field schema the grammar
 * is judged against, the client-side check that mirrors the server's
 * validation of a saved Dice block (ADR 0006), the pruning of overrides the
 * Template no longer supports, and the copy of each refusal class. Pure; the network and the React tree live elsewhere.
 */

import type { DiceError, DiceErrorClass } from '../dice/errors';
import type { Expression } from '../dice/parse';
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
  for (const [key, child] of Object.entries(node.children)) {
    collectKinds(child, `${path}.${key}`, into);
  }
}

/** Builds the schema the Dice grammar is judged against from the Template's field tree. */
export function diceSchemaFromFields(fields: Readonly<Record<string, FieldNode>>): DiceSchema {
  const kinds: Record<string, FieldKind> = {};
  for (const [key, node] of Object.entries(fields)) {
    collectKinds(node, key, kinds);
  }
  return schemaFromKinds(kinds);
}

/**
 * Checks [text] for the Row keyed [rowKey] exactly as the server will, so an
 * invalid expression is refused inline without a request. `undefined` means it
 * is accepted.
 */
export function checkExpression(
  text: string,
  schema: DiceSchema,
  rowKey: string,
): DiceError | undefined {
  const validated = validateRow(schema, rowKey, text);
  return validated.ok ? undefined : validated.error;
}

/**
 * Why [text] cannot be a Section expression over [rows], or `undefined` when it
 * fits at least one of them: the same rule the server applies on save, so a
 * Section the client accepts is never refused there.
 */
export function checkSectionExpression(
  text: string,
  schema: DiceSchema,
  rows: readonly string[],
): DiceError | undefined {
  const validated = validateSection(schema, rows, text);
  return validated.ok ? undefined : validated.error;
}

/** The Row keys of each Section of the Template, keyed by Section. */
export function diceSectionRows(
  fields: Readonly<Record<string, FieldNode>>,
): ReadonlyMap<string, readonly string[]> {
  return new Map(
    buildSections({ ...fields }).map((section) => [section.key, section.rows.map(rowKey)]),
  );
}

export interface PruneContext {
  readonly rowKeys: ReadonlySet<string>;
  readonly sectionRows: ReadonlyMap<string, readonly string[]>;
  readonly schema: DiceSchema;
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

/**
 * Drops the overrides the Template no longer supports, by the rules the server
 * enforces on save: a key naming no Row or Section, a Row expression that no
 * longer fits its Row, or a Section expression that fits none of its Rows. An
 * explicit "no dice" (`null`) is a choice and survives when its key is known.
 * Dropping one falls the Row back to the Template default, never breaks the
 * Sheet, and keeps a stale entry out of the next save.
 */
export function pruneStaleChoices(choices: DiceChoices, context: PruneContext): DiceBlock {
  const { rowKeys, sectionRows, schema } = context;
  return {
    row: keepEntries(
      choices.row ?? {},
      (key) => rowKeys.has(key),
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

/** The catalogue key of the user-facing copy for a refusal class. */
export function diceErrorMessageKey(errorClass: DiceErrorClass): MessageKey {
  return ERROR_KEYS[errorClass];
}

export interface RowResolveInput {
  readonly rowKey: string;
  readonly sectionKey: string;
  readonly schema: DiceSchema;
  readonly rowKeys: ReadonlySet<string>;
  readonly sectionRows: ReadonlyMap<string, readonly string[]>;
  readonly defaults: ResolveDiceDefaults;
  readonly block: DiceBlock;
}

/**
 * Which expression applies to a Row on this sheet. Stale overrides are pruned
 * first, so a server override the Template no longer supports falls back to
 * the Template default instead of breaking the Row.
 */
export function resolveRowForSheet(input: RowResolveInput): Resolution {
  const choices = pruneStaleChoices(input.block, {
    rowKeys: input.rowKeys,
    sectionRows: input.sectionRows,
    schema: input.schema,
  });
  return resolveDice({
    rowKey: input.rowKey,
    sectionKey: input.sectionKey,
    schema: input.schema,
    template: input.defaults,
    choices,
  });
}

/** Adapts the Template's dice defaults to the resolver's shape. */
export function templateDiceDefaults(dice: TemplateDiceDefaults | undefined): ResolveDiceDefaults {
  return { sectionDice: dice?.section, rowDice: dice?.row };
}

/** The most dice terms a summary names before it stops listing them. */
const SUMMARY_MAX_DICE = 2;

/**
 * The dice an expression throws, as the Roll button shows them (`d20`, `2d6+d4`): every die term
 * in order with its sign, a count of 1 left implicit, numbers and Field references omitted since
 * the result panel lists them. A third die term and beyond collapse into `+…`. `undefined` when
 * the expression throws no dice, so the button falls back to its plain word.
 */
export function diceSummary(expression: Expression): string | undefined {
  const dice = expression.terms.filter((term) => term.kind === 'dice');
  if (dice.length === 0) {
    return undefined;
  }
  const named = dice.slice(0, SUMMARY_MAX_DICE).map((term, index) => {
    const sign = term.sign === -1 ? '-' : index === 0 ? '' : '+';
    const count = term.count === 1 ? '' : String(term.count);
    return `${sign}${count}d${term.faces}`;
  });
  return dice.length > SUMMARY_MAX_DICE ? `${named.join('')}+…` : named.join('');
}
