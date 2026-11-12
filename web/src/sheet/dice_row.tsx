/**
 * The Dice controls of the sheet (ADR 0005, LGN-166): on a Row, a Roll button that names the
 * dice it throws (a long press on it opens the editor) and a quieter pencil Edit button, at the
 * trailing edge of the Row's label next to its value; on a Section, a die icon button in the
 * title row for the Section expression, whose editor carries the note that a Section choice does not replace the
 * Template's Rows. The editor is one dialog and the Roll result one floating panel, both owned
 * by the provider and portalled to the document body, so neither reflows the sheet nor inherits
 * the sheet's column rules or a Row's containment. Arrange mode replaces these frames with its
 * own, so none of these controls is drawn while arranging.
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
import { createPortal } from 'react-dom';
import type { DiceBlock } from '../api/models/dice';
import { performRoll, type RollResult } from '../dice/roll';
import type { DiceResult } from '../dice/errors';
import type { DiceSchema } from '../dice/fit';
import type { DiceDefaults, Resolution } from '../dice/resolve';
import { message, messageWith } from '../l10n/strings';
import type { RowActionsInfo, SectionFrameProps, SheetFormFrames } from '../engine/sheet_form';
import { Dialog } from '../ui/dialog';
import type { DiceCheck, DiceStore, DiceTarget } from './dice_store';
import { diceErrorMessageKey, diceSummary, resolveRowForSheet } from './dice_view';
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

/** What the one open editor edits: a Row override or a Section expression. */
interface EditRequest {
  readonly target: DiceTarget;
  readonly keyName: string;
  readonly label: string;
  readonly current: string;
}

interface DiceSession {
  /** The open editor as `row:<key>` or `section:<key>`; at most one is open. */
  readonly editing: string | null;
  readonly openEditor: (request: EditRequest) => void;
  readonly sendRoll: (session: RollSession) => void;
}

const DiceSessionContext = createContext<DiceSession>({
  editing: null,
  openEditor: () => undefined,
  sendRoll: () => undefined,
});

function editKeyOf(target: DiceTarget, keyName: string): string {
  return `${target}:${keyName}`;
}

/**
 * Owns the single Roll panel and the single open editor; a new Roll replaces the old panel and
 * a new Roll gets a fresh panel key, so the same Row rolled twice is announced twice.
 */
export function DiceProvider({
  value,
  children,
}: {
  readonly value: DiceContextValue;
  readonly children: ReactNode;
}) {
  const [roll, setRoll] = useState<(RollSession & { readonly id: number }) | null>(null);
  const [request, setRequest] = useState<EditRequest | null>(null);
  const returnFocus = useRef<HTMLElement | null>(null);
  const rollCount = useRef(0);
  const dismiss = useCallback(() => setRoll(null), []);
  const sendRoll = useCallback((next: RollSession) => {
    rollCount.current += 1;
    setRoll({ ...next, id: rollCount.current });
  }, []);
  const openEditor = useCallback((next: EditRequest) => {
    returnFocus.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setRoll(null);
    setRequest(next);
  }, []);
  const closeEditor = useCallback(() => {
    setRequest(null);
    returnFocus.current?.focus();
    returnFocus.current = null;
  }, []);
  const editing = request === null ? null : editKeyOf(request.target, request.keyName);
  const session = useMemo<DiceSession>(
    () => ({ editing, openEditor, sendRoll }),
    [editing, openEditor, sendRoll],
  );

  return (
    <DiceContext.Provider value={value}>
      <DiceSessionContext.Provider value={session}>
        {children}
        {request !== null &&
          createPortal(
            <DiceEditor
              key={editing}
              target={request.target}
              keyName={request.keyName}
              label={request.label}
              current={request.current}
              onClose={closeEditor}
            />,
            document.body,
          )}
        {roll !== null &&
          createPortal(
            <DicePanel
              key={roll.id}
              label={roll.label}
              outcome={roll.outcome}
              onDismiss={dismiss}
            />,
            document.body,
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

  const title =
    target === 'section'
      ? messageWith('dice.editSection', { label })
      : messageWith('dice.edit', { label });

  return (
    <Dialog open title={title} onClose={onClose}>
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
          {message('dice.expressionLabel')}
        </label>
        <input
          ref={input}
          id={inputId}
          className="lg-dice-editor__input"
          type="text"
          value={text}
          placeholder={message('dice.expressionHint')}
          autoComplete="off"
          autoCapitalize="off"
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
        <div className="lg-dice-editor__alternatives">
          <button
            type="button"
            className="lg-button lg-button--ghost lg-button--sm"
            onClick={() => save(undefined)}
          >
            {message('dice.useTemplate')}
          </button>
          <button
            type="button"
            className="lg-button lg-button--ghost lg-button--sm"
            onClick={() => save(null)}
          >
            {message('dice.noDice')}
          </button>
        </div>
        <div className="lg-dice-editor__actions">
          <button
            type="button"
            className="lg-button lg-button--secondary lg-button--md"
            onClick={onClose}
          >
            {message('dice.cancel')}
          </button>
          <button type="submit" className="lg-button lg-button--primary lg-button--md">
            {message('dice.save')}
          </button>
        </div>
      </form>
    </Dialog>
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

/**
 * A Row's inline Dice actions, for the engine's `rowActionsBuilder`: the Roll button (a long press
 * on it opens the editor) and the Edit button, side by side on the Row's own line.
 */
export function DiceRowActions({ rowKey, sectionKey, label }: RowActionsInfo) {
  const dice = useDiceContext();
  const { editing, openEditor, sendRoll } = useContext(DiceSessionContext);
  const pressTimer = useRef<number | undefined>(undefined);
  const longPressed = useRef(false);
  const resolution = useRowResolution(rowKey, sectionKey);
  const isEditing = editing === editKeyOf('row', rowKey);
  const showEdit = rowOffersEdit(resolution);
  const edit = () =>
    openEditor({ target: 'row', keyName: rowKey, label, current: resolution.expression ?? '' });

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

  const summary = resolution.rollable ? diceSummary(resolution.parsed) : undefined;
  const editName = messageWith('dice.edit', { label });

  return (
    <span className="lg-dice-actions">
      {resolution.rollable && (
        <button
          type="button"
          className="lg-dice-button lg-dice-button--roll"
          aria-label={
            summary === undefined
              ? messageWith('dice.roll', { label })
              : messageWith('dice.rollWith', { label, dice: summary })
          }
          onPointerDown={() => {
            longPressed.current = false;
            cancelPress();
            pressTimer.current = window.setTimeout(() => {
              pressTimer.current = undefined;
              longPressed.current = true;
              edit();
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
          <DieIcon />
          <span className="lg-dice-button__text">{summary ?? message('dice.rollShort')}</span>
        </button>
      )}
      {showEdit && (
        <button
          type="button"
          className="lg-dice-button lg-dice-button--icon"
          aria-label={editName}
          title={editName}
          aria-haspopup="dialog"
          aria-expanded={isEditing}
          data-dice-off={resolution.rollable ? undefined : ''}
          onClick={edit}
        >
          {resolution.rollable ? <PencilIcon /> : <DieIcon off />}
        </button>
      )}
    </span>
  );
}

function sectionExpression(block: DiceBlock, defaults: DiceDefaults, sectionKey: string): string {
  if (Object.hasOwn(block.section, sectionKey)) {
    return block.section[sectionKey] ?? '';
  }
  return defaults.sectionDice?.[sectionKey] ?? '';
}

/**
 * The Section frame: the default chrome with the Section's Dice button at the trailing edge of
 * its title row, so the button costs no line of its own.
 */
export function DiceSectionFrame({
  sectionKey,
  label,
  className,
  title,
  children,
}: SectionFrameProps) {
  const dice = useDiceContext();
  const block = useDiceBlock(dice.store);
  const { editing, openEditor } = useContext(DiceSessionContext);
  const isEditing = editing === editKeyOf('section', sectionKey);

  const expression = sectionExpression(block, dice.defaults, sectionKey);
  const name = messageWith('dice.editSection', { label });

  return (
    <section className={className} data-section-key={sectionKey}>
      <div className="lg-dice-section">
        {title}
        <button
          type="button"
          className="lg-dice-button lg-dice-button--icon lg-dice-section__button"
          aria-label={name}
          title={name}
          aria-haspopup="dialog"
          aria-expanded={isEditing}
          data-dice-set={expression === '' ? undefined : ''}
          onClick={() =>
            openEditor({ target: 'section', keyName: sectionKey, label, current: expression })
          }
        >
          <DieIcon />
        </button>
      </div>
      {children}
    </section>
  );
}

/**
 * A six-sided die drawn on `currentColor`; `off` strikes it through for a Row whose dice the
 * player turned off. The icons share one 24-unit grid and stroke so they read as one set.
 */
function DieIcon({ off = false }: { readonly off?: boolean }) {
  return (
    <svg className="lg-dice-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <rect x="3.5" y="3.5" width="17" height="17" rx="3.5" />
      <circle cx="8.5" cy="8.5" r="1.4" />
      <circle cx="12" cy="12" r="1.4" />
      <circle cx="15.5" cy="15.5" r="1.4" />
      {off && <path d="M3 21 21 3" />}
    </svg>
  );
}

/** A pencil drawn on the same grid as {@link DieIcon}: the Row's edit-dice action. */
function PencilIcon() {
  return (
    <svg className="lg-dice-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M15.5 4.5 19.5 8.5 9 19H5v-4z" />
      <path d="M13 7l4 4" />
    </svg>
  );
}

/** The frames the sheet uses outside Arrange mode: the Dice Section frame. */
export const DICE_FRAMES: SheetFormFrames = { Section: DiceSectionFrame };
