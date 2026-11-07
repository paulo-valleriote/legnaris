/**
 * Whether a Dice expression can be rolled for a Row: every Field it names must
 * exist and be numeric, where `@` and `@.<column>` resolve against the Row key.
 * Mirrors `backend/internal/dice/fit.go`; the shared vectors pin both.
 */

import { refuse, succeed, type DiceError, type DiceResult } from './errors';
import { parseDice, termLabel, type Expression, type FieldTerm } from './parse';

export type FieldKind = 'number' | 'text' | 'bool';

/**
 * Answers what a dotted Field path is, or `undefined` for a path that is not a
 * Field (a missing path, a group, a Section or a table Row key).
 */
export type DiceSchema = (path: string) => FieldKind | undefined;

/** Builds a {@link DiceSchema} over a record of path → kind; inherited keys such as `constructor` are never Fields. */
export function schemaFromKinds(kinds: Readonly<Record<string, FieldKind>>): DiceSchema {
  return (path) => (Object.hasOwn(kinds, path) ? kinds[path] : undefined);
}

/** The Field path a reference names for the Row keyed [rowKey]: `@` is the Row, `@.col` its column. */
export function resolveFieldPath(term: FieldTerm, rowKey: string): string {
  if (!term.relative) {
    return term.path;
  }
  return term.column === '' ? rowKey : `${rowKey}.${term.column}`;
}

/** Checks that [expression] can be rolled on the Row keyed [rowKey]; reports the first problem. */
export function fitsRow(
  expression: Expression,
  schema: DiceSchema,
  rowKey: string,
): DiceResult<Expression> {
  for (const term of expression.terms) {
    if (term.kind !== 'field') {
      continue;
    }
    const path = resolveFieldPath(term, rowKey);
    const kind = schema(path);
    if (kind === undefined) {
      return refuse(
        'unknown_field',
        `"${termLabel(term)}" points at "${path}", which does not exist`,
      );
    }
    if (kind !== 'number') {
      return refuse('non_numeric_field', `"${path}" is not a number`);
    }
  }
  return succeed(expression);
}

/** Parses [expr] and checks it fits the Row keyed [rowKey]: the check a Row-level expression must pass before it is stored. */
export function validateRow(
  schema: DiceSchema,
  rowKey: string,
  expr: string,
): DiceResult<Expression> {
  const parsed = parseDice(expr);
  return parsed.ok ? fitsRow(parsed.value, schema, rowKey) : parsed;
}

/**
 * Parses [expr] and checks it fits at least one of the Section's Rows, since a
 * Section expression is inherited by every Row but only has to be rollable on
 * some. A Section with no Rows fits nothing; when no Row fits, the error carries
 * the class of the first Row's problem.
 */
export function validateSection(
  schema: DiceSchema,
  rowKeys: readonly string[],
  expr: string,
): DiceResult<Expression> {
  const parsed = parseDice(expr);
  if (!parsed.ok) {
    return parsed;
  }
  if (rowKeys.length === 0) {
    return refuse('no_rows', 'the Section has no Rows');
  }
  let first: DiceError | undefined;
  for (const rowKey of rowKeys) {
    const fit = fitsRow(parsed.value, schema, rowKey);
    if (fit.ok) {
      return fit;
    }
    first ??= fit.error;
  }
  return refuse(
    first?.class ?? 'unknown_field',
    `it fits none of the Rows in the Section: ${first?.message ?? ''}`,
  );
}
