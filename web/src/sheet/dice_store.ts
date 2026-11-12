/**
 * The Dice store (ADR 0005, ADR 0006): the face the Dice controls read and write, over the
 * sheet's edit store. A choice is checked against the local grammar and fit first and, when
 * accepted, becomes pending work in the edit store, like an edited value: it is sent with the
 * next save, reverted by a discard and backed up with the draft. Nothing here reaches the
 * network.
 */

import type { DiceBlock } from '../api/models/dice';
import type { DiceError } from '../dice/errors';
import type { DiceTarget, PendingDiceKeys, SheetEditStore } from './sheet_edit';
import { pendingDiceKeys } from './sheet_edit';

export type { DiceTarget };

/** Judges an expression before it is accepted; `undefined` accepts it. */
export type DiceCheck = (
  target: DiceTarget,
  key: string,
  expression: string,
) => DiceError | undefined;

/** `ok` means the change passed the local check and is now pending. */
export type DiceChange = { readonly ok: true } | { readonly ok: false; readonly error: DiceError };

export interface DiceStoreOptions {
  readonly edit: Pick<SheetEditStore, 'getState' | 'subscribe' | 'setDice'>;
  readonly check?: DiceCheck;
}

export interface DiceStore {
  /** The Dice block as the player has it now, pending choices included. */
  getBlock(): DiceBlock;
  /** The Row and Section keys whose choice is not saved yet. */
  getPendingKeys(): PendingDiceKeys;
  subscribe(listener: () => void): () => void;
  /** Sets a Row expression; `null` is "no dice", `undefined` removes the override (Template default). */
  setRow(key: string, expression: string | null | undefined): DiceChange;
  /** Sets a Section expression with the same meaning as {@link DiceStore.setRow}. */
  setSection(key: string, expression: string | null | undefined): DiceChange;
}

const OK: DiceChange = { ok: true };

/** Builds the Dice store over one sheet's edit store. */
export function createDiceStore(options: DiceStoreOptions): DiceStore {
  const check: DiceCheck = options.check ?? (() => undefined);
  const { edit } = options;
  let keysFor: DiceBlock | undefined;
  let keysAgainst: DiceBlock | undefined;
  let keys: PendingDiceKeys = { row: new Set(), section: new Set() };

  function apply(
    target: DiceTarget,
    key: string,
    expression: string | null | undefined,
  ): DiceChange {
    if (typeof expression === 'string') {
      const error = check(target, key, expression);
      if (error !== undefined) {
        return { ok: false, error };
      }
    }
    edit.setDice(target, key, expression);
    return OK;
  }

  return {
    getBlock: () => edit.getState().dice,
    getPendingKeys: () => {
      const state = edit.getState();
      if (keysFor !== state.dice || keysAgainst !== state.savedDice) {
        keysFor = state.dice;
        keysAgainst = state.savedDice;
        keys = pendingDiceKeys(state);
      }
      return keys;
    },
    subscribe: (listener) => edit.subscribe(listener),
    setRow: (key, expression) => apply('row', key, expression),
    setSection: (key, expression) => apply('section', key, expression),
  };
}
