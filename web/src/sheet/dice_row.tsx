/**
 * The Dice controls of a Row (ADR 0005, LGN-166): a Roll button and an Edit
 * button drawn only on a Row that has a rollable expression, one Roll panel and
 * one inline editor at a time, and the context that hands both the Dice store,
 * the schema and the values to the frame. Arrange mode replaces the Row frame
 * with its own, so these controls are never drawn while arranging.
 */

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react';
import type { DiceBlock } from '../api/models/dice';
import { performRoll, type RollResult } from '../dice/roll';
import type { DiceResult } from '../dice/errors';
import type { DiceSchema } from '../dice/fit';
import type { Resolution, DiceDefaults } from '../dice/resolve';
import { message, messageWith } from '../l10n/strings';
import type { SheetFormFrames, RowFrameProps } from '../engine/sheet_form';
import type { DiceStore } from './dice_store';
import { checkExpression, diceErrorMessageKey, resolveRowForSheet } from './dice_view';
import { DicePanel } from './dice_panel';
import './dice.css';

/** What the Dice controls of one sheet read and write. */
export interface DiceContextValue {
  readonly store: DiceStore;
  readonly schema: DiceSchema;
  readonly defaults: DiceDefaults;
  readonly rowKeys: ReadonlySet<string>;
  readonly sectionKeys: ReadonlySet<string>;
  /** The saved values a Roll reads when nothing is unsaved. */
  readonly savedValues: () => Record<string, unknown>;
  /** The unsaved edits, which win over the saved values in a Roll. */
  readonly draftValues: () => Record<string, unknown>;
}

const DiceContext = createContext<DiceContextValue | null>(null);

function useDiceContext(): DiceContextValue {
  const value = useContext(DiceContext);
  if (value === null) {
    throw new Error('the Dice controls need a DiceProvider above them');
  }
  return value;
}

/** The current Dice block of the sheet, re-rendered on every save. */
export function useDiceBlock(store: DiceStore): DiceBlock {
  const state = useSyncExternalStore(store.subscribe, store.getState);
  return state.block;
}

interface RollSession {
  readonly rowKey: string;
  readonly label: string;
  readonly outcome: DiceResult<RollResult>;
}

interface DiceSession {
  readonly editing: string | null;
  readonly setEditing: (rowKey: string | null) => void;
  readonly sendRoll: (session: RollSession) => void;
}

const DiceSessionContext = createContext<DiceSession>({
  editing: null,
  setEditing: () => undefined,
  sendRoll: () => undefined,
});

/** Owns the single Roll panel and the single open editor; a new Roll replaces the old panel. */
export function DiceProvider({
  value,
  children,
}: {
  readonly value: DiceContextValue;
  readonly children: ReactNode;
}) {
  const [roll, setRoll] = useState<RollSession | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const dismiss = useCallback(() => setRoll(null), []);
  const session = useMemo<DiceSession>(
    () => ({ editing, setEditing, sendRoll: setRoll }),
    [editing],
  );

  return (
    <DiceContext.Provider value={value}>
      <DiceSessionContext.Provider value={session}>
        {children}
        {roll !== null && (
          <DicePanel
            key={roll.rowKey}
            label={roll.label}
            outcome={roll.outcome}
            onDismiss={dismiss}
          />
        )}
      </DiceSessionContext.Provider>
    </DiceContext.Provider>
  );
}

interface DiceEditorProps {
  readonly rowKey: string;
  readonly label: string;
  readonly current: string;
  readonly onClose: () => void;
}

function DiceEditor({ rowKey, label, current, onClose }: DiceEditorProps) {
  const dice = useDiceContext();
  const [text, setText] = useState(current);
  const [errorText, setErrorText] = useState<string | undefined>(undefined);
  const inputId = `dice-expression-${rowKey.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
  const errorId = `${inputId}-error`;

  const finish = (expression: string | null | undefined) => {
    void dice.store.setRow(rowKey, expression).then(() => {
      const state = dice.store.getState();
      if (state.errorKey !== undefined) {
        setErrorText(message(state.errorKey));
        return;
      }
      onClose();
    });
  };

  const submit = () => {
    const expression = text.trim();
    const error = checkExpression(expression, dice.schema, rowKey);
    if (error !== undefined) {
      setErrorText(message(diceErrorMessageKey(error.class)));
      return;
    }
    setErrorText(undefined);
    finish(expression);
  };

  return (
    <form
      className="lg-dice-editor"
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <label className="lg-dice-editor__label" htmlFor={inputId}>
        {messageWith('dice.editLabel', { label })}
      </label>
      <input
        id={inputId}
        className="lg-dice-editor__input"
        type="text"
        value={text}
        autoComplete="off"
        spellCheck={false}
        aria-invalid={errorText !== undefined}
        aria-describedby={errorText !== undefined ? errorId : undefined}
        onChange={(event) => {
          setText(event.target.value);
          setErrorText(undefined);
        }}
      />
      {errorText !== undefined && (
        <p id={errorId} className="lg-dice-editor__error" role="alert">
          {errorText}
        </p>
      )}
      <div className="lg-dice-editor__actions">
        <button type="submit" className="lg-dice-button">
          {message('dice.save')}
        </button>
        <button type="button" className="lg-dice-button" onClick={() => finish(null)}>
          {message('dice.noDice')}
        </button>
        <button type="button" className="lg-dice-button" onClick={() => finish(undefined)}>
          {message('dice.useTemplate')}
        </button>
        <button type="button" className="lg-dice-button" onClick={onClose}>
          {message('dice.cancel')}
        </button>
      </div>
    </form>
  );
}

/** The Row frame that adds the Dice controls beside a Row's content; the default chrome is kept. */
export function DiceRowFrame({
  rowKey,
  kind,
  label,
  sectionKey,
  className,
  children,
}: RowFrameProps) {
  const dice = useDiceContext();
  const block = useDiceBlock(dice.store);
  const { editing, setEditing, sendRoll } = useContext(DiceSessionContext);
  const editButton = useRef<HTMLButtonElement>(null);
  const resolution: Resolution = resolveRowForSheet({
    rowKey,
    sectionKey,
    schema: dice.schema,
    defaults: dice.defaults,
    rowKeys: dice.rowKeys,
    sectionKeys: dice.sectionKeys,
    block,
  });
  const rollable = resolution.rollable;
  const isEditing = editing === rowKey;

  const roll = () => {
    sendRoll({
      rowKey,
      label,
      outcome: performRoll(resolution, {
        rowKey,
        values: dice.savedValues(),
        draft: dice.draftValues(),
      }),
    });
  };

  const close = () => {
    setEditing(null);
    editButton.current?.focus();
  };

  return (
    <div className={className} data-row-key={rowKey} data-row-kind={kind}>
      {children}
      {rollable && (
        <div className="lg-dice-row">
          <button
            type="button"
            className="lg-dice-button lg-dice-button--roll"
            aria-label={messageWith('dice.roll', { label })}
            onClick={roll}
          >
            {message('dice.rollShort')}
          </button>
          <button
            ref={editButton}
            type="button"
            className="lg-dice-button"
            aria-label={messageWith('dice.edit', { label })}
            aria-expanded={isEditing}
            onClick={() => setEditing(isEditing ? null : rowKey)}
          >
            {message('dice.editShort')}
          </button>
        </div>
      )}
      {rollable && isEditing && (
        <DiceEditor
          key={rowKey}
          rowKey={rowKey}
          label={label}
          current={resolution.expression ?? ''}
          onClose={close}
        />
      )}
    </div>
  );
}

/** The frames the sheet uses outside Arrange mode: the default Section chrome and the Dice Row frame. */
export const DICE_FRAMES: SheetFormFrames = { Row: DiceRowFrame };
