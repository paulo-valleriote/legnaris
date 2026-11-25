import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const css = readFileSync(path.resolve(here, '../landing/landing.css'), 'utf8');

const HERO_COPY_SELECTORS = [
  '.lg-landing__bar',
  '.lg-landing__badge',
  '.lg-landing__hero-title',
  '.lg-landing__hero-lead',
  '.lg-landing__hero-body .lg-landing__actions',
];

const HERO_SHEET_SELECTOR = '.lg-landing__hero-body > .lg-chain';

function blockFor(selector: string): string {
  const start = css.indexOf(selector);
  expect(start, `${selector} is declared`).toBeGreaterThanOrEqual(0);
  return css.slice(start, css.indexOf('}', start));
}

function animationFor(selector: string): string {
  const declaration = /animation:([^;]+);/.exec(blockFor(selector));
  expect(declaration, `${selector} declares an animation`).not.toBeNull();
  return declaration === null ? '' : declaration[1].replace(/\s+/g, ' ').trim();
}

function keyframesFor(name: string): string {
  const start = css.indexOf(`@keyframes ${name}`);
  expect(start, `${name} is declared`).toBeGreaterThanOrEqual(0);
  const open = css.indexOf('{', start);
  let depth = 0;
  for (let index = open; index < css.length; index += 1) {
    if (css[index] === '{') {
      depth += 1;
    }
    if (css[index] === '}') {
      depth -= 1;
      if (depth === 0) {
        return css.slice(open, index + 1);
      }
    }
  }
  throw new Error(`${name} is never closed`);
}

function delaySteps(animation: string): number {
  const multiplier = /calc\(var\(--lg-motion-120\) \* (\d+)\)/.exec(animation);
  if (multiplier !== null) {
    return Number(multiplier[1]);
  }
  return animation.includes('var(--lg-motion-120)') ? 1 : 0;
}

describe('the landing hero motion', () => {
  it('Happy path: the hero copy lifts into place without waiting on opacity', () => {
    for (const selector of HERO_COPY_SELECTORS) {
      const animation = animationFor(selector);

      expect(animation, `${selector} lifts`).toContain('lg-landing-lift');
      expect(animation, `${selector} does not rise`).not.toContain('lg-landing-rise');
      expect(animation, `${selector} keeps the entrance duration`).toContain(
        'var(--lg-motion-600)',
      );
      expect(animation, `${selector} keeps the entrance easing`).toContain('var(--lg-ease-out)');
      expect(animation, `${selector} keeps its final frame`).toContain('both');
    }
  });

  it('Edge: the liftoff keyframe moves the element and never fades it', () => {
    const lift = keyframesFor('lg-landing-lift');

    expect(lift).not.toContain('opacity');
    expect(lift).toContain('transform: translateY(var(--lg-space-8))');
    expect(lift).toContain('transform: none');
  });

  it('Edge: no hero copy rule waits longer than two motion steps', () => {
    for (const selector of HERO_COPY_SELECTORS) {
      expect(delaySteps(animationFor(selector)), `${selector} delay`).toBeLessThanOrEqual(2);
    }

    for (const selector of ['.lg-landing__bar', '.lg-landing__badge', '.lg-landing__hero-title']) {
      expect(delaySteps(animationFor(selector)), `${selector} starts at once`).toBe(0);
    }
    expect(delaySteps(animationFor('.lg-landing__hero-lead'))).toBe(1);
    expect(delaySteps(animationFor('.lg-landing__hero-body .lg-landing__actions'))).toBe(2);
    expect(delaySteps(animationFor(HERO_SHEET_SELECTOR))).toBe(2);
  });

  it('Happy path: the hero Strength chain lifts in without waiting on opacity', () => {
    const animation = animationFor(HERO_SHEET_SELECTOR);

    expect(animation).toContain('lg-landing-lift');
    expect(animation).not.toContain('lg-landing-rise');
  });

  it('Edge: no landing rule fades the hero in from transparent', () => {
    expect(css).not.toContain('@keyframes lg-landing-rise');
  });

  it('Edge: the hero carries no lazy placeholder to wait on', () => {
    expect(css).not.toContain('.lg-landing__hero-body > .lg-landing__demo-loading');
  });
});
