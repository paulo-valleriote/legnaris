import { refuse, succeed, type DiceError, type DiceResult } from './errors';
import { parseDice, termLabel, type Expression, type FieldTerm } from './parse';

export type FieldKind = 'number' | 'text' | 'bool';

export type DiceSchema = (path: string) => FieldKind | undefined;

export function schemaFromKinds(kinds: Readonly<Record<string, FieldKind>>): DiceSchema {
  return (path) => (Object.hasOwn(kinds, path) ? kinds[path] : undefined);
}

export function resolveFieldPath(term: FieldTerm, rowKey: string): string {
  if (!term.relative) {
    return term.path;
  }
  return term.column === '' ? rowKey : `${rowKey}.${term.column}`;
}

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

export function validateRow(
  schema: DiceSchema,
  rowKey: string,
  expr: string,
): DiceResult<Expression> {
  const parsed = parseDice(expr);
  return parsed.ok ? fitsRow(parsed.value, schema, rowKey) : parsed;
}

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
