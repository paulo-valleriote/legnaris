import { describe, expect, it } from 'vitest';
import type { FieldNode, FieldSchema } from '../api/models/template';
import type { SheetField } from '../engine/field_grouping';
import { buildSections } from '../engine/field_grouping';
import {
  MAX_BESIDE_DEPENDENTS,
  buildDependentsIndex,
  dependentsAreStale,
  dependentsOf,
} from '../engine/dependents';

function schema(overrides: Partial<FieldSchema> = {}): FieldSchema {
  return { type: 'number', label: '', isEditable: true, defaultValue: 0, ...overrides };
}

function input(overrides: Partial<FieldSchema> = {}): FieldNode {
  return { kind: 'leaf', schema: schema(overrides) };
}

function derived(
  on: readonly (readonly string[])[],
  overrides: Partial<FieldSchema> = {},
): FieldNode {
  return input({ isEditable: false, formula: 'x', dependsOn: on, ...overrides });
}

function group(children: Record<string, FieldNode>, columns?: readonly string[]): FieldNode {
  return { kind: 'group', children, columns };
}

function indexOf(fields: Record<string, FieldNode>) {
  return buildDependentsIndex(buildSections(fields));
}

function fieldAt(fields: Record<string, FieldNode>, path: string): SheetField {
  const found = buildSections(fields)
    .flatMap((section) => section.leaves)
    .find((leaf) => leaf.path === path);
  if (found === undefined) {
    throw new Error(`no leaf ${path}`);
  }
  return found;
}

function dependentPaths(fields: Record<string, FieldNode>, source: string): string[] {
  return dependentsOf(indexOf(fields), fieldAt(fields, source)).map((d) => d.path);
}

describe('buildDependentsIndex', () => {
  it('Edge: an empty tree has no edges', () => {
    expect(indexOf({}).size).toBe(0);
  });

  it('Edge: a tree with no dependsOn has no edges', () => {
    expect(indexOf({ a: group({ x: input(), y: input() }) }).size).toBe(0);
  });

  it('Edge: an empty dependsOn list adds no edge', () => {
    expect(indexOf({ a: group({ x: input(), m: derived([]) }) }).size).toBe(0);
  });

  it('Edge: an editable leaf that lists a dependency is not a dependent', () => {
    const fields = { a: group({ x: input(), y: input({ dependsOn: [['a', 'x']] }) }) };

    expect(indexOf(fields).size).toBe(0);
  });

  it('Edge: a dependency naming no leaf (external, stale, a group) is ignored', () => {
    const fields = {
      a: group({ x: input(), m: derived([['rules', 'ghost'], ['a'], ['a', 'x']]) }),
    };

    expect([...indexOf(fields).keys()]).toEqual(['a.x']);
  });

  it('Edge: empty, malformed and empty-segment dependencies are ignored', () => {
    const fields = { a: group({ x: input(), m: derived([[], ['a', ''], ['']]) }) };

    expect(indexOf(fields).size).toBe(0);
  });

  it('Edge: a repeated dependency counts once', () => {
    const fields = {
      a: group({
        x: input(),
        m: derived([
          ['a', 'x'],
          ['a', 'x'],
        ]),
      }),
    };

    expect(
      indexOf(fields)
        .get('a.x')
        ?.map((d) => d.path),
    ).toEqual(['a.m']);
  });

  it('Edge: a leaf listing itself is not its own dependent', () => {
    const fields = { a: group({ m: derived([['a', 'm']]) }) };

    expect(indexOf(fields).size).toBe(0);
  });

  it('Edge: a cycle between derived leaves terminates and keeps both direct edges', () => {
    const fields = {
      a: group({ p: derived([['a', 'q']]), q: derived([['a', 'p']]) }),
    };
    const index = indexOf(fields);

    expect(index.get('a.q')?.map((d) => d.path)).toEqual(['a.p']);
    expect(index.get('a.p')?.map((d) => d.path)).toEqual(['a.q']);
  });

  it('Happy path: collects dependents across sections and table cells in tree order', () => {
    const fields = {
      stats: group({ str: input() }),
      skills: group({ ath: group({ total: derived([['stats', 'str']]) }, ['total']) }),
      mods: group({ strMod: derived([['stats', 'str']]) }),
    };

    expect(
      indexOf(fields)
        .get('stats.str')
        ?.map((d) => d.path),
    ).toEqual(['skills.ath.total', 'mods.strMod']);
  });
});

describe('dependentsOf', () => {
  const base = (count: number): Record<string, FieldNode> => ({
    a: group({
      x: input({ label: 'X' }),
      ...Object.fromEntries(
        Array.from({ length: count }, (_, i) => [`d${i}`, derived([['a', 'x']])]),
      ),
    }),
  });

  it('Edge: an input that feeds nothing shows nothing', () => {
    expect(dependentPaths(base(0), 'a.x')).toEqual([]);
  });

  it('Happy path: one direct dependent is shown', () => {
    expect(dependentPaths(base(1), 'a.x')).toEqual(['a.d0']);
  });

  it('Happy path: two direct dependents are shown, in tree order', () => {
    expect(MAX_BESIDE_DEPENDENTS).toBe(2);
    expect(dependentPaths(base(2), 'a.x')).toEqual(['a.d0', 'a.d1']);
  });

  it('Edge: three or more direct dependents show nothing', () => {
    expect(dependentPaths(base(3), 'a.x')).toEqual([]);
    expect(dependentPaths(base(20), 'a.x')).toEqual([]);
  });

  it('Edge: derived of derived is not direct (the grandchild is never beside the input)', () => {
    const fields = {
      a: group({
        x: input(),
        m: derived([['a', 'x']]),
        t: derived([['a', 'm']]),
      }),
    };

    expect(dependentPaths(fields, 'a.x')).toEqual(['a.m']);
  });

  it('Edge: a derived leaf is never a source', () => {
    const fields = {
      a: group({ x: input(), m: derived([['a', 'x']]), t: derived([['a', 'm']]) }),
    };

    expect(dependentPaths(fields, 'a.m')).toEqual([]);
  });

  it('Edge: a config switch, a toggle and a text input are never sources', () => {
    const fields = {
      config: group({ useX: input({ label: 'Use X' }) }),
      a: group({
        flag: input({ type: 'bool' }),
        note: input({ type: 'text' }),
        m: derived([
          ['config', 'useX'],
          ['a', 'flag'],
          ['a', 'note'],
        ]),
      }),
    };

    expect(dependentPaths(fields, 'config.useX')).toEqual([]);
    expect(dependentPaths(fields, 'a.flag')).toEqual([]);
    expect(dependentPaths(fields, 'a.note')).toEqual([]);
  });

  it('Edge: a field absent from the index shows nothing', () => {
    const stranger: SheetField = { path: 'nowhere.x', schema: schema() };

    expect(dependentsOf(new Map(), stranger)).toEqual([]);
  });

  it('Edge: a dependent may itself be a non-number derived leaf', () => {
    const fields = { a: group({ x: input(), s: derived([['a', 'x']], { type: 'text' }) }) };

    expect(dependentPaths(fields, 'a.x')).toEqual(['a.s']);
  });
});

describe('dependentsAreStale', () => {
  it.each([
    {
      name: 'Happy path: edited and not previewed is stale',
      edited: true,
      previewed: false,
      want: true,
    },
    { name: 'Edge: edited and previewed is settled', edited: true, previewed: true, want: false },
    { name: 'Edge: never edited is settled', edited: false, previewed: false, want: false },
    {
      name: 'Edge: never edited and previewed is settled',
      edited: false,
      previewed: true,
      want: false,
    },
  ])('$name', ({ edited, previewed, want }) => {
    expect(dependentsAreStale(edited, previewed)).toBe(want);
  });
});
