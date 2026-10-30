import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { buildSections, rowFieldPaths, sectionKeyOfRow } from '../engine/field_grouping';
import { revealFirstRecalculated, revealRow, revealSection } from '../sheet/reveal_row';
import type { RevealView } from '../sheet/reveal_row';
import { ARRANGE_FIELDS } from './layout_fixtures';

function view(onReveal: (key: string) => void = () => undefined): RevealView {
  return { reveal: vi.fn(onReveal), showAll: vi.fn() };
}

type Child = HTMLElement | string;

function el(tag: string, attrs: Record<string, string> = {}, ...children: Child[]): HTMLElement {
  const element = document.createElement(tag);
  for (const [name, value] of Object.entries(attrs)) {
    element.setAttribute(name, value);
  }
  element.append(...children);
  return element;
}

function section(key: string, ...inner: HTMLElement[]): HTMLElement {
  const toggle = el('button', { class: 'lg-section__toggle' }, key);
  return el('section', { 'data-section-key': key }, el('h2', {}, toggle), ...inner);
}

function row(key: string, mark?: string): HTMLElement {
  const badge = mark === undefined ? [] : [el('span', { 'data-mark': mark }, mark)];
  return el('div', { 'data-row-key': key }, key, ...badge);
}

function mount(...nodes: HTMLElement[]): void {
  document.body.replaceChildren(...nodes);
}

let scrolled: Element[];

beforeEach(() => {
  scrolled = [];
  document.body.replaceChildren();
  Element.prototype.scrollIntoView = vi.fn(function (this: Element) {
    scrolled.push(this);
  });
});

afterEach(() => {
  document.body.replaceChildren();
});

describe('sectionKeyOfRow', () => {
  const tests: { name: string; key: string; expected: string }[] = [
    {
      name: 'Happy path: a lone field key names its first segment',
      key: 'alpha.one',
      expected: 'alpha',
    },
    {
      name: 'Happy path: an exclusive key names the part before the hash',
      key: 'beta#pick',
      expected: 'beta',
    },
    {
      name: 'Edge: a nested exclusive key names the top-level segment',
      key: 'beta.inner#pick',
      expected: 'beta',
    },
    { name: 'Edge: a top-level leaf is its own section', key: 'solo', expected: 'solo' },
    { name: 'Edge: an empty key stays empty', key: '', expected: '' },
  ];
  for (const tc of tests) {
    it(tc.name, () => {
      expect(sectionKeyOfRow(tc.key)).toBe(tc.expected);
    });
  }
});

describe('rowFieldPaths', () => {
  it('Happy path: every Row kind lists the paths it draws, in order', () => {
    const rows = buildSections(ARRANGE_FIELDS).flatMap((entry) => entry.rows);

    expect(rows.map(rowFieldPaths)).toEqual([
      ['alpha.one'],
      ['alpha.two'],
      ['alpha.three'],
      ['beta.note'],
      ['beta.stats.str', 'beta.stats.dex'],
      ['beta.pickA', 'beta.pickB'],
      ['gamma.solo'],
    ]);
  });
});

describe('revealRow with the view', () => {
  it('Happy path: the view is asked for the Row Section before the Row is looked up', () => {
    const order: string[] = [];
    mount(section('alpha'));
    const controls = view((key) => {
      order.push(`reveal:${key}`);
      document.querySelector('[data-section-key="alpha"]')!.append(row('alpha.one'));
    });

    const found = revealRow('alpha.one', document, controls);

    expect(found).toBe(true);
    expect(order).toEqual(['reveal:alpha']);
    expect(document.querySelector('[data-row-key="alpha.one"]')).toHaveFocus();
    expect(scrolled).toHaveLength(1);
  });

  it('Edge: without a view it behaves as before', () => {
    mount(section('alpha', row('alpha.one')));

    expect(revealRow('alpha.one')).toBe(true);
    expect(revealRow('alpha.ghost')).toBe(false);
  });

  it('Edge: a stale key returns false and moves nothing', () => {
    mount(section('alpha', row('alpha.one')));

    expect(revealRow('ghost.row', document, view())).toBe(false);
    expect(scrolled).toHaveLength(0);
  });
});

describe('revealSection', () => {
  it('Happy path: scrolls the Section to the start and focuses its title control', () => {
    mount(section('alpha'), section('beta'));

    const found = revealSection('beta');

    expect(found).toBe(true);
    expect(scrolled).toEqual([document.querySelector('[data-section-key="beta"]')]);
    expect(document.querySelector('[data-section-key="beta"] .lg-section__toggle')).toHaveFocus();
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ block: 'start' });
  });

  it('Edge: opens the Section through the view before scrolling', () => {
    mount();
    const controls = view((key) => {
      mount(section(key));
    });

    expect(revealSection('alpha', document, controls)).toBe(true);
    expect(controls.reveal).toHaveBeenCalledWith('alpha');
    expect(scrolled).toHaveLength(1);
  });

  it('Edge: focuses the Arrange handle when the title is not a disclosure control', () => {
    mount(
      el(
        'section',
        { 'data-section-key': 'alpha' },
        el('button', { class: 'lg-arrange-handle' }, 'grip'),
      ),
    );

    revealSection('alpha');

    expect(document.querySelector('.lg-arrange-handle')).toHaveFocus();
  });

  it('Edge: a Section with no control is still scrolled to, and nothing throws', () => {
    mount(el('section', { 'data-section-key': 'alpha' }));

    expect(revealSection('alpha')).toBe(true);
    expect(scrolled).toHaveLength(1);
  });

  it('Edge: an unknown Section returns false and moves nothing', () => {
    mount(section('alpha'));

    expect(revealSection('ghost')).toBe(false);
    expect(scrolled).toHaveLength(0);
    expect(document.body).toHaveFocus();
  });
});

describe('revealFirstRecalculated', () => {
  it('Happy path: shows every Row first, then reveals the first recalculated one', () => {
    mount(
      section(
        'alpha',
        row('alpha.one', 'modified'),
        row('alpha.two', 'recalculated'),
        row('alpha.three', 'recalculated'),
      ),
    );
    const controls = view();

    expect(revealFirstRecalculated(document, controls)).toBe(true);

    expect(controls.showAll).toHaveBeenCalledTimes(1);
    expect(document.querySelector('[data-row-key="alpha.two"]')).toHaveFocus();
  });

  it('Edge: no recalculated Row returns false', () => {
    mount(section('alpha', row('alpha.one', 'modified')));

    expect(revealFirstRecalculated(document, view())).toBe(false);
    expect(scrolled).toHaveLength(0);
  });
});
