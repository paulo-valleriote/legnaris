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
import { markLabel } from '../engine/sheet_form';
import { Dialog } from '../ui/dialog';
import type { DiceCheck, DiceStore, DiceTarget } from './dice_store';
import type { PendingDiceKeys } from './sheet_edit';
import { diceErrorMessageKey, diceSummary, resolveRowForSheet } from './dice_view';
import { DicePanel } from './dice_panel';
import { DieIcon, dieShapeFor } from './die_icon';
import './dice.css';

export const LONG_PRESS_MS = 500;

export interface DiceContextValue {
  readonly store: DiceStore;
  readonly schema: DiceSchema;
  readonly defaults: DiceDefaults;
  readonly check: DiceCheck;
  readonly rowKeys: ReadonlySet<string>;
  readonly sectionRows: ReadonlyMap<string, readonly string[]>;
  readonly savedValues: () => Record<string, unknown>;
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

export function useDiceBlock(store: DiceStore): DiceBlock {
  return useSyncExternalStore(store.subscribe, store.getBlock);
}

export function usePendingDiceKeys(store: DiceStore): PendingDiceKeys {
  return useSyncExternalStore(store.subscribe, store.getPendingKeys);
}

interface RollSession {
  readonly rowKey: string;
  readonly label: string;
  readonly outcome: DiceResult<RollResult>;
}

interface EditRequest {
  readonly target: DiceTarget;
  readonly keyName: string;
  readonly label: string;
  readonly current: string;
}

interface DiceSession {
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
    if (!change.ok) {
      setErrorText(message(diceErrorMessageKey(change.error.class)));
      return;
    }
    onClose();
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
    sectionRows: dice.sectionRows,
    block,
  });
}

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
          <DieIcon shape={dieShapeFor(resolution.parsed)} />
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

export function DiceSectionFrame({
  sectionKey,
  label,
  className,
  title,
  children,
}: SectionFrameProps) {
  const dice = useDiceContext();
  const block = useDiceBlock(dice.store);
  const pending = usePendingDiceKeys(dice.store).section.has(sectionKey);
  const { editing, openEditor } = useContext(DiceSessionContext);
  const isEditing = editing === editKeyOf('section', sectionKey);

  const expression = sectionExpression(block, dice.defaults, sectionKey);
  const name = messageWith('dice.editSection', { label });

  return (
    <section className={className} data-section-key={sectionKey}>
      <div className="lg-dice-section">
        {title}
        {pending && (
          <span className="lg-field-row__mark" data-mark="modified">
            {markLabel('modified')}
          </span>
        )}
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

function PencilIcon() {
  return (
    <svg className="lg-dice-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M15.5 4.5 19.5 8.5 9 19H5v-4z" />
      <path d="M13 7l4 4" />
    </svg>
  );
}

export const DICE_FRAMES: SheetFormFrames = { Section: DiceSectionFrame };
