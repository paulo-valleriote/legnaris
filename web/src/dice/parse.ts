import { refuse, succeed, type DiceResult } from './errors';

export const LIMITS = {
  maxLength: 64,
  maxTerms: 8,
  minDice: 1,
  maxDice: 20,
  minFaces: 1,
  maxFaces: 1000,
  maxWholeNumber: 9999,
} as const;

export interface DiceTerm {
  readonly kind: 'dice';
  readonly sign: 1 | -1;
  readonly count: number;
  readonly faces: number;
}

export interface NumberTerm {
  readonly kind: 'number';
  readonly sign: 1 | -1;
  readonly value: number;
}

export type FieldTerm =
  | {
      readonly kind: 'field';
      readonly sign: 1 | -1;
      readonly relative: false;
      readonly path: string;
    }
  | {
      readonly kind: 'field';
      readonly sign: 1 | -1;
      readonly relative: true;
      readonly column: string;
    };

export type Term = DiceTerm | NumberTerm | FieldTerm;

export interface Expression {
  readonly terms: readonly Term[];
}

const DICE_TERM = /^(\d*)[dD](\d+)$/;
const NUMBER_TERM = /^\d+$/;
const RELATIVE_TERM = /^@(?:\.([A-Za-z][A-Za-z0-9]*))?$/;
const BARE_WORD_TERM = /^(?:[dD]|[kK][hHlL]\d*)$/;
const PATH_TERM = /^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)*$/;
const OPERATOR = /[+-]/;

function trimBlanks(text: string): string {
  let start = 0;
  let end = text.length;
  while (start < end && (text[start] === ' ' || text[start] === '\t')) {
    start += 1;
  }
  while (end > start && (text[end - 1] === ' ' || text[end - 1] === '\t')) {
    end -= 1;
  }
  return text.slice(start, end);
}

function splitOperators(expr: string): string[] {
  const pieces: string[] = [];
  let start = 0;
  for (let i = 0; i < expr.length; i += 1) {
    if (OPERATOR.test(expr[i])) {
      pieces.push(trimBlanks(expr.slice(start, i)), expr[i]);
      start = i + 1;
    }
  }
  pieces.push(trimBlanks(expr.slice(start)));
  return pieces;
}

const signOf = (operator: string): 1 | -1 => (operator === '-' ? -1 : 1);

function boundedInt(digits: string, limit: number): number | null {
  let value = 0;
  for (const digit of digits) {
    value = value * 10 + (digit.charCodeAt(0) - 48);
    if (value > limit) {
      return null;
    }
  }
  return value;
}

type UnsignedDice = Omit<DiceTerm, 'sign'>;

function parseDiceTerm(token: string, countDigits: string, facesDigits: string) {
  let count = 1;
  if (countDigits !== '') {
    const parsed = boundedInt(countDigits, LIMITS.maxDice);
    if (parsed === null || parsed < LIMITS.minDice) {
      return refuse<UnsignedDice>(
        'dice_count',
        `"${token}": roll between ${LIMITS.minDice} and ${LIMITS.maxDice} dice at a time`,
      );
    }
    count = parsed;
  }
  const faces = boundedInt(facesDigits, LIMITS.maxFaces);
  if (faces === null || faces < LIMITS.minFaces) {
    return refuse<UnsignedDice>(
      'dice_faces',
      `"${token}": a die has between ${LIMITS.minFaces} and ${LIMITS.maxFaces} faces`,
    );
  }
  return succeed<UnsignedDice>({ kind: 'dice', count, faces });
}

function parseTerm(token: string, sign: 1 | -1): DiceResult<Term> {
  if (token === '') {
    return refuse('missing_term', 'a term is missing next to a + or -');
  }
  const dice = DICE_TERM.exec(token);
  if (dice) {
    const parsed = parseDiceTerm(token, dice[1], dice[2]);
    return parsed.ok ? succeed({ ...parsed.value, sign }) : parsed;
  }
  if (NUMBER_TERM.test(token)) {
    const value = boundedInt(token, LIMITS.maxWholeNumber);
    if (value === null) {
      return refuse('number_range', `"${token}": whole numbers go up to ${LIMITS.maxWholeNumber}`);
    }
    return succeed({ kind: 'number', sign, value });
  }
  const relative = RELATIVE_TERM.exec(token);
  if (relative) {
    return succeed({ kind: 'field', sign, relative: true, column: relative[1] ?? '' });
  }
  if (BARE_WORD_TERM.test(token)) {
    return refuse(
      'unreadable_term',
      `cannot read "${token}": a die needs its faces (NdM) and keep modifiers are not supported`,
    );
  }
  if (PATH_TERM.test(token)) {
    return succeed({ kind: 'field', sign, relative: false, path: token });
  }
  return refuse(
    'unreadable_term',
    `cannot read "${token}": only NdM, whole numbers, Field paths and @, joined by + or -`,
  );
}

export function parseDice(expr: string): DiceResult<Expression> {
  if (typeof expr !== 'string') {
    return refuse('empty', 'a Dice expression is text');
  }
  const length = [...expr].length;
  if (length > LIMITS.maxLength) {
    return refuse(
      'too_long',
      `the expression has ${length} characters (limit ${LIMITS.maxLength})`,
    );
  }
  if (trimBlanks(expr) === '') {
    return refuse('empty', 'the expression is empty');
  }

  const pieces = splitOperators(expr);
  const terms: Term[] = [];
  let index = 0;
  let sign: 1 | -1 = 1;
  if (pieces[0] === '' && pieces.length > 1) {
    sign = signOf(pieces[1]);
    index = 2;
  }
  while (index < pieces.length) {
    const term = parseTerm(pieces[index], sign);
    if (!term.ok) {
      return term;
    }
    terms.push(term.value);
    index += 1;
    if (index < pieces.length) {
      sign = signOf(pieces[index]);
      index += 1;
    }
  }
  if (terms.length > LIMITS.maxTerms) {
    return refuse(
      'too_many_terms',
      `the expression has ${terms.length} terms (limit ${LIMITS.maxTerms})`,
    );
  }
  return succeed({ terms });
}

export function termLabel(term: Term): string {
  switch (term.kind) {
    case 'dice':
      return `${term.count}d${term.faces}`;
    case 'number':
      return String(term.value);
    case 'field':
      if (!term.relative) {
        return term.path;
      }
      return term.column === '' ? '@' : `@.${term.column}`;
  }
}

export function canonicalForm(expression: Expression): string {
  return expression.terms
    .map((term, index) => {
      const label = termLabel(term);
      if (index === 0) {
        return term.sign < 0 ? `-${label}` : label;
      }
      return `${term.sign < 0 ? '-' : '+'} ${label}`;
    })
    .join(' ');
}
