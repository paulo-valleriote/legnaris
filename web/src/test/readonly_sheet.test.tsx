import { afterEach, describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { CAMPAIGN_ID, baseRoutes, clientFor, ok, scripted } from './campaign_helpers';
import type { Route } from './campaign_helpers';
import {
  FIFTH_CHARACTER_SHEET_ID,
  FIFTH_STAT_BLOCK_SHEET_ID,
  fifthCharacterSheetData,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
  fifthStatBlockSheetPayload,
  fifthStatBlockTemplatePayload,
} from './fifth_fixtures';
import { setViewportWidth } from './match_media';
import { renderApp } from './render';

const GM = `/campaigns/${CAMPAIGN_ID}/sheets/${FIFTH_CHARACTER_SHEET_ID}`;
const NPC = `/campaigns/${CAMPAIGN_ID}/npcs/${FIFTH_STAT_BLOCK_SHEET_ID}`;

function routes(
  sheetPath: string,
  templateId: string,
  sheet: Record<string, unknown>,
  template: Record<string, unknown>,
): Route[] {
  return [
    ...baseRoutes(),
    { method: 'GET', path: `/templates/${templateId}`, respond: () => ok(template) },
    { method: 'GET', path: sheetPath, respond: () => ok(sheet) },
  ];
}

function expectReadOnly(): void {
  expect(document.querySelector('[data-rollable="true"]')).toBeNull();
  expect(
    document.querySelectorAll(
      'input:not([readonly]):not([disabled]), textarea:not([readonly]):not([disabled])',
    ),
  ).toHaveLength(0);
  expect(document.querySelectorAll('select:not([disabled])')).toHaveLength(0);
  expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
  for (const name of [
    /edit sheet/i,
    /save/i,
    /discard/i,
    /preview/i,
    /damage/i,
    /heal/i,
    /level up/i,
    /^roll/i,
    /add to/i,
    /remove/i,
    /undo/i,
    /reset/i,
    /spend/i,
    /rest/i,
  ]) {
    expect(screen.queryByRole('button', { name })).toBeNull();
  }
}

afterEach(() => {
  setViewportWidth(1280);
});

describe('read-only sheet layout for another user', () => {
  it('Happy path: the GM sees a player 5E sheet with the preset layout and no edit or roll control', async () => {
    const script = scripted(
      routes(
        GM,
        'fifth_character_v1',
        fifthCharacterSheetPayload(),
        fifthCharacterTemplatePayload(),
      ),
    );
    renderApp(GM, { client: clientFor(script.transport) });

    expect(await screen.findByTestId('sheet-play')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Brenna', level: 1 })).toBeInTheDocument();
    expect(screen.getByTestId('compose-hit-points')).toBeInTheDocument();
    expectReadOnly();
  });

  it('Happy path: the phone GM view keeps the preset layout and no edit or roll control', async () => {
    setViewportWidth(390);
    const script = scripted(
      routes(
        GM,
        'fifth_character_v1',
        fifthCharacterSheetPayload(),
        fifthCharacterTemplatePayload(),
      ),
    );
    renderApp(GM, { client: clientFor(script.transport) });

    expect(await screen.findByTestId('sheet-play')).toBeInTheDocument();
    expect(document.querySelector('[data-preset-section="hitPoints"]')).not.toBeNull();
    expectReadOnly();
  });

  it('Happy path: active saves, conditions and pips keep their visual state without aria on a bare span', async () => {
    const base = fifthCharacterSheetData();
    const status = base['status'] as Record<string, unknown>;
    const data = {
      ...base,
      saves: {
        ...(base['saves'] as Record<string, unknown>),
        strength: { prof: 1, misc: 0, total: 3 },
      },
      status: {
        ...status,
        conditions: { ...(status['conditions'] as Record<string, unknown>), poisoned: 1 },
        deathSaves: { successes: 2, failures: 0 },
      },
    };
    const script = scripted(
      routes(
        GM,
        'fifth_character_v1',
        fifthCharacterSheetPayload({ data }),
        fifthCharacterTemplatePayload(),
      ),
    );
    renderApp(GM, { client: clientFor(script.transport) });

    expect(await screen.findByTestId('sheet-play')).toBeInTheDocument();
    expect(document.querySelectorAll('.lg-compose-condition[data-on]')).toHaveLength(1);
    expect(document.querySelectorAll('.lg-compose-flag[data-on]').length).toBeGreaterThan(0);
    expect(document.querySelectorAll('.lg-compose-pip[data-on]').length).toBeGreaterThanOrEqual(2);
    const labelled = Array.from(document.querySelectorAll('span[aria-label]'));
    const bare = labelled.find((span) => span.getAttribute('role') === null);
    expect(bare?.outerHTML).toBeUndefined();
    expectReadOnly();
  });

  it('Happy path: the NPC route renders a 5E stat block with the preset layout and no edit or roll control', async () => {
    const script = scripted(
      routes(
        NPC,
        'fifth_stat_block_v1',
        fifthStatBlockSheetPayload({ kind: 'npc' }),
        fifthStatBlockTemplatePayload(),
      ),
    );
    renderApp(NPC, { client: clientFor(script.transport) });

    expect(await screen.findByTestId('sheet-play')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Gray Ooze', level: 1 })).toBeInTheDocument();
    expectReadOnly();
  });
});
