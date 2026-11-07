import { describe, expect, it } from 'vitest';
import {
  parseDice,
  schemaFromKinds,
  validateRow,
  validateSection,
  type DiceSchema,
  type Expression,
} from '../dice';
import { diceVectors } from './dice_vectors';

const schema = schemaFromKinds(diceVectors.fields);

function parsed(expr: string): Expression {
  const result = parseDice(expr);
  if (!result.ok) {
    throw new Error(`fixture does not parse: ${expr}`);
  }
  return result.value;
}

describe('validateRow against the shared vectors', () => {
  it.each(diceVectors.fit)(
    'Vector: $name ($expr on $row)',
    ({ expr, row, fits, class: reason }) => {
      const result = validateRow(schema, row, expr);
      expect(result.ok).toBe(fits);
      if (!result.ok) {
        expect(result.error.class).toBe(reason);
      }
    },
  );
});

describe('validateSection against the shared vectors', () => {
  it.each(diceVectors.sections)('Vector: $name ($expr)', ({ expr, rows, fits, class: reason }) => {
    const result = validateSection(schema, rows, expr);
    expect(result.ok).toBe(fits);
    if (!result.ok) {
      expect(result.error.class).toBe(reason);
    }
  });
});

describe('validation edge cases', () => {
  it('Error: a syntax error is reported before any Field is looked at', () => {
    const result = validateRow(schema, 'combat.initiative', '1d20 +');
    expect(!result.ok && result.error.class).toBe('missing_term');
  });

  it('Error: a Section with no Rows fits nothing', () => {
    const result = validateSection(schema, [], '1d20');
    expect(!result.ok && result.error.class).toBe('no_rows');
  });

  it('Edge: a Section expression that fits only one of its Rows is accepted', () => {
    const result = validateSection(
      schema,
      ['modifiers.strengthMod', 'skills.acrobatics'],
      '1d20 + @.total',
    );
    expect(result.ok).toBe(true);
  });

  it('Error: a Field that is a group, not a leaf, is unknown', () => {
    const result = validateRow(schema, 'combat.initiative', '1d20 + stats');
    expect(!result.ok && result.error.class).toBe('unknown_field');
  });

  it('Error: a text or boolean Field is not numeric', () => {
    for (const path of ['identity.name', 'features.darkvision']) {
      const result = validateRow(schema, 'combat.initiative', path);
      expect(!result.ok && result.error.class).toBe('non_numeric_field');
    }
  });

  it('Error: Object.prototype keys are never Fields', () => {
    const result = validateRow(schema, 'combat.initiative', 'constructor');
    expect(!result.ok && result.error.class).toBe('unknown_field');
    const inherited = validateRow(schema, 'combat.initiative', '1d20 + toString');
    expect(!inherited.ok && inherited.error.class).toBe('unknown_field');
  });

  it('Error: a schema that knows no Field refuses every reference', () => {
    const empty: DiceSchema = () => undefined;
    const result = validateRow(empty, 'combat.initiative', '@');
    expect(!result.ok && result.error.class).toBe('unknown_field');
    expect(validateRow(empty, 'combat.initiative', '1d20 + 5').ok).toBe(true);
  });

  it('Happy path: the validated expression is the parsed one', () => {
    const result = validateRow(schema, 'combat.initiative', 'd20 + @');
    expect(result.ok && result.value).toEqual(parsed('1d20 + @'));
  });
});
