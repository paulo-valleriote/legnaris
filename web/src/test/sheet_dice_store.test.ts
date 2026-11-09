import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse } from '../api/transport';
import type { DiceBlock } from '../api/models/dice';
import type { DiceError } from '../dice/errors';
import { createDiceStore, type DiceStore } from '../sheet/dice_store';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const baseUrl = 'https://api.example.test';
const SHEET = '01a0f967-f42b-710e-851f-8f293a634921';
const EMPTY: DiceBlock = { section: {}, row: {} };

function storeWith(
  transport: ReturnType<typeof createFakeTransport>['transport'],
  initial: DiceBlock = EMPTY,
  check: (target: string, key: string, expression: string) => DiceError | undefined = () =>
    undefined,
): DiceStore {
  const client = createApiClient({ baseUrl, transport });
  return createDiceStore({ client, sheetId: SHEET, initial, check });
}

/** A transport whose attempts wait for the test to settle each one in order. */
function gatedTransport() {
  const requests: HttpRequest[] = [];
  const settlers: Array<(outcome: HttpResponse | { failure: unknown }) => void> = [];
  const transport = (request: HttpRequest): Promise<HttpResponse> => {
    requests.push(request);
    return new Promise<HttpResponse>((resolve, reject) => {
      settlers.push((outcome) => {
        if ('failure' in outcome) {
          reject(outcome.failure);
          return;
        }
        resolve(outcome);
      });
    });
  };
  return {
    requests,
    settle(index: number, outcome: HttpResponse | { failure: unknown }) {
      settlers[index]?.(outcome);
    },
    transport,
  };
}

const flush = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

describe('dice store: saving a change', () => {
  it('Happy path: a row expression is sent at once as the full block and the stored block is kept', async () => {
    const fake = createFakeTransport(
      successOutcome({ dice: { section: {}, row: { 'attrs.init': '1d20' } } }),
    );
    const store = storeWith(fake.transport);

    const outcome = await store.setRow('attrs.init', '1d20');

    expect(outcome).toEqual({ ok: true });
    expect(fake.lastRequest?.method).toBe('PUT');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/sheets/${SHEET}/dice`);
    expect(fake.lastRequest?.body).toBe('{"dice":{"section":{},"row":{"attrs.init":"1d20"}}}');
    expect(store.getState().block.row).toEqual({ 'attrs.init': '1d20' });
    expect(store.getState().saving).toBe(false);
    expect(store.getState().errorKey).toBeUndefined();
  });

  it('Happy path: subscribers are told when the state changes', async () => {
    const fake = createFakeTransport(successOutcome({ dice: { section: {}, row: { a: '1d4' } } }));
    const store = storeWith(fake.transport);
    let notified = 0;
    store.subscribe(() => {
      notified += 1;
    });

    await store.setRow('a', '1d4');

    expect(notified).toBeGreaterThan(0);
  });

  it('Edge: setting the value already stored sends nothing and succeeds', async () => {
    const fake = createFakeTransport(successOutcome({ dice: EMPTY }));
    const store = storeWith(fake.transport, { section: {}, row: { a: '1d4' } });

    await expect(store.setRow('a', '1d4')).resolves.toEqual({ ok: true });
    expect(fake.attempts).toBe(0);
  });

  it('Edge: undefined removes the override so the Template default applies again', async () => {
    const fake = createFakeTransport(successOutcome({ dice: EMPTY }));
    const store = storeWith(fake.transport, { section: {}, row: { a: '1d4' } });

    await store.setRow('a', undefined);

    expect(fake.lastRequest?.body).toBe('{"dice":{"section":{},"row":{}}}');
    expect(store.getState().block.row).toEqual({});
  });

  it('Edge: null is an explicit "no dice" and is sent as null', async () => {
    const fake = createFakeTransport(
      successOutcome({ dice: { section: { attrs: null }, row: {} } }),
    );
    const store = storeWith(fake.transport);

    await store.setSection('attrs', null);

    expect(fake.lastRequest?.body).toBe('{"dice":{"section":{"attrs":null},"row":{}}}');
  });
});

describe('dice store: refusing before any request', () => {
  it('Edge: an expression the client check rejects is refused inline, no request is made, the value is unchanged', async () => {
    const fake = createFakeTransport(successOutcome({ dice: EMPTY }));
    const refusal: DiceError = { class: 'unreadable_term', message: 'bad' };
    const store = storeWith(fake.transport, EMPTY, () => refusal);

    const outcome = await store.setRow('attrs.init', '1d20 + 1.5');

    expect(outcome).toEqual({ ok: false, error: refusal });
    expect(fake.attempts).toBe(0);
    expect(store.getState().block).toEqual(EMPTY);
  });
});

describe('dice store: failures revert', () => {
  it('Error: a 400 from the server restores the previous block and says why, without an unsaved flag', async () => {
    const fake = createFakeTransport(errorOutcome('INVALID_REQUEST', 'bad', 400));
    const store = storeWith(fake.transport, { section: {}, row: { a: '1d4' } });

    const outcome = await store.setRow('a', '2d6');

    expect(outcome).toEqual({ ok: true });
    expect(store.getState().block.row).toEqual({ a: '1d4' });
    expect(store.getState().errorKey).toBe('dice.error.serverRefused');
    expect(store.getState().saving).toBe(false);
  });

  it('Error: a network failure restores the previous block and shows the save error', async () => {
    const fake = createFakeTransport({ failure: new Error('offline') });
    const store = storeWith(fake.transport, { section: {}, row: { a: '1d4' } });

    await store.setRow('a', '2d6');

    expect(store.getState().block.row).toEqual({ a: '1d4' });
    expect(store.getState().errorKey).toBe('dice.error.saveFailed');
  });

  it('Edge: a later successful save clears the error', async () => {
    const fake = createFakeTransport(
      errorOutcome('INVALID_REQUEST', 'bad', 400),
      successOutcome({ dice: { section: {}, row: { a: '2d6' } } }),
    );
    const store = storeWith(fake.transport);

    await store.setRow('a', '2d6');
    expect(store.getState().errorKey).toBe('dice.error.serverRefused');

    await store.setRow('a', '2d6');
    expect(store.getState().errorKey).toBeUndefined();
    expect(store.getState().block.row).toEqual({ a: '2d6' });
  });
});

describe('dice store: rapid edits', () => {
  it('Edge: a second toggle while the first is in flight is queued, and the last request carries the final value', async () => {
    const gate = gatedTransport();
    const store = storeWith(gate.transport);

    const first = store.setRow('a', '1d4');
    const second = store.setRow('a', '1d6');
    await flush();
    expect(gate.requests.length).toBe(1);

    gate.settle(0, successOutcome({ dice: { section: {}, row: { a: '1d6' } } }));
    await first;
    await flush();
    expect(gate.requests.length).toBe(2);
    expect(gate.requests[1]?.body).toBe('{"dice":{"section":{},"row":{"a":"1d6"}}}');

    gate.settle(1, successOutcome({ dice: { section: {}, row: { a: '1d6' } } }));
    await second;
    expect(store.getState().block.row).toEqual({ a: '1d6' });
    expect(store.getState().saving).toBe(false);
  });

  it('Edge: when an in-flight save fails, the queued edits are dropped and the confirmed block is restored', async () => {
    const gate = gatedTransport();
    const store = storeWith(gate.transport, { section: {}, row: { a: '1d4' } });

    const first = store.setRow('a', '1d6');
    const second = store.setRow('b', '1d8');
    await flush();
    gate.settle(0, { failure: new Error('offline') });
    await Promise.all([first, second]);
    await flush();

    expect(gate.requests.length).toBe(1);
    expect(store.getState().block).toEqual({ section: {}, row: { a: '1d4' } });
    expect(store.getState().errorKey).toBe('dice.error.saveFailed');
    expect(store.getState().saving).toBe(false);
  });
});
