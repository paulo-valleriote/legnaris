import { startEditing } from './start_editing';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { FLOATING_FLOOR_ATTRIBUTE } from '../sheet/dice_panel';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheetData,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import type { FakeTransport } from './fake_transport';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

vi.setConfig({ testTimeout: 20_000 });

const BASE_URL = 'http://api.test';
const BAR_LIFT = 120;
const here = path.dirname(fileURLToPath(import.meta.url));
const diceCss = readFileSync(path.resolve(here, '../sheet/dice.css'), 'utf8');

function sheetData(): Record<string, unknown> {
  const base = fifthCharacterSheetData();
  const combat = base.combat as Record<string, unknown>;
  return {
    ...base,
    combat: { ...combat, initiative: { alert: 0, misc: 0, total: 3 } },
  };
}

async function openPreset(): Promise<{
  readonly fake: FakeTransport;
  readonly user: ReturnType<typeof userEvent.setup>;
}> {
  const fake = createFakeTransport(
    successOutcome(fifthCharacterSheetPayload({ data: sheetData() })),
    successOutcome(fifthCharacterTemplatePayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  await startEditing();
  return { fake, user };
}

function panel(): HTMLElement {
  const element = document.querySelector<HTMLElement>('.lg-dice-panel');
  if (element === null) {
    throw new Error('the roll result panel is not drawn');
  }
  return element;
}

function card(): HTMLElement | null {
  return screen.queryByTestId('sheet-pending-card');
}

function levelInput(): HTMLElement {
  return within(
    document.querySelector<HTMLElement>('[data-field-path="identity.level"]') as HTMLElement,
  ).getByRole('textbox');
}

async function raiseLevel(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  const input = levelInput();
  await user.clear(input);
  await user.type(input, '6');
}

async function roll(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(screen.getByRole('button', { name: /^Roll Initiative/ }));
  await waitFor(() => expect(document.querySelector('.lg-dice-panel')).not.toBeNull());
}

function stubBarRect(): () => void {
  const original = Element.prototype.getBoundingClientRect;
  Element.prototype.getBoundingClientRect = function getBoundingClientRect(): DOMRect {
    if (this instanceof HTMLElement && this.dataset.testid === 'sheet-pending-card') {
      return { top: window.innerHeight - BAR_LIFT } as DOMRect;
    }
    return original.call(this);
  };
  return () => {
    Element.prototype.getBoundingClientRect = original;
  };
}

afterEach(cleanup);

describe('the roll result above the preset pending bar', () => {
  it('Edge: with no pending bar the result keeps its viewport-edge position', async () => {
    const { user } = await openPreset();

    await roll(user);

    expect(card()).toBeNull();
    expect(panel().style.getPropertyValue('--dice-floor')).toBe('0px');
  });

  it('Edge: a bar appearing after a result is shown lifts it without a scroll or resize', async () => {
    const { user } = await openPreset();
    await roll(user);
    expect(panel().style.getPropertyValue('--dice-floor')).toBe('0px');
    const restore = stubBarRect();
    try {
      await raiseLevel(user);

      await waitFor(() =>
        expect(panel().style.getPropertyValue('--dice-floor')).toBe(`${BAR_LIFT}px`),
      );
    } finally {
      restore();
    }
  });

  it('Edge: saving while a result is shown returns it to the viewport edge', async () => {
    const { fake, user } = await openPreset();
    const restore = stubBarRect();
    try {
      await raiseLevel(user);
      await roll(user);
      expect(panel().style.getPropertyValue('--dice-floor')).toBe(`${BAR_LIFT}px`);

      await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));

      await waitFor(() => expect(card()).toBeNull());
      await waitFor(() => expect(panel().style.getPropertyValue('--dice-floor')).toBe('0px'));
      expect(fake.requests.some((request) => request.method === 'PATCH')).toBe(true);
    } finally {
      restore();
    }
  });

  it('Happy path: a pending bar before the roll keeps the total, parts and one status fully above it', async () => {
    const { user } = await openPreset();
    const restore = stubBarRect();
    try {
      await raiseLevel(user);
      expect(card()).not.toBeNull();
      expect(card()?.hasAttribute(FLOATING_FLOOR_ATTRIBUTE)).toBe(true);

      await roll(user);

      await waitFor(() =>
        expect(panel().style.getPropertyValue('--dice-floor')).toBe(`${BAR_LIFT}px`),
      );
      expect(document.querySelector('.lg-dice-panel__total-label')).toHaveTextContent('Initiative');
      expect(document.querySelector('.lg-dice-panel__parts')).toHaveTextContent('1d20');
      expect(within(panel()).getAllByRole('status')).toHaveLength(1);
    } finally {
      restore();
    }
  });
});

describe('the roll result motion', () => {
  it('Edge: the panel rise-in is switched off under reduced motion', () => {
    const reduced = diceCss.slice(diceCss.indexOf('@media (prefers-reduced-motion: reduce)'));
    expect(diceCss).toMatch(/\.lg-dice-panel\s*\{[^}]*animation:\s*lg-dice-panel-in/);
    expect(reduced).toMatch(/\.lg-dice-panel\s*\{[^}]*animation:\s*none/);
  });
});
