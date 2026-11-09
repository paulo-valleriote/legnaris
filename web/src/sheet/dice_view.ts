/**
 * The sheet's view of Dice expressions (ADR 0005): the Field schema the grammar
 * is judged against, the client-side check that mirrors the server's PUT, the
 * pruning of overrides the Template no longer supports, and the copy of each
 * refusal class. Pure; the network and the React tree live elsewhere.
 */

import type { DiceError, DiceErrorClass } from '../dice/errors';
import { parseDice } from '../dice/parse';
import { schemaFromKinds, validateRow, type DiceSchema, type FieldKind } from '../dice/fit';
import {
  resolveDice,
  type DiceChoices,
  type DiceDefaults as ResolveDiceDefaults,
  type Resolution,
} from '../dice/resolve';
import type { DiceBlock, DiceDefaults as TemplateDiceDefaults } from '../api/models/dice';
import type { FieldNode } from '../api/models/template';
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

/** Whether [text] parses at all, independent of any Row (used for Section expressions). */
export function checkSectionExpression(text: string): DiceError | undefined {
  const parsed = parseDice(text);
  return parsed.ok ? undefined : parsed.error;
}

export interface PruneContext {
  readonly rowKeys: ReadonlySet<string>;
  readonly sectionKeys: ReadonlySet<string>;
  readonly schema: DiceSchema;
}

function keepEntries(
  record: Readonly<Record<string, string | null>>,
  keep: (key: string, expression: string) => boolean,
): Record<string, string | null> {
  return Object.fromEntries(
    Object.entries(record).filter(([key, expression]) =>
      expression === null ? true : keep(key, expression),
    ),
  );
}

/**
 * Drops the overrides the server may still hold but the Template no longer
 * supports: a key naming no Row or Section, or a Row expression that no longer
 * fits. An explicit "no dice" (`null`) is a choice and always survives.
 * Dropping one falls the Row back to the Template default, never breaks the Sheet.
 */
export function pruneStaleChoices(choices: DiceChoices, context: PruneContext): DiceChoices {
  return {
    row: keepEntries(choices.row ?? {}, (key, expression) => {
      return context.rowKeys.has(key) && validateRow(context.schema, key, expression).ok;
    }),
    section: keepEntries(choices.section ?? {}, (key) => context.sectionKeys.has(key)),
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
  readonly sectionKeys: ReadonlySet<string>;
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
    sectionKeys: input.sectionKeys,
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
