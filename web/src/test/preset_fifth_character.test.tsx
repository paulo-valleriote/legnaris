import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ResetAction } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';
import type { Sheet } from '../api/models/sheet';
import { parseSheet, valueAtPath } from '../api/models/sheet';
import { message, messageWith } from '../l10n/strings';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { PresetPlayView } from '../sheet/preset/play_view';
import { PRESET_CONFIGS } from '../sheet/preset/registry';
import { numberValue } from '../sheet/preset/values';
import { setViewportWidth } from './match_media';
import { renderPreset } from './preset_fixtures';
import goldenCharacter from '../../../testdata/templates/fifth_character_v1.json';

const TEMPLATE_ID = 'fifth_character_v1';

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
});

function goldenTemplate() {
  return parseTemplateDetail(goldenCharacter.data);
}

function fifthSheet(spellcasting?: Record<string, unknown>): Sheet {
  const sheet = parseSheet({
    id: '01a14236-21c9-7546-945e-40c46d1fcd71',
    template_id: TEMPLATE_ID,
    kind: 'character',
    name: 'Brenna',
    data: {
      identity: {
        name: 'Brenna',
        class: 'Ranger',
        subclass: 'Hunter',
        species: 'Human',
        background: 'Outlander',
        level: 5,
        proficiencyBonus: 3,
        jackOfAllTrades: 0,
      },
      abilities: {
        strength: { score: 12, mod: 1 },
        dexterity: { score: 16, mod: 3 },
        constitution: { score: 14, mod: 2 },
        intelligence: { score: 10, mod: 0 },
        wisdom: { score: 13, mod: 1 },
        charisma: { score: 8, mod: -1 },
      },
      saves: {
        strength: { prof: 1, misc: 0, total: 4 },
        dexterity: { prof: 0, misc: 0, total: 3 },
        constitution: { prof: 1, misc: 0, total: 5 },
        intelligence: { prof: 0, misc: 0, total: 0 },
        wisdom: { prof: 0, misc: 0, total: 1 },
        charisma: { prof: 0, misc: 0, total: -1 },
      },
      skills: {
        arcana: { rank: 0, misc: 0, total: -4 },
        athletics: { rank: 1, misc: 0, total: 4 },
        stealth: { rank: 2, misc: 0, total: 9 },
      },
      exhaustion: { level: 2, penalty: -4 },
      passive: { perception: 11 },
      armor: { armorClass: 15 },
      combat: {
        initiative: { alert: 0, misc: 0, total: -1 },
        speed: { base: 30, total: 20 },
        hp: { base: 36, perLevel: 8, misc: 0, max: 40, current: 28, temp: 4 },
      },
      status: {
        deathSaves: { successes: 1, failures: 0 },
        conditions: { blinded: 0, poisoned: 1 },
      },
      hitDice: {
        primary: { size: 10, count: 5, spent: 2, remaining: 3 },
        d8: { extra: 1, spent: 1, remaining: 0 },
      },
      spellcasting: {
        slot1: { total: 4, spent: 1, remaining: 3 },
        pact: { count: 2, level: 1, used: 1, remaining: 1 },
      },
      attacks: {},
      inventory: {},
      features: {},
      spells: {},
      resources: {},
    },
    dice: { section: {}, row: {} },
    created_at: '2026-11-20T10:00:00Z',
    updated_at: '2026-11-20T10:00:00Z',
  });
  return spellcasting === undefined ? sheet : { ...sheet, data: { ...sheet.data, spellcasting } };
}

function renderSlots(spellcasting?: Record<string, unknown>) {
  const preset = presetFromConfig(fifthCharacterConfig);
  return renderPreset(<PresetPlayView preset={preset} />, {
    template: goldenTemplate(),
    sheet: fifthSheet(spellcasting),
  });
}

function resetBinding(
  run: (action: ResetAction, requestId: string) => Promise<Record<string, unknown>>,
) {
  return {
    run,
    onApplied: vi.fn(),
    refresh: vi.fn(async () => undefined),
    requestId: () => 'request-1',
  };
}

function renderPlay() {
  const preset = presetFromConfig(fifthCharacterConfig);
  const run = vi.fn(async () => ({}));
  const binding = resetBinding(run);
  const rendered = renderPreset(<PresetPlayView preset={preset} />, {
    template: goldenTemplate(),
    sheet: fifthSheet(),
    reset: binding,
  });
  return { ...rendered, preset, run, binding };
}

describe('the 5E Character preset config', () => {
  it('Happy path: every bound path exists in the fifth character golden', () => {
    expect(fifthCharacterConfig.templateId).toBe(TEMPLATE_ID);
    expect(configPathErrors(fifthCharacterConfig, goldenTemplate())).toEqual([]);
  });

  it('Happy path: the registry resolves the fifth character template', () => {
    expect(PRESET_CONFIGS).toContain(fifthCharacterConfig);
  });

  it('Happy path: Play draws every placed piece', () => {
    renderPlay();

    const pieces = fifthCharacterConfig.pieces;
    const data = fifthSheet().data;
    const limitOf = (max: string | number) =>
      typeof max === 'number' ? max : (numberValue(valueAtPath(data, max)) ?? 0);
    const pipsCount =
      pieces.filter((piece) => piece.kind === 'pips' && limitOf(piece.max) > 0).length +
      pieces.filter(
        (piece) =>
          piece.kind === 'conditions' &&
          piece.exhaustion !== undefined &&
          limitOf(piece.exhaustion.max) > 0,
      ).length +
      pieces
        .flatMap((piece) =>
          piece.kind === 'spellSlots'
            ? [...piece.levels, ...(piece.pact === undefined ? [] : [piece.pact])]
            : [],
        )
        .filter((slot) => limitOf(slot.max) > 0).length;

    expect(document.querySelectorAll('.lg-compose-pips')).toHaveLength(pipsCount);
    expect(screen.queryByText('Level 9 spell slots spent')).not.toBeInTheDocument();
    expect(screen.queryByText(messageWith('sheet.play.level', { count: 9 }))).toBeNull();
    expect(document.querySelectorAll('[data-testid="compose-record-table"]')).toHaveLength(
      pieces.filter((piece) => piece.kind === 'recordTable').length,
    );
    expect(document.querySelectorAll('[data-testid="compose-stat-pills"]')).toHaveLength(
      pieces.filter((piece) => piece.kind === 'statPills').length,
    );
    expect(screen.getByTestId('compose-hit-points')).toBeInTheDocument();
    expect(screen.getByTestId('compose-ability-grid')).toBeInTheDocument();
    expect(screen.getByTestId('compose-save-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-skill-list')).toBeInTheDocument();
    expect(screen.getByTestId('compose-conditions')).toBeInTheDocument();
    expect(screen.queryAllByTestId('compose-record-row')).toHaveLength(0);
    expect(screen.getByRole('group', { name: message('sheet.reset.group') })).toBeInTheDocument();
    expect(screen.queryByTestId('compose-roll-mode')).toBeNull();
    expect(document.querySelector('.lg-preset__columns--compose')).not.toBeNull();
  });

  it('Happy path: the header carries the roll-mode switch beside the name', () => {
    const preset = presetFromConfig(fifthCharacterConfig);
    renderPreset(<preset.Header />, { template: goldenTemplate(), sheet: fifthSheet() });

    const header = document.querySelector('.lg-compose-header');
    const trailing = header?.querySelector('.lg-compose-header__trailing');
    expect(trailing?.querySelector('[data-testid="compose-roll-mode"]')).not.toBeNull();
    expect(header?.querySelector('[data-testid="sheet-name"]')).not.toBeNull();
  });

  it('Edge: the header trailing piece is not drawn again as a Play section', () => {
    const preset = presetFromConfig(fifthCharacterConfig);

    expect(preset.sections.map((section) => section.key)).not.toContain('rollMode');
  });

  it('Happy path: the wide and narrow layouts place every section exactly once', () => {
    const preset = presetFromConfig(fifthCharacterConfig);
    const sectionKeys = new Set(preset.sections.map((section) => section.key));
    const placed = (
      blocks: readonly { sections?: readonly string[]; columns?: readonly (readonly string[])[] }[],
    ) =>
      blocks.flatMap((block) =>
        block.columns === undefined ? [...(block.sections ?? [])] : block.columns.flat(),
      );

    expect(placed(preset.layout ?? []).sort()).toEqual([...sectionKeys].sort());
    expect(placed(preset.narrowLayout ?? []).sort()).toEqual([...sectionKeys].sort());
  });

  it('Happy path: the desktop Play draws the grouped jump bar at the top of the body', () => {
    renderPlay();

    const nav = screen.getByTestId('sheet-section-nav');
    expect(
      within(nav)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual([
      message('preset.nav.abilities'),
      message('preset.nav.combat'),
      message('preset.nav.items'),
      message('preset.nav.spells'),
      message('preset.nav.resources'),
    ]);
    expect(nav).toBe(document.querySelector('.lg-preset__body')?.firstElementChild);
  });

  it('Happy path: the phone Play draws the grouped jump bar after identity', () => {
    setViewportWidth(390);
    renderPlay();

    const nav = screen.getByTestId('sheet-section-nav');
    expect(
      within(nav)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual([
      message('preset.nav.abilities'),
      message('preset.nav.combat'),
      message('preset.nav.items'),
      message('preset.nav.spells'),
      message('preset.nav.resources'),
    ]);
    expect(nav.previousElementSibling).toBe(
      document.querySelector('[data-preset-section="identity"]'),
    );
  });

  it('Happy path: the phone Play opens on hit points, then death saves and hit dice', () => {
    setViewportWidth(390);
    renderPlay();

    const order = Array.from(document.querySelectorAll('[data-preset-section]'))
      .map((section) => section.getAttribute('data-preset-section'))
      .slice(0, 4);
    expect(order).toEqual([
      'hitPoints',
      'deathSavesSuccesses',
      'deathSavesFailures',
      'hitDicePrimary',
    ]);
  });

  it('Happy path: every jump target resolves to a rendered section', () => {
    renderPlay();

    const targets = fifthCharacterConfig.sectionNav?.targets ?? [];
    expect(targets.length).toBeGreaterThan(0);
    for (const target of targets) {
      const resolved = target.sections.some((key) => {
        const section = document.querySelector(`[data-preset-section="${key}"]`);
        return section !== null && section.childElementCount > 0;
      });
      expect(resolved).toBe(true);
    }
  });

  it('Happy path: a grouped jump reveals and focuses its first present section', async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView =
      scrollIntoView as unknown as typeof Element.prototype.scrollIntoView;
    const user = userEvent.setup();
    renderPlay();

    await user.click(screen.getByRole('button', { name: /^Go to Combat/ }));

    const target = document.querySelector<HTMLElement>('[data-preset-section="attacks"]');
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'smooth' });
    expect(target?.contains(document.activeElement)).toBe(true);
  });

  it('Edge: on load the first chip is current, not Combat', () => {
    const tops: Record<string, number> = {
      abilities: 400,
      attacks: 1600,
      inventory: 2200,
      features: 2500,
      spellSlots: 2700,
      spells: 2800,
    };
    const original = Element.prototype.getBoundingClientRect;
    Element.prototype.getBoundingClientRect = function rect(this: Element) {
      const key = this.closest('[data-preset-section]')?.getAttribute('data-preset-section') ?? '';
      return { top: tops[key] ?? 0 } as DOMRect;
    };
    try {
      renderPlay();
      const nav = screen.getByTestId('sheet-section-nav');
      const current = within(nav)
        .getAllByRole('button')
        .filter((button) => button.getAttribute('aria-current') === 'location');
      expect(current.map((button) => button.textContent)).toEqual([
        message('preset.nav.abilities'),
      ]);
    } finally {
      Element.prototype.getBoundingClientRect = original;
    }
  });

  it('Happy path: a clicked chip stays current once its section reaches the scroll margin', async () => {
    const tops: Record<string, number> = {
      abilities: -900,
      attacks: 140,
      inventory: 900,
      spellSlots: 1450,
      spells: 1500,
      features: 2100,
    };
    const original = Element.prototype.getBoundingClientRect;
    const originalStyle = window.getComputedStyle;
    Element.prototype.getBoundingClientRect = function rect(this: Element) {
      const key = this.closest<HTMLElement>('[data-preset-section]')?.dataset.presetSection ?? '';
      return { top: tops[key] ?? 0 } as DOMRect;
    };
    const styleSpy = vi
      .spyOn(window, 'getComputedStyle')
      .mockImplementation((element: Element, pseudo?: string | null) => {
        const style = originalStyle(element, pseudo);
        if (element.closest('[data-preset-section]') !== null) {
          Object.defineProperty(style, 'scrollMarginBlockStart', { value: '140px' });
        }
        return style;
      });
    Element.prototype.scrollIntoView =
      vi.fn() as unknown as typeof Element.prototype.scrollIntoView;
    try {
      const user = userEvent.setup();
      renderPlay();
      await user.click(screen.getByRole('button', { name: /^Go to Combat/ }));
      act(() => {
        window.dispatchEvent(new Event('scroll'));
      });
      const nav = screen.getByTestId('sheet-section-nav');
      const current = within(nav)
        .getAllByRole('button')
        .filter((button) => button.getAttribute('aria-current') === 'location');
      expect(current.map((button) => button.textContent)).toEqual([message('preset.nav.combat')]);
    } finally {
      Element.prototype.getBoundingClientRect = original;
      styleSpy.mockRestore();
    }
  });

  it('Happy path: a skill roll uses the Exhaustion penalised dice', async () => {
    const user = userEvent.setup();
    renderPlay();

    await user.click(screen.getByRole('button', { name: /^Roll Arcana/ }));

    const panel = document.querySelector('.lg-dice-panel');
    expect(panel).not.toBeNull();
    expect(panel?.textContent).toContain('Exhaustion penalty');
  });

  it('Happy path: an ability, a save and the initiative pill roll from their bound Row', async () => {
    const user = userEvent.setup();
    renderPlay();

    await user.click(screen.getByRole('button', { name: /^Roll Strength modifier/ }));
    await user.click(
      within(screen.getByTestId('compose-save-saves.dexterity.total')).getByRole('button', {
        name: /^Roll Dexterity save/,
      }),
    );
    await user.click(screen.getByRole('button', { name: /^Roll Initiative/ }));

    const panel = document.querySelector('.lg-dice-panel')?.textContent ?? '';
    expect(panel).toContain('Strength modifier');
    expect(panel).toContain('Dexterity save total');
    expect(panel).toContain('Initiative');
  });

  it('Happy path: a long rest goes through the existing Reset path', async () => {
    const user = userEvent.setup();
    const { run } = renderPlay();

    await user.click(screen.getByRole('button', { name: 'Long rest' }));

    expect(run).toHaveBeenCalledTimes(1);
    expect(run).toHaveBeenCalledWith(expect.objectContaining({ id: 'long_rest' }), 'request-1');
  });

  describe('the 5E spell slots group', () => {
    it('Happy path: titled rows per level with slots, the pact row and the higher-levels note', () => {
      renderSlots();

      const group = screen.getByTestId('compose-spell-slots');
      expect(
        within(group).getByRole('heading', { name: message('sheet.play.spellSlots') }),
      ).toBeInTheDocument();
      expect(
        within(group).getByText(messageWith('sheet.play.level', { count: 1 })),
      ).toBeInTheDocument();
      expect(group.querySelectorAll('.lg-compose-pips')).toHaveLength(2);
      expect(within(group).getByText(message('preset.slots.more'))).toBeInTheDocument();
    });

    it('Edge: levels without slots draw no row', () => {
      renderSlots({ slot1: { total: 4, spent: 0 }, slot2: { total: 0, spent: 0 } });

      const group = screen.getByTestId('compose-spell-slots');
      expect(within(group).queryByText(messageWith('sheet.play.level', { count: 2 }))).toBeNull();
    });

    it('Edge: with the highest level filled the higher-levels note disappears', () => {
      renderSlots({ slot1: { total: 4, spent: 0 }, slot9: { total: 1, spent: 0 } });

      const group = screen.getByTestId('compose-spell-slots');
      expect(
        within(group).getByText(messageWith('sheet.play.level', { count: 9 })),
      ).toBeInTheDocument();
      expect(within(group).queryByText(message('preset.slots.more'))).toBeNull();
    });

    it('Edge: a sheet with no slots at all draws no pip row in the group', () => {
      renderSlots({});

      expect(
        screen.getByTestId('compose-spell-slots').querySelectorAll('.lg-compose-pips'),
      ).toHaveLength(0);
    });

    it('Happy path: tapping a level pip spends a slot through the edit store', async () => {
      const user = userEvent.setup();
      const { store } = renderSlots({ slot1: { total: 4, spent: 0 } });

      const group = screen.getByTestId('compose-spell-slots');
      await user.click(within(group).getAllByRole('button', { pressed: false })[0]);

      expect(store.getPendingDelta()).toEqual({ spellcasting: { slot1: { spent: 1 } } });
    });
  });
});
