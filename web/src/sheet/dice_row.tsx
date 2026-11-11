/**
 * The Dice controls of the sheet (ADR 0005, LGN-166): on a Row, a Roll button
 * (a long press on it opens the editor) and an Edit button, inline on the Row's line; on a
 * Section, a Dice button for the Section expression, whose editor carries the note that a Section
 * choice does not replace the Template's Rows. One Roll panel and one open editor at
 * a time. Arrange mode replaces these frames with its own, so none of these
 * controls is drawn while arranging.
 */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
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
import type { DiceDefaults, Resolution } from '../dice/resolve';
import { message, messageWith } from '../l10n/strings';
import type {
  RowActionsInfo,
  RowFrameProps,
  SectionFrameProps,
  SheetFormFrames,
} from '../engine/sheet_form';
import type { DiceCheck, DiceStore, DiceTarget } from './dice_store';
import { diceErrorMessageKey, resolveRowForSheet } from './dice_view';
import { DicePanel } from './dice_panel';
import './dice.css';

/** How long a press on the Roll button must last to open the editor instead of rolling. */
export const LONG_PRESS_MS = 500;

/** What the Dice controls of one sheet read and write. */
export interface DiceContextValue {
  readonly store: DiceStore;
  readonly schema: DiceSchema;
  readonly defaults: DiceDefaults;
  readonly check: DiceCheck;
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
  /** The open editor as `row:<key>` or `section:<key>`; at most one is open. */
  readonly editing: string | null;
  readonly setEditing: (editKey: string | null) => void;
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
  readonly target: DiceTarget;
  readonly keyName: string;
  readonly label: string;
  readonly current: string;
  readonly onClose: () => void;
}

function DiceEditor({ target, keyName, label, current, onClose }: DiceEditorProps) {
  const dice = useDiceContext();
  const [text, setText] = useState(current);
  const [errorText, setErrorText] = useState<string | undefined>(undefined);
  const input = useRef<HTMLInputElement>(null);
  const inputId = `dice-expression-${target}-${keyName.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
  const errorId = `${inputId}-error`;

  useEffect(() => {
    input.current?.focus();
  }, []);

  const save = (expression: string | null | undefined) => {
    const change =
      target === 'row'
        ? dice.store.setRow(keyName, expression)
        : dice.store.setSection(keyName, expression);
    void change.then((outcome) => {
      if (!outcome.ok) {
        setErrorText(message(diceErrorMessageKey(outcome.error.class)));
        return;
      }
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
    const error = dice.check(target, keyName, expression);
    if (error !== undefined) {
      setErrorText(message(diceErrorMessageKey(error.class)));
      return;
    }
    setErrorText(undefined);
    save(expression);
  };

  return (
    <form
      className="lg-dice-editor"
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      {target === 'section' && (
        <p className="lg-dice-editor__note">{message('dice.sectionNote')}</p>
      )}
      <label className="lg-dice-editor__label" htmlFor={inputId}>
        {messageWith('dice.editLabel', { label })}
      </label>
      <input
        ref={input}
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
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            event.stopPropagation();
            onClose();
          }
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
        <button type="button" className="lg-dice-button" onClick={() => save(null)}>
          {message('dice.noDice')}
        </button>
        <button type="button" className="lg-dice-button" onClick={() => save(undefined)}>
          {message('dice.useTemplate')}
        </button>
        <button type="button" className="lg-dice-button" onClick={onClose}>
          {message('dice.cancel')}
        </button>
      </div>
    </form>
  );
}

/** Whether a Row shows the Edit button: a rollable Row, or one the player turned off (the way back). */
function rowOffersEdit(resolution: Resolution): boolean {
  return (
    resolution.rollable ||
    resolution.source === 'sheet-row-off' ||
    resolution.source === 'sheet-section-off'
  );
}

function useRowResolution(rowKey: string, sectionKey: string): Resolution {
  const dice = useDiceContext();
  const block = useDiceBlock(dice.store);
  return resolveRowForSheet({
    rowKey,
    sectionKey,
    schema: dice.schema,
    defaults: dice.defaults,
    rowKeys: dice.rowKeys,
    sectionKeys: dice.sectionKeys,
    block,
  });
}

const EDIT_BUTTON_ATTRIBUTE = 'data-dice-edit';

/**
 * A Row's inline Dice actions, for the engine's `rowActionsBuilder`: the Roll button (a long press
 * on it opens the editor) and the Edit button, side by side on the Row's own line.
 */
export function DiceRowActions({ rowKey, sectionKey, label }: RowActionsInfo) {
  const dice = useDiceContext();
  const { editing, setEditing, sendRoll } = useContext(DiceSessionContext);
  const pressTimer = useRef<number | undefined>(undefined);
  const longPressed = useRef(false);
  const resolution = useRowResolution(rowKey, sectionKey);
  const editKey = `row:${rowKey}`;
  const isEditing = editing === editKey;
  const showEdit = rowOffersEdit(resolution);

  const cancelPress = () => {
    if (pressTimer.current !== undefined) {
      window.clearTimeout(pressTimer.current);
      pressTimer.current = undefined;
    }
  };

  useEffect(() => cancelPress, []);

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

  if (!resolution.rollable && !showEdit) {
    return null;
  }

  return (
    <span className="lg-dice-actions">
      {resolution.rollable && (
        <button
          type="button"
          className="lg-dice-button lg-dice-button--roll"
          aria-label={messageWith('dice.roll', { label })}
          onPointerDown={() => {
            longPressed.current = false;
            cancelPress();
            pressTimer.current = window.setTimeout(() => {
              pressTimer.current = undefined;
              longPressed.current = true;
              setEditing(editKey);
            }, LONG_PRESS_MS);
          }}
          onPointerUp={cancelPress}
          onPointerLeave={cancelPress}
          onPointerCancel={cancelPress}
          onClick={() => {
            if (longPressed.current) {
              longPressed.current = false;
              return;
            }
            roll();
          }}
        >
          {message('dice.rollShort')}
        </button>
      )}
      {showEdit && (
        <button
          {...{ [EDIT_BUTTON_ATTRIBUTE]: editKey }}
          type="button"
          className="lg-dice-button"
          aria-label={messageWith('dice.edit', { label })}
          aria-expanded={isEditing}
          onClick={() => setEditing(isEditing ? null : editKey)}
        >
          {message('dice.editShort')}
        </button>
      )}
    </span>
  );
}

/** The Row frame that adds the Dice editor under a Row's content; the default chrome is kept. */
export function DiceRowFrame({
  rowKey,
  kind,
  label,
  sectionKey,
  className,
  children,
}: RowFrameProps) {
  const { editing, setEditing } = useContext(DiceSessionContext);
  const frame = useRef<HTMLDivElement>(null);
  const resolution = useRowResolution(rowKey, sectionKey);
  const editKey = `row:${rowKey}`;
  const isEditing = editing === editKey;
  const showEdit = rowOffersEdit(resolution);

  const close = () => {
    setEditing(null);
    frame.current?.querySelector<HTMLElement>(`[${EDIT_BUTTON_ATTRIBUTE}]`)?.focus();
  };

  return (
    <div ref={frame} className={className} data-row-key={rowKey} data-row-kind={kind}>
      {children}
      {showEdit && isEditing && (
        <DiceEditor
          key={editKey}
          target="row"
          keyName={rowKey}
          label={label}
          current={resolution.expression ?? ''}
          onClose={close}
        />
      )}
    </div>
  );
}

function sectionExpression(block: DiceBlock, defaults: DiceDefaults, sectionKey: string): string {
  if (Object.hasOwn(block.section, sectionKey)) {
    return block.section[sectionKey] ?? '';
  }
  return defaults.sectionDice?.[sectionKey] ?? '';
}

/** The Section frame: the default chrome plus the note on what a Section choice means, and its expression editor. */
export function DiceSectionFrame({
  sectionKey,
  label,
  className,
  title,
  children,
}: SectionFrameProps) {
  const dice = useDiceContext();
  const block = useDiceBlock(dice.store);
  const { editing, setEditing } = useContext(DiceSessionContext);
  const editButton = useRef<HTMLButtonElement>(null);
  const editKey = `section:${sectionKey}`;
  const isEditing = editing === editKey;

  const close = () => {
    setEditing(null);
    editButton.current?.focus();
  };

  return (
    <section className={className} data-section-key={sectionKey}>
      {title}
      <div className="lg-dice-section">
        <button
          ref={editButton}
          type="button"
          className="lg-dice-button"
          aria-label={messageWith('dice.editSection', { label })}
          aria-expanded={isEditing}
          onClick={() => setEditing(isEditing ? null : editKey)}
        >
          {message('dice.sectionButton')}
        </button>
      </div>
      {isEditing && (
        <DiceEditor
          key={editKey}
          target="section"
          keyName={sectionKey}
          label={label}
          current={sectionExpression(block, dice.defaults, sectionKey)}
          onClose={close}
        />
      )}
      {children}
    </section>
  );
}

/** The frames the sheet uses outside Arrange mode: the Dice Section and Row frames. */
export const DICE_FRAMES: SheetFormFrames = { Section: DiceSectionFrame, Row: DiceRowFrame };
