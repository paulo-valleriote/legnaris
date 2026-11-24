import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { parseSheet } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { parseTemplateDetail } from '../api/models/template';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import type { PresetConfig } from '../sheet/preset/compose/types';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import { fifthStatBlockConfig } from '../sheet/preset/compose/configs/fifth_stat_block';
import type { PresetResetBinding } from '../sheet/preset/preset_context';
import { PresetPlayView } from '../sheet/preset/play_view';
import { renderPreset } from './preset_fixtures';
import goldenCharacter from '../../../testdata/templates/fifth_character_v1.json';
import goldenStatBlock from '../../../testdata/templates/fifth_stat_block_v1.json';

const COMPOSE_CSS = readFileSync(
  path.resolve(process.cwd(), 'src/sheet/preset/compose/compose.css'),
  'utf8',
);

const ABILITIES = [
  'strength',
  'dexterity',
  'constitution',
  'intelligence',
  'wisdom',
  'charisma',
] as const;

const SKILLS = [
  'acrobatics',
  'animalHandling',
  'arcana',
  'athletics',
  'deception',
  'history',
  'insight',
  'intimidation',
  'investigation',
  'medicine',
  'nature',
  'perception',
  'performance',
  'persuasion',
  'religion',
  'sleightOfHand',
  'stealth',
  'survival',
] as const;

const CONDITIONS = [
  'blinded',
  'charmed',
  'deafened',
  'frightened',
  'grappled',
  'incapacitated',
  'invisible',
  'paralyzed',
  'petrified',
  'poisoned',
  'prone',
  'restrained',
  'stunned',
  'unconscious',
] as const;

function abilityData() {
  return Object.fromEntries(ABILITIES.map((ability) => [ability, { score: 14, mod: 2 }]));
}

function saveData() {
  return Object.fromEntries(ABILITIES.map((ability) => [ability, { prof: 0, misc: 0, total: 2 }]));
}

function skillData() {
  return Object.fromEntries(SKILLS.map((skill) => [skill, { rank: 0, misc: 0, total: 2 }]));
}

function characterTemplate(): TemplateDetail {
  return parseTemplateDetail(goldenCharacter.data);
}

function statBlockTemplate(): TemplateDetail {
  return parseTemplateDetail(goldenStatBlock.data);
}

function characterSheet() {
  return parseSheet({
    id: '01a14236-21c9-7546-945e-40c46d1fcd80',
    template_id: 'fifth_character_v1',
    kind: 'character',
    name: 'Brenna',
    data: {
      identity: { name: 'Brenna', class: 'Ranger' },
      combat: { hp: { current: 20, max: 30, temp: 0 } },
      abilities: abilityData(),
      saves: saveData(),
      skills: skillData(),
      status: {
        conditions: Object.fromEntries(CONDITIONS.map((condition) => [condition, 0])),
        deathSaves: { successes: 0, failures: 0 },
      },
      exhaustion: { level: 0, penalty: 0 },
      hitDice: { primary: { spent: 0, count: 3 } },
    },
    created_at: '2026-10-01T21:38:54.635Z',
    updated_at: '2026-10-02T09:00:00.000Z',
  });
}

function statBlockSheet() {
  return parseSheet({
    id: '01a14236-21c9-7546-945e-40c46d1fcd81',
    template_id: 'fifth_stat_block_v1',
    kind: 'npc',
    name: 'Owlbear',
    data: {
      identity: { name: 'Owlbear', challengeRating: 3 },
      hitPoints: { current: 40, maximum: 59, temp: 0 },
      abilities: abilityData(),
      saves: saveData(),
      skills: skillData(),
      exhaustion: { level: 0, penalty: 0 },
      details: {
        resistances: 'None',
        immunities: 'None',
        senses: 'Darkvision 60 ft.',
        languages: 'None',
      },
    },
    created_at: '2026-10-01T21:38:54.635Z',
    updated_at: '2026-10-02T09:00:00.000Z',
  });
}

function stubReset(): PresetResetBinding {
  return {
    run: () => Promise.resolve({}),
    onApplied: () => undefined,
    refresh: () => Promise.resolve(),
  };
}

function interactiveElements(root: HTMLElement): HTMLElement[] {
  return Array.from(
    root.querySelectorAll<HTMLElement>(
      'button, input, select, textarea, a[href], [tabindex]:not([tabindex="-1"])',
    ),
  ).filter(
    (element) =>
      !element.hasAttribute('disabled') &&
      element.closest('[aria-hidden="true"]') === null &&
      element.closest('[inert]') === null,
  );
}

function renderConfig(
  config: PresetConfig,
  template: TemplateDetail,
  sheet: ReturnType<typeof characterSheet> | ReturnType<typeof statBlockSheet>,
) {
  const preset = presetFromConfig(config);
  return renderPreset(
    <>
      <preset.Header />
      <PresetPlayView preset={preset} />
    </>,
    { template, sheet, reset: stubReset() },
  );
}

async function tabThrough(root: HTMLElement): Promise<HTMLElement[]> {
  const user = userEvent.setup();
  const expected = interactiveElements(root);
  const reached: HTMLElement[] = [];
  for (let index = 0; index < expected.length + 4; index += 1) {
    await user.tab();
    const active = document.activeElement;
    if (active instanceof HTMLElement && root.contains(active)) {
      reached.push(active);
    }
  }
  return reached;
}

function press(element: HTMLElement, key: 'Enter' | ' ') {
  const user = userEvent.setup();
  element.focus();
  return user.keyboard(key === ' ' ? ' ' : `{${key}}`);
}

const configs = [
  {
    name: 'the fifth-edition character config',
    config: fifthCharacterConfig,
    template: characterTemplate,
    sheet: characterSheet,
    hasConditionChip: true,
  },
  {
    name: 'the fifth-edition stat block config',
    config: fifthStatBlockConfig,
    template: statBlockTemplate,
    sheet: statBlockSheet,
    hasConditionChip: false,
  },
] as const;

describe.each(configs)('$name accessibility', ({ config, template, sheet, hasConditionChip }) => {
  it('Happy path: every interactive element in Play has an accessible name', () => {
    const { container } = renderConfig(config, template(), sheet());

    const interactive = interactiveElements(container);
    expect(interactive.length).toBeGreaterThan(0);
    for (const element of interactive) {
      expect(element).toHaveAccessibleName();
    }
  });

  it('Happy path: every interactive element in Play is reachable by Tab', async () => {
    const { container } = renderConfig(config, template(), sheet());

    const expected = interactiveElements(container);
    const reached = await tabThrough(container);

    expect(new Set(reached)).toEqual(new Set(expected));
  });

  it('Happy path: a pip toggles with Enter and Space', async () => {
    const { container } = renderConfig(config, template(), sheet());
    const pip = container.querySelector<HTMLButtonElement>('button.lg-compose-pip');

    expect(pip).not.toBeNull();
    expect(pip).toHaveAttribute('aria-pressed', 'false');
    await press(pip as HTMLElement, 'Enter');
    expect(pip).toHaveAttribute('aria-pressed', 'true');
    await press(pip as HTMLElement, ' ');
    expect(pip).toHaveAttribute('aria-pressed', 'false');
  });

  it('Happy path: a save proficiency toggle works with Enter and Space', async () => {
    const { container } = renderConfig(config, template(), sheet());
    const flag = container.querySelector<HTMLButtonElement>('button.lg-compose-flag');

    expect(flag).not.toBeNull();
    expect(flag).toHaveAttribute('aria-pressed', 'false');
    await press(flag as HTMLElement, 'Enter');
    expect(flag).toHaveAttribute('aria-pressed', 'true');
    await press(flag as HTMLElement, ' ');
    expect(flag).toHaveAttribute('aria-pressed', 'false');
  });

  it('Happy path: a skill rank cycles with Enter and Space', async () => {
    const { container } = renderConfig(config, template(), sheet());
    const rank = container.querySelector<HTMLButtonElement>('button.lg-compose-rank');

    expect(rank).not.toBeNull();
    const before = rank?.getAttribute('aria-label');
    await press(rank as HTMLElement, 'Enter');
    expect(rank?.getAttribute('aria-label')).not.toBe(before);
    const afterEnter = rank?.getAttribute('aria-label');
    await press(rank as HTMLElement, ' ');
    expect(rank?.getAttribute('aria-label')).not.toBe(afterEnter);
  });

  it.runIf(hasConditionChip)(
    'Happy path: a condition chip toggles with Enter and Space',
    async () => {
      const { container } = renderConfig(config, template(), sheet());
      const chip = container.querySelector<HTMLButtonElement>('button.lg-compose-condition');

      expect(chip).not.toBeNull();
      expect(chip).toHaveAttribute('aria-pressed', 'false');
      await press(chip as HTMLElement, ' ');
      expect(chip).toHaveAttribute('aria-pressed', 'true');
      await press(chip as HTMLElement, 'Enter');
      expect(chip).toHaveAttribute('aria-pressed', 'false');
    },
  );
});

function reducedMotionBlock(): string {
  const match = /@media \(prefers-reduced-motion: reduce\) \{([\s\S]*?)\n\}/.exec(COMPOSE_CSS);
  expect(match).not.toBeNull();
  return match?.[1] ?? '';
}

function transitionSelectors(css: string): string[] {
  const selectors = new Set<string>();
  for (const rule of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selector = rule[1].trim();
    const body = rule[2];
    if (selector.startsWith('@') || !/transition\s*:/.test(body)) {
      continue;
    }
    if (/transition\s*:\s*none/.test(body)) {
      continue;
    }
    for (const part of selector.split(',')) {
      selectors.add(part.trim());
    }
  }
  return [...selectors];
}

describe('the composable pieces under prefers-reduced-motion', () => {
  it('Happy path: every animated selector is turned off inside the reduced-motion block', () => {
    const reduced = reducedMotionBlock();
    const selectors = transitionSelectors(COMPOSE_CSS);

    expect(selectors.length).toBeGreaterThan(0);
    for (const selector of selectors) {
      expect(reduced).toContain(selector);
    }
  });

  it('Happy path: no piece declares a keyframe animation', () => {
    expect(COMPOSE_CSS).not.toMatch(/@keyframes\s/);
    expect(COMPOSE_CSS).not.toMatch(/animation\s*:/);
  });

  it('Happy path: both configs render every piece with reduced motion requested', () => {
    const original = window.matchMedia;
    window.matchMedia = ((media: string) => {
      const query = original(media);
      if (media.includes('prefers-reduced-motion')) {
        return { ...query, matches: true } as MediaQueryList;
      }
      return query;
    }) as typeof window.matchMedia;

    try {
      for (const entry of configs) {
        const { container, unmount } = renderConfig(entry.config, entry.template(), entry.sheet());
        expect(within(container).getByTestId('sheet-play')).toBeInTheDocument();
        expect(document.getAnimations?.() ?? []).toHaveLength(0);
        unmount();
      }
    } finally {
      window.matchMedia = original;
    }

    expect(screen.queryByTestId('sheet-play')).not.toBeInTheDocument();
  });
});
