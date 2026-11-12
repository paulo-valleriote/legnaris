import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from '../api/client';
import type { DiceBlock } from '../api/models/dice';
import { parseSheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import type { DiceError } from '../dice/errors';
import { message } from '../l10n/strings';
import { createMemorySheetStore } from '../library/sheet_store';
import { createDiceStore, type DiceCheck } from '../sheet/dice_store';
import {
  canPreview,
  canSave,
  createSheetEditStore,
  isDicePending,
  isPending,
  pendingCount,
} from '../sheet/sheet_edit';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const EMPTY: DiceBlock = { section: {}, row: {} };
const NO_SLEEP = { sleep: () => Promise.resolve(), random: () => 0 };

function setup(
  transport: Parameters<typeof createApiClient>[0]['transport'],
  dice: DiceBlock = EMPTY,
  check?: DiceCheck,
) {
  const client = createApiClient({ baseUrl: BASE_URL, transport });
  const sheetStore = createMemorySheetStore();
  const edit = createSheetEditStore(
    {
      template: parseTemplateDetail(templateDetailPayload()),
      sheet: parseSheet(sheetPayload({ dice })),
    },
    { client, store: sheetStore, saveRetry: NO_SLEEP },
  );
  const dicer = createDiceStore({ edit, ...(check === undefined ? {} : { check }) });
  return { edit, dice: dicer, sheetStore };
}

function bodyOf(request: { body?: string | undefined } | undefined): unknown {
  return JSON.parse(request?.body ?? 'null');
}

describe('dice store: a choice is pending work (ADR 0006)', () => {
  it('Happy path: a Row expression is applied at once, pends, and sends nothing', () => {
    const fake = createFakeTransport(successOutcome({}));
    const { edit, dice } = setup(fake.transport);

    const outcome = dice.setRow('identity.level', '1d20 + @');

    expect(outcome).toEqual({ ok: true });
    expect(dice.getBlock()).toEqual({ section: {}, row: { 'identity.level': '1d20 + @' } });
    expect(isPending(edit.getState())).toBe(true);
    expect(isDicePending(edit.getState())).toBe(true);
    expect(canSave(edit.getState())).toBe(true);
    expect(pendingCount(edit.getState())).toBe(1);
    expect(dice.getPendingKeys().row).toEqual(new Set(['identity.level']));
    expect(fake.attempts).toBe(0);
  });

  it('Happy path: subscribers are told when a choice changes', () => {
    const { dice } = setup(createFakeTransport(successOutcome({})).transport);
    const listener = vi.fn();
    dice.subscribe(listener);

    dice.setSection('identity', '1d20');

    expect(listener).toHaveBeenCalled();
  });

  it('Edge: setting the value already saved changes nothing and notifies nobody', () => {
    const { edit, dice } = setup(createFakeTransport(successOutcome({})).transport, {
      section: {},
      row: { 'identity.level': '1d20' },
    });
    const listener = vi.fn();
    dice.subscribe(listener);

    expect(dice.setRow('identity.level', '1d20')).toEqual({ ok: true });

    expect(listener).not.toHaveBeenCalled();
    expect(isPending(edit.getState())).toBe(false);
  });

  it('Edge: a choice set back to its saved value is no longer pending', () => {
    const { edit, dice } = setup(createFakeTransport(successOutcome({})).transport, {
      section: { identity: '1d20' },
      row: {},
    });

    dice.setSection('identity', '2d6');
    dice.setSection('identity', '1d20');

    expect(isPending(edit.getState())).toBe(false);
    expect(dice.getPendingKeys().section.size).toBe(0);
    expect(edit.getPendingDice()).toBeUndefined();
  });

  it('Edge: undefined removes the override, and a removal is pending until saved', () => {
    const { edit, dice } = setup(createFakeTransport(successOutcome({})).transport, {
      section: {},
      row: { 'identity.level': '1d20' },
    });

    dice.setRow('identity.level', undefined);

    expect(dice.getBlock().row).toEqual({});
    expect(dice.getPendingKeys().row).toEqual(new Set(['identity.level']));
    expect(isPending(edit.getState())).toBe(true);
  });

  it('Edge: null is an explicit "no dice", distinct from an absent override', () => {
    const { dice } = setup(createFakeTransport(successOutcome({})).transport);

    dice.setSection('ability', null);

    expect(dice.getBlock().section).toEqual({ ability: null });
    expect(dice.getPendingKeys().section).toEqual(new Set(['ability']));
  });

  it('Edge: an expression the local check rejects is refused and changes nothing', () => {
    const refusal: DiceError = { class: 'unreadable_term', message: 'bad' };
    const { edit, dice } = setup(
      createFakeTransport(successOutcome({})).transport,
      EMPTY,
      () => refusal,
    );

    expect(dice.setRow('identity.level', 'banana')).toEqual({ ok: false, error: refusal });
    expect(dice.getBlock()).toEqual(EMPTY);
    expect(isPending(edit.getState())).toBe(false);
  });

  it('Edge: the check never judges a null or a removal', () => {
    const check = vi.fn(() => undefined);
    const { dice } = setup(createFakeTransport(successOutcome({})).transport, EMPTY, check);

    dice.setRow('identity.level', null);
    dice.setRow('identity.level', undefined);

    expect(check).not.toHaveBeenCalled();
  });

  it('Edge: a Dice choice alone never enables a Preview, which recalculates values only', () => {
    const { edit, dice } = setup(createFakeTransport(successOutcome({})).transport);

    dice.setRow('identity.level', '1d20');

    expect(canPreview(edit.getState())).toBe(false);
  });

  it('Edge: a Dice choice keeps the preview marks of an earlier Preview', async () => {
    const fake = createFakeTransport(successOutcome({ derived: { total: 5 } }));
    const { edit, dice } = setup(fake.transport);
    edit.setPath('identity.level', 4);
    await edit.preview();
    const marks = edit.getState().previewMarks;

    dice.setRow('identity.level', '1d20');

    expect(marks).toBeDefined();
    expect(edit.getState().previewMarks).toBe(marks);
  });

  it('Edge: the pending keys snapshot is stable between changes, so a subscriber does not loop', () => {
    const { dice } = setup(createFakeTransport(successOutcome({})).transport);
    dice.setRow('identity.level', '1d20');

    expect(dice.getPendingKeys()).toBe(dice.getPendingKeys());
  });
});

describe('dice store: saved with the sheet (ADR 0006)', () => {
  it('Happy path: a dice-only save sends one PATCH carrying the whole block and no data', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const { edit, dice } = setup(fake.transport, { section: { ability: null }, row: {} });
    dice.setRow('identity.level', '1d20 + @');

    await edit.save();

    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('PATCH');
    expect(bodyOf(fake.lastRequest)).toEqual({
      dice: { section: { ability: null }, row: { 'identity.level': '1d20 + @' } },
    });
    expect(isPending(edit.getState())).toBe(false);
    expect(edit.getState().savedDice).toEqual({
      section: { ability: null },
      row: { 'identity.level': '1d20 + @' },
    });
  });

  it('Happy path: values and dice go out in the same PATCH', async () => {
    const fake = createFakeTransport(successOutcome({ derived: { total: 5 } }));
    const { edit, dice } = setup(fake.transport);
    edit.setPath('identity.level', 4);
    dice.setSection('identity', '1d20');

    await edit.save();

    expect(fake.attempts).toBe(1);
    expect(bodyOf(fake.lastRequest)).toEqual({
      data: { identity: { level: 4 } },
      dice: { section: { identity: '1d20' }, row: {} },
    });
    expect(edit.getState().data).toMatchObject({ derived: { total: 5 } });
    expect(isPending(edit.getState())).toBe(false);
  });

  it('Edge: a value-only save carries no dice key', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const { edit } = setup(fake.transport, { section: { identity: '1d20' }, row: {} });
    edit.setPath('identity.level', 4);

    await edit.save();

    expect(bodyOf(fake.lastRequest)).toEqual({ data: { identity: { level: 4 } } });
  });

  it('Edge: clearing every override sends an empty block, which the server stores as none', async () => {
    const fake = createFakeTransport(successOutcome({}));
    const { edit, dice } = setup(fake.transport, { section: {}, row: { 'identity.level': '1d4' } });
    dice.setRow('identity.level', undefined);

    await edit.save();

    expect(bodyOf(fake.lastRequest)).toEqual({ dice: { section: {}, row: {} } });
  });

  it('Error: a refused save keeps the choice pending and shows the mapped copy, with no rollback', async () => {
    const fake = createFakeTransport(errorOutcome('INVALID_REQUEST', 'invalid dice', 400));
    const { edit, dice } = setup(fake.transport);
    dice.setRow('identity.level', '1d20');

    await edit.save();

    expect(edit.getState().saveError).toBe(message('api.error.invalidEdit'));
    expect(dice.getBlock().row).toEqual({ 'identity.level': '1d20' });
    expect(isPending(edit.getState())).toBe(true);
    expect(fake.attempts).toBe(1);
  });

  it('Error: a persistent 409 stops after three sends and keeps the choice pending', async () => {
    const fake = createFakeTransport(errorOutcome('CONFLICT', 'conflict', 409));
    const { edit, dice } = setup(fake.transport);
    dice.setRow('identity.level', '1d20');

    await edit.save();

    expect(fake.attempts).toBe(3);
    expect(edit.getState().saveError).toBe(message('sheet.error.saveConflict'));
    expect(isDicePending(edit.getState())).toBe(true);
  });

  it('Edge: a choice made while a save is in flight stays pending after it succeeds', async () => {
    const deferred = createDeferredTransport();
    const { edit, dice } = setup(deferred.transport);
    dice.setRow('identity.level', '1d20');

    const saving = edit.save();
    dice.setRow('identity.level', '2d6');
    deferred.resolve(successOutcome({}));
    await saving;

    expect(edit.getState().savedDice.row).toEqual({ 'identity.level': '1d20' });
    expect(dice.getPendingKeys().row).toEqual(new Set(['identity.level']));
    expect(isPending(edit.getState())).toBe(true);
  });

  it('Edge: the pending keys follow the new saved block after a save', async () => {
    const { edit, dice } = setup(createFakeTransport(successOutcome({})).transport);
    dice.setRow('identity.level', '1d20');
    expect(dice.getPendingKeys().row.size).toBe(1);

    await edit.save();

    expect(dice.getPendingKeys().row.size).toBe(0);
  });

  it('Happy path: discard returns the block to the saved one', () => {
    const { edit, dice } = setup(createFakeTransport(successOutcome({})).transport, {
      section: {},
      row: { 'identity.level': '1d4' },
    });
    dice.setRow('identity.level', '1d20');
    dice.setSection('ability', null);

    edit.discard();

    expect(dice.getBlock()).toEqual({ section: {}, row: { 'identity.level': '1d4' } });
    expect(isPending(edit.getState())).toBe(false);
  });
});

describe('dice store: the draft (ADR 0006)', () => {
  it('Happy path: restore brings back a pending block from the draft', async () => {
    const { edit, dice, sheetStore } = setup(createFakeTransport(successOutcome({})).transport);
    await sheetStore.writeDraft(SHEET_ID, {}, { section: { identity: '1d20' }, row: {} });

    await edit.restore();

    expect(dice.getBlock()).toEqual({ section: { identity: '1d20' }, row: {} });
    expect(isPending(edit.getState())).toBe(true);
  });

  it('Edge: restore drops draft keys the Template no longer has', async () => {
    const { edit, dice, sheetStore } = setup(createFakeTransport(successOutcome({})).transport);
    await sheetStore.writeDraft(
      SHEET_ID,
      {},
      {
        section: { identity: '1d20', retired: '1d4' },
        row: { 'identity.level': null, 'identity.gone': '1d6', 'ability#ability': '1d8' },
      },
    );

    await edit.restore();

    expect(dice.getBlock()).toEqual({
      section: { identity: '1d20' },
      row: { 'identity.level': null, 'ability#ability': '1d8' },
    });
  });

  it('Edge: a draft written before dice existed restores its values and leaves the block alone', async () => {
    const saved: DiceBlock = { section: { identity: '1d20' }, row: {} };
    const { edit, dice, sheetStore } = setup(
      createFakeTransport(successOutcome({})).transport,
      saved,
    );
    sheetStore.putRawDraft(SHEET_ID, {
      schemaVersion: 3,
      id: SHEET_ID,
      delta: { identity: { level: 9 } },
    });

    await edit.restore();

    expect(dice.getBlock()).toEqual(saved);
    expect(edit.getPendingDelta()).toEqual({ identity: { level: 9 } });
  });

  it('Edge: an offline save keeps a dice-only change on the device and says so', async () => {
    const fake = createFakeTransport({ failure: new TypeError('Failed to fetch') });
    const { edit, dice, sheetStore } = setup(fake.transport);
    dice.setRow('identity.level', '1d20');

    await edit.save();

    expect(edit.getState().saveError).toBe(message('sheet.error.saveOffline'));
    expect(await sheetStore.readDraft(SHEET_ID)).toEqual({
      delta: {},
      dice: { section: {}, row: { 'identity.level': '1d20' } },
    });
  });

  it('Happy path: a successful save deletes the draft', async () => {
    const { edit, dice, sheetStore } = setup(createFakeTransport(successOutcome({})).transport);
    dice.setRow('identity.level', '1d20');
    await sheetStore.writeDraft(SHEET_ID, {}, edit.getPendingDice());

    await edit.save();

    expect(await sheetStore.readDraft(SHEET_ID)).toBeNull();
  });
});
