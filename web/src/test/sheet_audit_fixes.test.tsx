import { afterEach, describe, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { cleanup, fireEvent, render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { parseSheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import type { FieldSchema } from '../api/models/template';
import { SheetFieldControl } from '../engine/field_control';
import { createMemorySheetStore } from '../library/sheet_store';
import { message } from '../l10n/strings';
import { centerInline, hiddenEdges, SectionIndex } from '../sheet/section_index';
import { createSheetEditStore, pendingCount } from '../sheet/sheet_edit';
import { findScroller } from '../sheet/use_active_section';
import { createViewStore, initialView, TEMPLATE_SETTINGS_KEYS } from '../sheet/view_store';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const here = path.dirname(fileURLToPath(import.meta.url));
const css = (relative: string): string => readFileSync(path.resolve(here, relative), 'utf8');

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function schema(overrides: Partial<FieldSchema> = {}): FieldSchema {
  return {
    type: 'number',
    label: 'Armor class',
    isEditable: false,
    defaultValue: 0,
    formula: 'combat_baseArmorClass + (modifiers_dexterityMod * 1)',
    ...overrides,
  } as FieldSchema;
}

describe('computed fields: accessible name is label and value, never the formula', () => {
  it('Happy path: a lone Row reads "Armor class 10" and keeps the formula as a tooltip', () => {
    render(
      <>
        <span id="row-label">Armor class</span>
        <SheetFieldControl
          path="derived.armorClass"
          schema={schema()}
          value={10}
          labelId="row-label"
        />
      </>,
    );

    const readout = screen.getByRole('group');

    expect(readout).toHaveAccessibleName('Armor class 10');
    expect(readout).not.toHaveAccessibleName(expect.stringContaining('combat_baseArmorClass'));
    expect(readout).toHaveAttribute('title', schema().formula);
  });

  it('Happy path: a table cell reads "Armor class: 10"', () => {
    render(<SheetFieldControl path="derived.armorClass" schema={schema()} value={10} />);

    expect(screen.getByRole('group')).toHaveAccessibleName('Armor class: 10');
  });

  it.each([
    ['Edge: an empty value', '', 'Armor class'],
    ['Edge: a null value', null, 'Armor class'],
    ['Edge: zero', 0, 'Armor class: 0'],
  ])('%s never leaks the formula into the name', (_name, value, expected) => {
    render(<SheetFieldControl path="derived.armorClass" schema={schema()} value={value} />);

    const readout = screen.getByRole('group');

    expect(readout).toHaveAccessibleName(expected);
    expect(readout).not.toHaveAccessibleName(expect.stringContaining('combat_'));
  });

  it('Edge: a derived readout carries the aria-hidden ƒ marker; an editable field does not', () => {
    const { container, rerender } = render(
      <SheetFieldControl path="derived.armorClass" schema={schema()} value={10} />,
    );
    expect(container.querySelector('.lg-field__fx')).toHaveAttribute('aria-hidden', 'true');

    rerender(
      <SheetFieldControl
        path="combat.hp"
        schema={schema({ isEditable: true, formula: undefined })}
        value={10}
        onChanged={() => undefined}
      />,
    );
    expect(container.querySelector('.lg-field__fx')).toBeNull();
  });
});

describe('keyboard focus ring', () => {
  it('Edge: shared buttons and steppers draw the full focus width, not a hairline', () => {
    expect(css('../ui/ui.css')).toMatch(
      /\.lg-button:focus-visible\s*\{[^}]*outline:\s*var\(--lg-focus-width\) solid var\(--lg-color-focus\)/,
    );
    expect(css('../engine/engine.css')).toMatch(
      /\.lg-field__step:focus-visible\s*\{[^}]*outline:\s*var\(--lg-focus-width\)/,
    );
  });

  it('Edge: the base layer rings every focusable element and no button rule removes it', () => {
    expect(css('../theme/base.css')).toMatch(
      /:focus-visible\s*\{[^}]*outline:\s*var\(--lg-focus-width\)/,
    );
    for (const file of [
      '../ui/ui.css',
      '../engine/engine.css',
      '../sheet/sheet.css',
      '../sheet/dice.css',
    ]) {
      expect(css(file)).not.toMatch(/button[^{]*\{[^}]*outline:\s*none/);
    }
  });
});

describe('disabled buttons are neutral', () => {
  it('Edge: the disabled rule resets every variant to the neutral surface, so Save is not orange', () => {
    const ui = css('../ui/ui.css');
    const disabled = ui.match(
      /\.lg-button:disabled,\s*\.lg-button\[aria-disabled='true'\]\s*\{([^}]*)\}/,
    );

    expect(disabled?.[1]).toContain('background: var(--lg-color-surface)');
    expect(disabled?.[1]).toContain('color: var(--lg-color-text-muted)');
    expect(css('../sheet/sheet.css')).not.toMatch(/lg-button--primary:disabled/);
  });
});

describe('document scrolling', () => {
  it('Edge: the content region and the shell no longer own a scrollbar', () => {
    const shell = css('../shell/shell.css');
    const content = shell.match(/\n\.lg-content\s*\{([^}]*)\}/)?.[1] ?? '';

    expect(content).not.toMatch(/overflow/);
    expect(shell.match(/(?:^|\n)\.lg-shell\s*\{([^}]*)\}/)?.[1] ?? '').not.toMatch(
      /\bheight:\s*100%/,
    );
  });

  it('Edge: sections land clear of the sticky strip', () => {
    expect(css('../theme/base.css')).toMatch(/scroll-padding-top/);
  });

  it('Edge: a content region that does not scroll is not the scroller (the viewport is)', () => {
    const { container } = render(
      <div className="lg-content">
        <div id="strip" />
      </div>,
    );

    expect(findScroller(container.querySelector('#strip') as Element)).toBeNull();
  });

  it('Happy path: a content region that scrolls is the scroller', () => {
    const { container } = render(
      <div className="lg-content" style={{ overflowY: 'auto' }}>
        <div id="strip" />
      </div>,
    );

    expect(findScroller(container.querySelector('#strip') as Element)).toBe(
      container.querySelector('.lg-content'),
    );
  });

  it('Edge: a strip outside any content region has no scroller', () => {
    const { container } = render(<div id="strip" />);

    expect(findScroller(container.querySelector('#strip') as Element)).toBeNull();
  });
});

const LINKS = [
  { key: 'identity', label: 'Identity' },
  { key: 'stats', label: 'Stats' },
  { key: 'skills', label: 'Skills' },
];

class StubObserver {
  static last: StubObserver | undefined;
  constructor(
    readonly callback: IntersectionObserverCallback,
    readonly options?: IntersectionObserverInit,
  ) {
    StubObserver.last = this;
  }
  observe() {}
  disconnect() {}
  unobserve() {}
  takeRecords() {
    return [];
  }
}

describe('section strip with the document as the scroller', () => {
  it('Happy path: the observer root is the viewport and the last Section wins at the page end', () => {
    vi.stubGlobal('IntersectionObserver', StubObserver);
    render(
      <div className="lg-content">
        <div className="lg-sheet">
          <SectionIndex
            sheetName="Aragorn"
            onTop={() => undefined}
            sections={LINKS}
            onlyPinned={false}
            filterDisabled={false}
            pinnedCount={0}
            onSetOnlyPinned={() => undefined}
            onJump={() => undefined}
            onEditPins={() => undefined}
            epoch="play"
          />
          {LINKS.map((link) => (
            <section key={link.key} data-section-key={link.key} />
          ))}
        </div>
      </div>,
    );
    expect(StubObserver.last?.options?.root).toBeNull();

    const root = document.documentElement;
    Object.defineProperty(root, 'scrollHeight', { value: 2000, configurable: true });
    Object.defineProperty(root, 'clientHeight', { value: 800, configurable: true });
    Object.defineProperty(root, 'scrollTop', { value: 1200, configurable: true });
    act(() => {
      fireEvent.scroll(window);
    });

    expect(screen.getByRole('button', { name: /Skills/ })).toHaveAttribute(
      'aria-current',
      'location',
    );
    Reflect.deleteProperty(root, 'scrollHeight');
    Reflect.deleteProperty(root, 'clientHeight');
    Reflect.deleteProperty(root, 'scrollTop');
  });
});

describe('section strip overflow affordance', () => {
  it('Edge: a row that fits hides no edge', () => {
    expect(hiddenEdges({ scrollLeft: 0, scrollWidth: 300, clientWidth: 300 })).toEqual({
      start: false,
      end: false,
    });
  });

  it('Edge: at the start only the end is hidden; at the end only the start; between, both', () => {
    expect(hiddenEdges({ scrollLeft: 0, scrollWidth: 900, clientWidth: 300 })).toEqual({
      start: false,
      end: true,
    });
    expect(hiddenEdges({ scrollLeft: 600, scrollWidth: 900, clientWidth: 300 })).toEqual({
      start: true,
      end: false,
    });
    expect(hiddenEdges({ scrollLeft: 200, scrollWidth: 900, clientWidth: 300 })).toEqual({
      start: true,
      end: true,
    });
  });

  it('Edge: a one pixel overshoot still counts as the edge', () => {
    expect(hiddenEdges({ scrollLeft: 599, scrollWidth: 900, clientWidth: 300 }).end).toBe(false);
  });

  it('Happy path: the active link is scrolled to the centre of the row', () => {
    const nav = document.createElement('nav');
    const chip = document.createElement('button');
    nav.getBoundingClientRect = () => ({ left: 0, width: 300 }) as DOMRect;
    chip.getBoundingClientRect = () => ({ left: 400, width: 100 }) as DOMRect;

    centerInline(nav, chip);

    expect(nav.scrollLeft).toBe(300);
  });

  it('Edge: a link already centred moves nothing', () => {
    const nav = document.createElement('nav');
    const chip = document.createElement('button');
    nav.getBoundingClientRect = () => ({ left: 0, width: 300 }) as DOMRect;
    chip.getBoundingClientRect = () => ({ left: 100, width: 100 }) as DOMRect;

    centerInline(nav, chip);

    expect(nav.scrollLeft).toBe(0);
  });

  it('Edge: the strip fades only an edge that hides links', () => {
    const strip = css('../sheet/section_index.css');

    expect(strip).toContain('.lg-sectionbar__nav[data-edge-start]');
    expect(strip).toContain('.lg-sectionbar__nav[data-edge-end]');
  });
});

describe('template settings start collapsed', () => {
  const KEYS = ['identity', 'stats', 'rules', 'config'];

  it('Edge: a sheet with no remembered view collapses the settings Sections only', () => {
    expect(initialView(KEYS).collapsed).toEqual(['rules', 'config']);
    expect(TEMPLATE_SETTINGS_KEYS).toEqual(['rules', 'config']);
  });

  it('Edge: a template without settings Sections starts with everything open', () => {
    expect(initialView(['identity', 'stats']).collapsed).toEqual([]);
    expect(initialView([]).collapsed).toEqual([]);
  });

  it('Edge: a store with no storage still starts with the settings collapsed', () => {
    const store = createViewStore({ sheetId: 's', sectionKeys: KEYS, storage: undefined });

    expect(store.getState().collapsed).toEqual(['rules', 'config']);
  });

  it('Edge: a remembered view wins, including one that opened the settings', () => {
    const storage = {
      getItem: () =>
        JSON.stringify({ v: 1, collapsed: [], onlyPinned: false, explainerSeen: true }),
      setItem: () => undefined,
    };
    const store = createViewStore({ sheetId: 's', sectionKeys: KEYS, storage });

    expect(store.getState().collapsed).toEqual([]);
  });

  it('Edge: a store whose storage throws falls back to the initial view', () => {
    const storage = {
      getItem: () => {
        throw new Error('blocked');
      },
      setItem: () => undefined,
    };
    const store = createViewStore({ sheetId: 's', sectionKeys: KEYS, storage });

    expect(store.getState().collapsed).toEqual(['rules', 'config']);
  });

  it('Happy path: opening a settings Section is remembered and expands it', () => {
    const store = createViewStore({ sheetId: 's', sectionKeys: KEYS, storage: undefined });

    store.toggle('rules');

    expect(store.getState().collapsed).toEqual(['config']);
  });
});

describe('the change count', () => {
  function editStore() {
    const fake = createFakeTransport(successOutcome({}));
    return createSheetEditStore(
      {
        template: parseTemplateDetail(templateDetailPayload()),
        sheet: parseSheet(sheetPayload()),
      },
      {
        client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
        store: createMemorySheetStore(),
      },
    );
  }

  it('Edge: a fresh store has no changes', () => {
    expect(pendingCount(editStore().getState())).toBe(0);
  });

  it('Edge: editing a value back to its saved value is not a change', () => {
    const store = editStore();

    store.setPath('identity.level', 9);
    store.setPath('identity.level', 3);

    expect(pendingCount(store.getState())).toBe(0);
  });

  it('Happy path: each edited field counts once, however often it is edited', () => {
    const store = editStore();

    store.setPath('identity.level', 4);
    store.setPath('identity.level', 5);
    store.setPath('identity.characterName', 'Strider');

    expect(pendingCount(store.getState())).toBe(2);
  });
});

describe('the sheet footer', () => {
  async function open() {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    renderApp(`/sheets/${SHEET_ID}`, {
      client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
    });
    await screen.findByTestId('sheet-name');
    return userEvent.setup();
  }

  it('Edge: with no edits the footer is quiet, shows no count and both actions are disabled', async () => {
    await open();

    expect(document.querySelector('.lg-sheet__footer')).toHaveClass('lg-sheet__footer--clean');
    expect(screen.queryByTestId('sheet-change-count')).toBeNull();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(screen.getByRole('button', { name: message('sheet.preview') })).toBeDisabled();
  });

  it('Happy path: one edit lifts the footer and counts "1 change"; a second field says "2 changes"', async () => {
    const user = await open();
    const level = screen.getByRole('textbox', { name: 'Level' });

    await user.clear(level);
    await user.type(level, '7');

    expect(document.querySelector('.lg-sheet__footer')).toHaveClass('lg-sheet__footer--dirty');
    expect(screen.getByTestId('sheet-change-count')).toHaveTextContent('1 change');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();

    const name = screen.getByRole('textbox', { name: 'Name' });
    await user.type(name, 'x');

    expect(screen.getByTestId('sheet-change-count')).toHaveTextContent('2 changes');
  });

  it('Edge: reverting the only edit hides the count again', async () => {
    const user = await open();
    const level = screen.getByRole('textbox', { name: 'Level' });

    await user.clear(level);
    await user.type(level, '7');
    await user.clear(level);
    await user.type(level, '3');

    expect(screen.queryByTestId('sheet-change-count')).toBeNull();
  });
});

describe('text and number alignment', () => {
  it('Edge: text inputs are left-aligned and only numeric inputs are right-aligned', () => {
    const engine = css('../engine/engine.css');

    expect(engine).toMatch(/\ninput\.lg-field\s*\{[^}]*text-align:\s*left/);
    expect(engine).toMatch(/\ninput\.lg-field--numeric\s*\{[^}]*text-align:\s*right/);
  });

  it('Edge: table text cells are wider than number cells', () => {
    const engine = css('../engine/engine.css');

    expect(engine).toMatch(/\.lg-field-table__cell\s*\{[^}]*flex:\s*2 1 0/);
    expect(engine).toMatch(/\.lg-field-table__cell--number\s*\{[^}]*flex:\s*1 1 0/);
  });
});
