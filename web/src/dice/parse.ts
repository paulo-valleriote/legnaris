import { refuse, succeed, type DiceError, type DiceResult } from './errors';

export const LIMITS = {
  maxLength: 64,
  maxTerms: 8,
  minDice: 1,
  maxDice: 20,
  minFaces: 1,
  maxFaces: 1000,
  maxWholeNumber: 9999,
} as const;

export type FieldReference =
  | { readonly relative: false; readonly path: string }
  | { readonly relative: true; readonly column: string };

export interface DiceTerm {
  readonly kind: 'dice';
  readonly sign: 1 | -1;
  readonly count: number;
  readonly faces: number;
  readonly countRef?: FieldReference;
  readonly facesRef?: FieldReference;
}

export interface NumberTerm {
  readonly kind: 'number';
  readonly sign: 1 | -1;
  readonly value: number;
}

export type FieldTerm = FieldReference & {
  readonly kind: 'field';
  readonly sign: 1 | -1;
};

export type Term = DiceTerm | NumberTerm | FieldTerm;

export interface Expression {
  readonly terms: readonly Term[];
  readonly minimum?: number;
}

const NUMBER_TERM = /^\d+$/;
const RELATIVE_TERM = /^@(?:\.([A-Za-z][A-Za-z0-9]*))?$/;
const BARE_WORD_TERM = /^(?:[dD]|[kK][hHlL]\d*)$/;
const PATH_TERM = /^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)*$/;
const MIN_SUFFIX = /[ \t]+min[ \t]+(\d+)[ \t]*$/;
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

interface PartParse {
  readonly match: boolean;
  readonly value: number;
  readonly ref?: FieldReference;
  readonly error?: DiceError;
}

const noMatch: PartParse = { match: false, value: 0 };

function parseReference(token: string): FieldReference | undefined {
  if (token.length < 3 || token[0] !== '{' || token[token.length - 1] !== '}') {
    return undefined;
  }
  const inner = token.slice(1, -1);
  if (inner.includes('{') || inner.includes('}')) {
    return undefined;
  }
  const relative = RELATIVE_TERM.exec(inner);
  if (relative) {
    return { relative: true, column: relative[1] ?? '' };
  }
  if (PATH_TERM.test(inner)) {
    return { relative: false, path: inner };
  }
  return undefined;
}

function parseCountPart(token: string, part: string): PartParse {
  if (part === '') {
    return { match: true, value: 1 };
  }
  const ref = parseReference(part);
  if (ref !== undefined) {
    return { match: true, value: 0, ref };
  }
  if (!NUMBER_TERM.test(part)) {
    return noMatch;
  }
  const value = boundedInt(part, LIMITS.maxDice);
  if (value === null || value < LIMITS.minDice) {
    return {
      match: true,
      value: 0,
      error: {
        class: 'dice_count',
        message: `"${token}": roll between ${LIMITS.minDice} and ${LIMITS.maxDice} dice at a time`,
      },
    };
  }
  return { match: true, value };
}

function parseFacesPart(token: string, part: string): PartParse {
  if (part === '') {
    return noMatch;
  }
  const ref = parseReference(part);
  if (ref !== undefined) {
    return { match: true, value: 0, ref };
  }
  if (!NUMBER_TERM.test(part)) {
    return noMatch;
  }
  const value = boundedInt(part, LIMITS.maxFaces);
  if (value === null || value < LIMITS.minFaces) {
    return {
      match: true,
      value: 0,
      error: {
        class: 'dice_faces',
        message: `"${token}": a die has between ${LIMITS.minFaces} and ${LIMITS.maxFaces} faces`,
      },
    };
  }
  return { match: true, value };
}

function splitDiceParts(
  token: string,
): { readonly countPart: string; readonly facesPart: string } | undefined {
  let depth = 0;
  for (let i = 0; i < token.length; i += 1) {
    const character = token[i];
    if (character === '{') {
      depth += 1;
    } else if (character === '}') {
      if (depth === 0) {
        return undefined;
      }
      depth -= 1;
    } else if ((character === 'd' || character === 'D') && depth === 0) {
      return { countPart: token.slice(0, i), facesPart: token.slice(i + 1) };
    }
  }
  return undefined;
}

function parseDiceTerm(
  token: string,
  countPart: string,
  facesPart: string,
): DiceResult<UnsignedDice> | undefined {
  const count = parseCountPart(token, countPart);
  if (!count.match) {
    return undefined;
  }
  if (count.error !== undefined) {
    return { ok: false, error: count.error };
  }
  const faces = parseFacesPart(token, facesPart);
  if (!faces.match) {
    return undefined;
  }
  if (faces.error !== undefined) {
    return { ok: false, error: faces.error };
  }
  return succeed<UnsignedDice>({
    kind: 'dice',
    count: count.value,
    faces: faces.value,
    ...(count.ref === undefined ? {} : { countRef: count.ref }),
    ...(faces.ref === undefined ? {} : { facesRef: faces.ref }),
  });
}

function parseTerm(token: string, sign: 1 | -1): DiceResult<Term> {
  if (token === '') {
    return refuse('missing_term', 'a term is missing next to a + or -');
  }
  const parts = splitDiceParts(token);
  if (parts !== undefined) {
    const dice = parseDiceTerm(token, parts.countPart, parts.facesPart);
    if (dice !== undefined) {
      return dice.ok ? succeed({ ...dice.value, sign }) : dice;
    }
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

interface SumWithMinimum {
  readonly sum: string;
  readonly minimum?: number;
}

function splitMinimum(expr: string): DiceResult<SumWithMinimum> {
  const match = MIN_SUFFIX.exec(expr);
  if (match === null) {
    return succeed({ sum: expr });
  }
  const minimum = boundedInt(match[1], LIMITS.maxWholeNumber);
  if (minimum === null) {
    return refuse(
      'number_range',
      `"${match[0].trim()}": minimums go up to ${LIMITS.maxWholeNumber}`,
    );
  }
  const sum = trimBlanks(expr.slice(0, match.index));
  if (sum === '') {
    return refuse('missing_term', 'a minimum needs an expression before it');
  }
  return succeed({ sum, minimum });
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

  const split = splitMinimum(expr);
  if (!split.ok) {
    return split;
  }
  const pieces = splitOperators(split.value.sum);
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
  return succeed({
    terms,
    ...(split.value.minimum === undefined ? {} : { minimum: split.value.minimum }),
  });
}

export function referenceLabel(reference: FieldReference): string {
  if (!reference.relative) {
    return reference.path;
  }
  return reference.column === '' ? '@' : `@.${reference.column}`;
}

function dicePartLabel(value: number, reference: FieldReference | undefined): string {
  return reference === undefined ? String(value) : `{${referenceLabel(reference)}}`;
}

export function termLabel(term: Term): string {
  switch (term.kind) {
    case 'dice':
      return `${dicePartLabel(term.count, term.countRef)}d${dicePartLabel(term.faces, term.facesRef)}`;
    case 'number':
      return String(term.value);
    case 'field':
      return referenceLabel(term);
  }
}

export function canonicalForm(expression: Expression): string {
  const terms = expression.terms
    .map((term, index) => {
      const label = termLabel(term);
      if (index === 0) {
        return term.sign < 0 ? `-${label}` : label;
      }
      return `${term.sign < 0 ? '-' : '+'} ${label}`;
    })
    .join(' ');
  return expression.minimum === undefined ? terms : `${terms} min ${expression.minimum}`;
}
