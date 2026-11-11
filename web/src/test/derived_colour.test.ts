import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { contrastRatio } from './contrast';
import { readThemeTokenValues, resolveThemeTokenValues, tokensCss } from './tokens_files';

const here = path.dirname(fileURLToPath(import.meta.url));

function readCss(relative: string): string {
  return readFileSync(path.resolve(here, relative), 'utf8');
}

interface Rule {
  readonly selector: string;
  readonly body: string;
}

function rules(css: string): Rule[] {
  const stripped = css.replace(/\/\*[\s\S]*?\*\//g, '');
  return [...stripped.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((match) => ({
    selector: match[1].trim(),
    body: match[2],
  }));
}

function rulesFor(css: string, selector: string): Rule[] {
  return rules(css).filter((rule) =>
    rule.selector.split(',').some((part) => part.trim() === selector),
  );
}

function colourOf(rule: Rule): string | undefined {
  return rule.body.match(/(?:^|[;\s])color:\s*([^;]+);/)?.[1].trim();
}

const engineCss = readCss('../engine/engine.css');
const DERIVED_SURFACES = [
  readCss('../engine/engine.css'),
  readCss('../sheet/sheet.css'),
  readCss('../sheet/section_index.css'),
];
const DERIVED_SELECTOR = /readonly--derived|row--derived|lg-dependent(?!s)/;

describe('derived values are neutral text, never the accent (brief A11)', () => {
  it('Happy path: the derived readout is --lg-color-text, bold, recessed and borderless', () => {
    const [derived] = rulesFor(engineCss, '.lg-field__readonly--derived');

    expect(colourOf(derived)).toBe('var(--lg-color-text)');
    expect(derived.body).toMatch(/font-weight:\s*var\(--lg-font-weight-bold\)/);
    expect(derived.body).toMatch(/background-color:\s*var\(--lg-color-inset\)/);
    expect(derived.body).toMatch(/border-color:\s*transparent/);
  });

  it('Edge: no rule for a derived readout, its row or a dependent sets the accent as text colour', () => {
    const offenders = DERIVED_SURFACES.flatMap((css) => rules(css))
      .filter((rule) => DERIVED_SELECTOR.test(rule.selector))
      .filter((rule) => /accent/.test(colourOf(rule) ?? ''))
      .map((rule) => rule.selector);

    expect(offenders).toEqual([]);
  });

  it('Edge: a marked derived readout keeps the accent on its border only, not on its value', () => {
    const marked = rulesFor(engineCss, '.lg-field__readonly.lg-field--marked');

    expect(marked.length).toBeGreaterThan(0);
    for (const rule of marked) {
      expect(rule.body).toMatch(/border-color:\s*var\(--lg-color-accent\)/);
      expect(colourOf(rule)).toBeUndefined();
    }
  });

  it('Edge: a table cell inherits the one readout rule, with no colour override', () => {
    expect(
      rulesFor(engineCss, '.lg-field__readonly--derived.lg-field__readonly--numeric'),
    ).toHaveLength(1);
  });

  it('Edge: a derived row label stays muted and a settled dependent is text, a stale one muted and slanted', () => {
    const [label] = rulesFor(engineCss, '.lg-row--derived .lg-field-row__label');
    const [settled] = rulesFor(engineCss, '.lg-dependent__value');
    const [stale] = rulesFor(engineCss, '.lg-dependent--stale .lg-dependent__value');

    expect(colourOf(label)).toBe('var(--lg-color-text-muted)');
    expect(colourOf(settled)).toBe('var(--lg-color-text)');
    expect(colourOf(stale)).toBe('var(--lg-color-text-muted)');
    expect(stale.body).toMatch(/font-style:\s*italic/);
    expect(colourOf(settled)).not.toBe(colourOf(stale));
  });
});

function appWorld(): Map<string, string> {
  const css = readFileSync(tokensCss, 'utf8');
  const start = css.indexOf(':root:has(.lg-shell)');
  const block = css.slice(css.indexOf('{', start) + 1, css.indexOf('}', start));
  const declarations = new Map(readThemeTokenValues('light'));
  for (const match of block.matchAll(/(--lg-[a-z0-9-]+)\s*:\s*([^;]+);/g)) {
    declarations.set(match[1], match[2].trim());
  }
  const resolve = (value: string, seen: Set<string>): string => {
    const alias = value.match(/^var\((--lg-[a-z0-9-]+)\)$/);
    const target = alias ? declarations.get(alias[1]) : undefined;
    if (!alias || target === undefined || seen.has(alias[1])) {
      return value;
    }
    return resolve(target, new Set([...seen, alias[1]]));
  };
  return new Map([...declarations].map(([name, value]) => [name, resolve(value, new Set())]));
}

const worlds: readonly (readonly [string, () => Map<string, string>])[] = [
  ['light theme', () => resolveThemeTokenValues('light')],
  ['dark theme', () => resolveThemeTokenValues('dark')],
  ['signed-in app world', appWorld],
];

describe('derived readout contrast and non-colour cues', () => {
  for (const [name, tokens] of worlds) {
    it(`Happy path: value text on the recessed readout (--lg-color-bg) meets AA in the ${name}`, () => {
      const values = tokens();

      expect(
        contrastRatio(
          values.get('--lg-color-text') as string,
          values.get('--lg-color-bg') as string,
        ),
      ).toBeGreaterThanOrEqual(4.5);
    });

    it(`Edge: the stale dependent (muted text) and the muted label still meet AA on the surface in the ${name}`, () => {
      const values = tokens();

      expect(
        contrastRatio(
          values.get('--lg-color-text-muted') as string,
          values.get('--lg-color-surface') as string,
        ),
      ).toBeGreaterThanOrEqual(4.5);
    });

    it(`Edge: settled and stale dependents differ in colour in the ${name}`, () => {
      const values = tokens();

      expect(values.get('--lg-color-text')).not.toBe(values.get('--lg-color-text-muted'));
    });

    it(`Edge: derived reads as derived without colour in the ${name}: the recessed layer differs from the raised input`, () => {
      const values = tokens();

      expect(values.get('--lg-color-bg')).not.toBe(values.get('--lg-color-raised'));
      expect(values.get('--lg-color-border-strong')).not.toBe(values.get('--lg-color-bg'));
    });
  }
});
