/**
 * The Dice edit store (ADR 0005): a row or Section expression is saved the
 * moment it is set, through the full-block `PUT /sheets/:id/dice`, and never
 * counts as unsaved work. A change shows at once; when the save fails the
 * last confirmed block comes back and a message says why.
 *
 * Saves run one at a time, each sending the block as it stands when its turn
 * comes, so rapid edits never race each other. A failure drops the edits queued
 * behind it: they were built on a block the server refused, so the confirmed
 * block is the only honest state to return to.
 */

import type { ApiClient } from '../api/client';
import { DomainFailure } from '../api/errors';
import type { DiceBlock, DiceOverrides } from '../api/models/dice';
import type { DiceError } from '../dice/errors';
import type { MessageKey } from '../l10n/strings';

export type DiceTarget = 'row' | 'section';

/** Judges an expression before it is saved; `undefined` accepts it. */
export type DiceCheck = (
  target: DiceTarget,
  key: string,
  expression: string,
) => DiceError | undefined;

/** `ok` means the change passed the local check and was applied; the save itself is reported in the state. */
export type DiceChange = { readonly ok: true } | { readonly ok: false; readonly error: DiceError };

export interface DiceStoreState {
  readonly block: DiceBlock;
  readonly saving: boolean;
  /** The copy key of the last failed save, cleared by the next successful one. */
  readonly errorKey: MessageKey | undefined;
}

export interface DiceStoreOptions {
  readonly client: Pick<ApiClient, 'setDice'>;
  readonly sheetId: string;
  readonly initial: DiceBlock;
  readonly check?: DiceCheck;
}

export interface DiceStore {
  getState(): DiceStoreState;
  subscribe(listener: () => void): () => void;
  /** Sets a Row expression; `null` is "no dice", `undefined` removes the override (Template default). */
  setRow(key: string, expression: string | null | undefined): Promise<DiceChange>;
  /** Sets a Section expression with the same meaning as {@link DiceStore.setRow}. */
  setSection(key: string, expression: string | null | undefined): Promise<DiceChange>;
}

const OK: DiceChange = { ok: true };

function failureKey(error: unknown): MessageKey {
  if (error instanceof DomainFailure && error.kind === 'invalidEdit') {
    return 'dice.error.serverRefused';
  }
  return 'dice.error.saveFailed';
}

function withEntry(
  overrides: DiceOverrides,
  key: string,
  expression: string | null | undefined,
): DiceOverrides {
  const next: Record<string, string | null> = { ...overrides };
  if (expression === undefined) {
    delete next[key];
  } else {
    next[key] = expression;
  }
  return next;
}

function isUnchanged(
  overrides: DiceOverrides,
  key: string,
  expression: string | null | undefined,
): boolean {
  if (expression === undefined) {
    return !Object.hasOwn(overrides, key);
  }
  return Object.hasOwn(overrides, key) && overrides[key] === expression;
}

/** Builds the Dice store over one sheet's block; the client is the only way it reaches the network. */
export function createDiceStore(options: DiceStoreOptions): DiceStore {
  const check: DiceCheck = options.check ?? (() => undefined);
  const listeners = new Set<() => void>();
  let confirmed = options.initial;
  let block = options.initial;
  let queued = 0;
  let generation = 0;
  let errorKey: MessageKey | undefined;
  let queue: Promise<void> = Promise.resolve();
  let snapshot: DiceStoreState = { block, saving: false, errorKey };

  function publish(): void {
    snapshot = { block, saving: queued > 0, errorKey };
    listeners.forEach((listener) => listener());
  }

  function succeed(stored: DiceBlock): void {
    confirmed = stored;
    errorKey = undefined;
    queued -= 1;
    if (queued === 0) {
      block = stored;
    }
    publish();
  }

  function fail(error: unknown): void {
    generation += 1;
    queued = 0;
    block = confirmed;
    errorKey = failureKey(error);
    publish();
  }

  function enqueue(): Promise<DiceChange> {
    const job = generation;
    queued += 1;
    publish();
    queue = queue.then(() => {
      if (job !== generation) {
        return undefined;
      }
      return options.client.setDice(options.sheetId, block).then(succeed, fail);
    });
    return queue.then(() => OK);
  }

  function apply(
    target: DiceTarget,
    key: string,
    expression: string | null | undefined,
  ): Promise<DiceChange> {
    const current = target === 'row' ? block.row : block.section;
    if (isUnchanged(current, key, expression)) {
      return Promise.resolve(OK);
    }
    if (typeof expression === 'string') {
      const error = check(target, key, expression);
      if (error !== undefined) {
        return Promise.resolve({ ok: false, error });
      }
    }
    block =
      target === 'row'
        ? { ...block, row: withEntry(block.row, key, expression) }
        : { ...block, section: withEntry(block.section, key, expression) };
    return enqueue();
  }

  return {
    getState: () => snapshot,
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    setRow: (key, expression) => apply('row', key, expression),
    setSection: (key, expression) => apply('section', key, expression),
  };
}
