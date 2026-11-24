import { valueAtPath } from '../api/models/sheet';
import type { FieldNode, FieldSchema, TemplateDetail } from '../api/models/template';
import { isPlainObject } from '../api/parse';
import type { DemoDerivation } from './demo_store';

type FormulaValue = number | boolean;

interface Token {
  readonly kind: 'number' | 'name' | 'symbol';
  readonly text: string;
  readonly value: number;
}

type FormulaNode =
  | { readonly kind: 'number'; readonly value: number }
  | { readonly kind: 'name'; readonly name: string }
  | { readonly kind: 'unary'; readonly operator: string; readonly operand: FormulaNode }
  | {
      readonly kind: 'binary';
      readonly operator: string;
      readonly left: FormulaNode;
      readonly right: FormulaNode;
    }
  | {
      readonly kind: 'conditional';
      readonly test: FormulaNode;
      readonly consequent: FormulaNode;
      readonly alternate: FormulaNode;
    }
  | { readonly kind: 'call'; readonly name: string; readonly args: readonly FormulaNode[] };

const SYMBOLS = [
  '&&',
  '||',
  '<=',
  '>=',
  '==',
  '!=',
  '+',
  '-',
  '*',
  '/',
  '%',
  '(',
  ')',
  ',',
  '<',
  '>',
  '?',
  ':',
  '!',
] as const;

const formulaCache = new Map<string, FormulaNode>();

function tokenize(formula: string): readonly Token[] {
  const tokens: Token[] = [];
  let index = 0;
  while (index < formula.length) {
    const character = formula[index] as string;
    if (/\s/.test(character)) {
      index += 1;
      continue;
    }
    if (/[0-9]/.test(character) || (character === '.' && /[0-9]/.test(formula[index + 1] ?? ''))) {
      let end = index;
      while (end < formula.length && /[0-9.]/.test(formula[end] as string)) {
        end += 1;
      }
      const text = formula.slice(index, end);
      const value = Number(text);
      if (!Number.isFinite(value)) {
        throw new Error(`formula holds an invalid number: ${text}`);
      }
      tokens.push({ kind: 'number', text, value });
      index = end;
      continue;
    }
    if (/[A-Za-z_]/.test(character)) {
      let end = index;
      while (end < formula.length && /[A-Za-z0-9_]/.test(formula[end] as string)) {
        end += 1;
      }
      tokens.push({ kind: 'name', text: formula.slice(index, end), value: 0 });
      index = end;
      continue;
    }
    const symbol = SYMBOLS.find((candidate) => formula.startsWith(candidate, index));
    if (symbol === undefined) {
      throw new Error(`formula holds an unknown symbol at ${String(index)}`);
    }
    tokens.push({ kind: 'symbol', text: symbol, value: 0 });
    index += symbol.length;
  }
  return tokens;
}

class FormulaParser {
  private position = 0;

  constructor(private readonly tokens: readonly Token[]) {}

  parse(): FormulaNode {
    const node = this.ternary();
    if (this.position !== this.tokens.length) {
      throw new Error('formula has trailing tokens');
    }
    return node;
  }

  private peek(): Token | undefined {
    return this.tokens[this.position];
  }

  private match(...symbols: readonly string[]): string | undefined {
    const token = this.peek();
    if (token === undefined || token.kind !== 'symbol' || !symbols.includes(token.text)) {
      return undefined;
    }
    this.position += 1;
    return token.text;
  }

  private expect(symbol: string): void {
    if (this.match(symbol) === undefined) {
      throw new Error(`formula expected ${symbol}`);
    }
  }

  private ternary(): FormulaNode {
    const test = this.or();
    if (this.match('?') === undefined) {
      return test;
    }
    const consequent = this.ternary();
    this.expect(':');
    const alternate = this.ternary();
    return { kind: 'conditional', test, consequent, alternate };
  }

  private or(): FormulaNode {
    let node = this.and();
    while (this.match('||') !== undefined) {
      node = { kind: 'binary', operator: '||', left: node, right: this.and() };
    }
    return node;
  }

  private and(): FormulaNode {
    let node = this.equality();
    while (this.match('&&') !== undefined) {
      node = { kind: 'binary', operator: '&&', left: node, right: this.equality() };
    }
    return node;
  }

  private equality(): FormulaNode {
    let node = this.comparison();
    let operator = this.match('==', '!=');
    while (operator !== undefined) {
      node = { kind: 'binary', operator, left: node, right: this.comparison() };
      operator = this.match('==', '!=');
    }
    return node;
  }

  private comparison(): FormulaNode {
    let node = this.additive();
    let operator = this.match('<', '<=', '>', '>=');
    while (operator !== undefined) {
      node = { kind: 'binary', operator, left: node, right: this.additive() };
      operator = this.match('<', '<=', '>', '>=');
    }
    return node;
  }

  private additive(): FormulaNode {
    let node = this.multiplicative();
    let operator = this.match('+', '-');
    while (operator !== undefined) {
      node = { kind: 'binary', operator, left: node, right: this.multiplicative() };
      operator = this.match('+', '-');
    }
    return node;
  }

  private multiplicative(): FormulaNode {
    let node = this.unary();
    let operator = this.match('*', '/', '%');
    while (operator !== undefined) {
      node = { kind: 'binary', operator, left: node, right: this.unary() };
      operator = this.match('*', '/', '%');
    }
    return node;
  }

  private unary(): FormulaNode {
    const operator = this.match('-', '+', '!');
    if (operator !== undefined) {
      return { kind: 'unary', operator, operand: this.unary() };
    }
    return this.primary();
  }

  private primary(): FormulaNode {
    const token = this.peek();
    if (token === undefined) {
      throw new Error('formula ended unexpectedly');
    }
    if (token.kind === 'number') {
      this.position += 1;
      return { kind: 'number', value: token.value };
    }
    if (token.kind === 'name') {
      this.position += 1;
      if (this.match('(') === undefined) {
        return { kind: 'name', name: token.text };
      }
      const args: FormulaNode[] = [];
      if (this.match(')') === undefined) {
        args.push(this.ternary());
        while (this.match(',') !== undefined) {
          args.push(this.ternary());
        }
        this.expect(')');
      }
      return { kind: 'call', name: token.text, args };
    }
    if (this.match('(') !== undefined) {
      const node = this.ternary();
      this.expect(')');
      return node;
    }
    throw new Error(`formula has an unexpected token: ${token.text}`);
  }
}

function parseFormula(formula: string): FormulaNode {
  const cached = formulaCache.get(formula);
  if (cached !== undefined) {
    return cached;
  }
  const node = new FormulaParser(tokenize(formula)).parse();
  formulaCache.set(formula, node);
  return node;
}

function numeric(value: FormulaValue): number {
  return typeof value === 'number' ? value : value ? 1 : 0;
}

function truthy(value: FormulaValue): boolean {
  return typeof value === 'boolean' ? value : value !== 0;
}

function argument(args: readonly number[], name: string, index: number): number {
  const value = args[index];
  if (value === undefined) {
    throw new Error(`${name} is missing an argument`);
  }
  return value;
}

function lookup(args: readonly number[]): number {
  if (args.length < 3 || args.length % 2 === 0) {
    throw new Error('lookup expects an input and at least one key/value pair');
  }
  const input = argument(args, 'lookup', 0);
  let result = 0;
  let previousKey = 0;
  for (let index = 1; index + 1 < args.length; index += 2) {
    const key = argument(args, 'lookup', index);
    const value = argument(args, 'lookup', index + 1);
    if (index > 1 && key <= previousKey) {
      throw new Error('lookup keys are not in ascending order');
    }
    if (index === 1 || key <= input) {
      result = value;
    }
    previousKey = key;
  }
  return result;
}

function call(name: string, args: readonly number[]): number {
  switch (name) {
    case 'floor':
      return Math.floor(argument(args, name, 0));
    case 'min':
      return Math.min(argument(args, name, 0), argument(args, name, 1));
    case 'max':
      return Math.max(argument(args, name, 0), argument(args, name, 1));
    case 'lookup':
      return lookup(args);
    default:
      throw new Error(`formula calls an unknown function: ${name}`);
  }
}

function evaluate(node: FormulaNode, env: Readonly<Record<string, number>>): FormulaValue {
  switch (node.kind) {
    case 'number':
      return node.value;
    case 'name': {
      const value = env[node.name];
      if (value === undefined) {
        throw new Error(`formula reads an unknown value: ${node.name}`);
      }
      return value;
    }
    case 'unary': {
      const operand = evaluate(node.operand, env);
      if (node.operator === '!') {
        return !truthy(operand);
      }
      const number = numeric(operand);
      return node.operator === '-' ? -number : number;
    }
    case 'conditional':
      return truthy(evaluate(node.test, env))
        ? evaluate(node.consequent, env)
        : evaluate(node.alternate, env);
    case 'call':
      return call(
        node.name,
        node.args.map((entry) => numeric(evaluate(entry, env))),
      );
    case 'binary': {
      if (node.operator === '&&') {
        return truthy(evaluate(node.left, env)) && truthy(evaluate(node.right, env));
      }
      if (node.operator === '||') {
        return truthy(evaluate(node.left, env)) || truthy(evaluate(node.right, env));
      }
      const left = numeric(evaluate(node.left, env));
      const right = numeric(evaluate(node.right, env));
      switch (node.operator) {
        case '+':
          return left + right;
        case '-':
          return left - right;
        case '*':
          return left * right;
        case '/':
          return left / right;
        case '%':
          return left % right;
        case '<':
          return left < right;
        case '<=':
          return left <= right;
        case '>':
          return left > right;
        case '>=':
          return left >= right;
        case '==':
          return left === right;
        case '!=':
          return left !== right;
        default:
          throw new Error(`formula uses an unknown operator: ${node.operator}`);
      }
    }
  }
}

interface DerivedLeaf {
  readonly path: string;
  readonly schema: FieldSchema;
}

interface RecordDerivation {
  readonly shape: Readonly<Record<string, FieldSchema>>;
  readonly fields: readonly DerivedLeaf[];
}

interface TemplateIndex {
  readonly leaves: ReadonlyMap<string, FieldSchema>;
  readonly derivedByPath: ReadonlyMap<string, DerivedLeaf>;
  readonly derived: readonly DerivedLeaf[];
  readonly recordDerived: ReadonlyMap<string, RecordDerivation>;
}

function indexTemplate(template: TemplateDetail): TemplateIndex {
  const leaves = new Map<string, FieldSchema>();
  const derivedByPath = new Map<string, DerivedLeaf>();
  const derived: DerivedLeaf[] = [];
  const recordDerived = new Map<string, RecordDerivation>();

  const walk = (nodes: Readonly<Record<string, FieldNode>>, prefix: string): void => {
    for (const [key, node] of Object.entries(nodes)) {
      const path = prefix === '' ? key : `${prefix}.${key}`;
      if (node.kind === 'leaf') {
        leaves.set(path, node.schema);
        if (
          !node.schema.isEditable &&
          node.schema.formula !== undefined &&
          node.schema.formula !== ''
        ) {
          const leaf = { path, schema: node.schema };
          derivedByPath.set(path, leaf);
          derived.push(leaf);
        }
        continue;
      }
      if (node.kind === 'list') {
        const shape: Record<string, FieldSchema> = {};
        const fields: DerivedLeaf[] = [];
        for (const [field, leaf] of Object.entries(node.list.shape)) {
          shape[field] = leaf.schema;
          if (
            !leaf.schema.isEditable &&
            leaf.schema.formula !== undefined &&
            leaf.schema.formula !== ''
          ) {
            fields.push({ path: `${path}.${field}`, schema: leaf.schema });
          }
        }
        if (fields.length > 0) {
          recordDerived.set(path, { shape, fields });
        }
        continue;
      }
      walk(node.children, path);
    }
  };

  walk(template.fields, '');
  return { leaves, derivedByPath, derived, recordDerived };
}

function readNumber(data: Record<string, unknown>, path: string, fallback: number): number {
  let value: unknown;
  try {
    value = valueAtPath(data, path);
  } catch {
    value = null;
  }
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function formulaEnvironment(
  schema: FieldSchema,
  resolve: (segments: readonly string[]) => number,
): Readonly<Record<string, number>> {
  const env: Record<string, number> = {};
  for (const dependency of schema.dependsOn ?? []) {
    env[dependency.join('_')] = resolve(dependency);
  }
  return env;
}

function evaluateSchema(
  schema: FieldSchema,
  resolve: (segments: readonly string[]) => number,
): number {
  const formula = schema.formula;
  if (formula === undefined || formula === '') {
    throw new Error('a derived leaf needs a formula');
  }
  const value = evaluate(parseFormula(formula), formulaEnvironment(schema, resolve));
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error('a formula must produce a finite number');
  }
  return value;
}

function recordEnvironment(
  schema: FieldSchema,
  record: Readonly<Record<string, unknown>>,
  shape: Readonly<Record<string, FieldSchema>>,
  staticValue: (segments: readonly string[]) => number,
): Readonly<Record<string, number>> {
  const env: Record<string, number> = {};
  for (const dependency of schema.dependsOn ?? []) {
    const field = dependency[0];
    if (dependency.length === 1 && field !== undefined && shape[field] !== undefined) {
      const raw = record[field];
      const fallback = shape[field].defaultValue;
      const value =
        typeof raw === 'number' && Number.isFinite(raw)
          ? raw
          : typeof fallback === 'number'
            ? fallback
            : 0;
      env[field] = value;
      env[`record_${field}`] = value;
      continue;
    }
    env[dependency.join('_')] = staticValue(dependency);
  }
  return env;
}

function derivationFor(index: TemplateIndex): DemoDerivation {
  return (data) => {
    const values = new Map<string, number>();
    const memo = new Map<string, number>();
    const computing = new Set<string>();

    const defaultAt = (path: string): number => {
      const schema = index.leaves.get(path);
      return typeof schema?.defaultValue === 'number' ? schema.defaultValue : 0;
    };

    function staticValue(segments: readonly string[]): number {
      const path = segments.join('.');
      const leaf = index.derivedByPath.get(path);
      if (leaf !== undefined) {
        return compute(leaf) ?? defaultAt(path);
      }
      return readNumber(data, path, defaultAt(path));
    }

    function compute(leaf: DerivedLeaf): number | undefined {
      const cached = memo.get(leaf.path);
      if (cached !== undefined) {
        return cached;
      }
      if (computing.has(leaf.path)) {
        return undefined;
      }
      computing.add(leaf.path);
      let value: number | undefined;
      try {
        value = evaluateSchema(leaf.schema, staticValue);
      } catch {
        value = undefined;
      }
      computing.delete(leaf.path);
      if (value !== undefined) {
        memo.set(leaf.path, value);
      }
      return value;
    }

    for (const leaf of index.derived) {
      const value = compute(leaf);
      if (value !== undefined) {
        values.set(leaf.path, value);
      }
    }

    for (const [listPath, listing] of index.recordDerived) {
      let raw: unknown;
      try {
        raw = valueAtPath(data, listPath);
      } catch {
        raw = null;
      }
      if (!isPlainObject(raw)) {
        continue;
      }
      for (const [recordId, record] of Object.entries(raw)) {
        if (!isPlainObject(record)) {
          continue;
        }
        for (const leaf of listing.fields) {
          try {
            const value = evaluate(
              parseFormula(leaf.schema.formula as string),
              recordEnvironment(leaf.schema, record, listing.shape, staticValue),
            );
            if (typeof value === 'number' && Number.isFinite(value)) {
              values.set(`${listPath}.${recordId}.${leaf.path.slice(listPath.length + 1)}`, value);
            }
          } catch {
            continue;
          }
        }
      }
    }

    return values;
  };
}

export function templateDerivation(template: TemplateDetail): DemoDerivation {
  return derivationFor(indexTemplate(template));
}
