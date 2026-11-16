import type { DiceBlock } from '../api/models/dice';
import type { DiceError } from '../dice/errors';
import type { DiceTarget, PendingDiceKeys, SheetEditStore } from './sheet_edit';
import { pendingDiceKeys } from './sheet_edit';

export type { DiceTarget };

export type DiceCheck = (
  target: DiceTarget,
  key: string,
  expression: string,
) => DiceError | undefined;

export type DiceChange = { readonly ok: true } | { readonly ok: false; readonly error: DiceError };

export interface DiceStoreOptions {
  readonly edit: Pick<SheetEditStore, 'getState' | 'subscribe' | 'setDice'>;
  readonly check?: DiceCheck;
}

export interface DiceStore {
  getBlock(): DiceBlock;
  getPendingKeys(): PendingDiceKeys;
  subscribe(listener: () => void): () => void;
  setRow(key: string, expression: string | null | undefined): DiceChange;
  setSection(key: string, expression: string | null | undefined): DiceChange;
}

const OK: DiceChange = { ok: true };

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
