/**
 * The sheet screen (WSC-05 … WSC-25, WSC-32, WSC-35 … WSC-39): reads the `:id`
 * route parameter, resolves the open situation and renders it. A loaded sheet
 * shows its name, mounts the F4 engine in builder mode over an edit store, and
 * renders a footer with the dirty indicator, the Preview and Save actions and
 * their inline results; the loading, missing, retryable and template situations
 * each render their own mapped copy.
 *
 * The screen owns the per-field subscription through the engine's builder seams
 * (AD-001): a control reads its own path from the store and reports an edit back,
 * and the preview marks reach the rows through the engine's mark seam.
 *
 * It also guards the unsaved work (WSC-36, WSC-37): `F5` refreshes the template
 * detail and leaving the screen asks for a discard confirmation first, both
 * through one modal that never stacks. A pending arrangement (Arrange mode, ADR
 * 0001) counts as unsaved work for both guards but never as unsaved values: its
 * store is separate from the edit store. A pin toggled in pin mode (ADR 0002) is saved at once,
 * so it is never unsaved work; while its request is in flight the screen counts as working, which
 * holds back a leave and `F5` exactly as an Arrange commit in flight does.
 */

import type { FailureKind } from '../api/errors';
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import type { ReactNode, RefObject } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import {
  EXCLUSIVE_NONE_VALUE,
  exclusiveValueIsOn,
  SheetExclusiveControl,
} from '../engine/exclusive_control';
import { DependentReadout } from '../engine/dependent_readout';
import { dependentsAreStale } from '../engine/dependents';
import { SheetFieldControl } from '../engine/field_control';
import type { SheetExclusiveGroup, SheetField } from '../engine/field_grouping';
import type { FieldNode } from '../api/models/template';
import { buildSections, fieldDisplayName, lastSegment, rowKey } from '../engine/field_grouping';
import type { SheetMark } from '../engine/sheet_mark';
import { markLabel, SheetForm } from '../engine/sheet_form';
import type { RowActionsInfo } from '../engine/sheet_form';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { useApiClient, useSheetStore } from '../app/providers';
import { templateDetailKey, updateSheetLayout } from '../catalog/queries';
import { startDraftCadence } from '../library/draft_backup';
import { discardDraft, discardLayoutDraft } from '../library/sheet_library';
import { registerLeaveGuard } from '../shell/leave_guard';
import { useSheetShortcuts } from '../shell/shortcut_registry';
import type { PreviewMarks, SheetEditState, SheetEditStore } from './sheet_edit';
import {
  canPreview,
  canSave,
  createSheetEditStore,
  isBusy,
  isPending,
  pendingCount,
  markForPaths,
} from './sheet_edit';
import { ARRANGE_FRAMES, ArrangeProvider } from './arrange_frames';
import { createDiceStore, type DiceCheck, type DiceStore } from './dice_store';
import { DICE_FRAMES, DiceProvider, DiceRowActions, type DiceContextValue } from './dice_row';
import {
  checkExpression,
  checkSectionExpression,
  diceSchemaFromFields,
  templateDiceDefaults,
} from './dice_view';
import type { LayoutState, LayoutStore } from './layout_store';
import { createLayoutStore, drawnArrangement, isArranging, isLayoutPending } from './layout_store';
import { footerStatus, recalculatedCount, recalculatedSummary } from './preview_summary';
import type { RevealView } from './reveal_row';
import { revealFirstRecalculated, revealSection, revealTop } from './reveal_row';
import { SectionIndex } from './section_index';
import { CollapseAllButton, PinnedEmpty, TemplateSettingsArea, useSheetView } from './sheet_view';
import { isTemplateSettingsKey } from './view_store';
import { HelpPanel, HelpToggle } from './sheet_help';
import type { SheetOpenResult } from './sheet_open';
import { useSheetOpen } from './sheet_open';
import './sheet.css';

interface RetryActionProps {
  readonly copy: string;
  readonly onRetry: () => void;
}

function RetryAction({ copy, onRetry }: RetryActionProps) {
  return (
    <div className="lg-sheet__failure">
      <p className="lg-sheet__message">{copy}</p>
      <Button variant="primary" size="sm" label={message('sheet.retryOpen')} onClick={onRetry} />
    </div>
  );
}

/** Subscribes to the whole edit state; the footer and the marks read this (WSC-12, WSC-23). */
/** The footer error class; an offline failure is not the player's fault, so it is not danger-toned. */
function errorClass(kind: FailureKind | undefined): string {
  return kind === 'offline' ? 'lg-sheet__error lg-sheet__error--offline' : 'lg-sheet__error';
}

function useEditState(store: SheetEditStore): SheetEditState {
  return useSyncExternalStore(store.subscribe, store.getState);
}

/** Subscribes to the preview marks, resolving the mark [paths] carry (WSC-23). */
function usePreviewMark(store: SheetEditStore, paths: readonly string[]): SheetMark | undefined {
  const subscribe = useCallback((listener: () => void) => store.subscribeMarks(listener), [store]);
  return useSyncExternalStore(subscribe, () => markForPaths(store.getState().previewMarks, paths));
}

interface ConnectedFieldProps {
  readonly store: SheetEditStore;
  readonly field: SheetField;
  readonly labelId?: string;
  readonly describedById?: string;
}

/** One editable control, subscribed to its own path so an unrelated edit never re-renders it (AD-001). */
function ConnectedField({ store, field, labelId, describedById }: ConnectedFieldProps) {
  const subscribe = useCallback(
    (listener: () => void) => store.subscribePath(field.path, listener),
    [store, field.path],
  );
  const value = useSyncExternalStore(subscribe, () =>
    valueAtPath(store.getState().data, field.path),
  );
  const mark = usePreviewMark(store, [field.path]);

  return (
    <SheetFieldControl
      path={field.path}
      schema={field.schema}
      value={value}
      mark={mark}
      labelId={labelId}
      describedBy={describedById}
      onChanged={(next) => store.setPath(field.path, next)}
    />
  );
}

interface ConnectedDependentProps {
  readonly store: SheetEditStore;
  readonly dependent: SheetField;
  readonly source: SheetField;
}

/**
 * One derived value beside its input. It follows the derived leaf's own path, so a Preview or a
 * Save that merges the cascade repaints it, and it reads stale from the input's edit state and the
 * preview marks, both coarse booleans, so only a change of that answer re-renders it.
 */
function ConnectedDependent({ store, dependent, source }: ConnectedDependentProps) {
  const subscribe = useCallback(
    (listener: () => void) => store.subscribePath(dependent.path, listener),
    [store, dependent.path],
  );
  const value = useSyncExternalStore(subscribe, () =>
    valueAtPath(store.getState().data, dependent.path),
  );
  const stale = useSyncExternalStore(store.subscribe, () => {
    const state = store.getState();
    return dependentsAreStale(state.editedPaths.has(source.path), state.previewMarks !== undefined);
  });

  return (
    <DependentReadout
      label={fieldDisplayName(lastSegment(dependent.path), dependent.schema)}
      value={value}
      stale={stale}
    />
  );
}

interface ConnectedExclusiveProps {
  readonly store: SheetEditStore;
  readonly group: SheetExclusiveGroup;
}

/** One exclusive group, subscribed to each member's path so a selection repaints it (WSC-15). */
function ConnectedExclusive({ store, group }: ConnectedExclusiveProps) {
  const paths = group.options.map((option) => option.path);
  const pathsKey = paths.join('\u0000');
  const subscribe = useCallback(
    (listener: () => void) => {
      const unsubscribes = pathsKey
        .split('\u0000')
        .map((path) => store.subscribePath(path, listener));
      return () => {
        for (const unsubscribe of unsubscribes) {
          unsubscribe();
        }
      };
    },
    [store, pathsKey],
  );
  const selected = useSyncExternalStore(subscribe, () => {
    const data = store.getState().data;
    for (const path of paths) {
      if (exclusiveValueIsOn(valueAtPath(data, path))) {
        return path;
      }
    }
    return undefined;
  });
  const mark = usePreviewMark(store, paths);

  return (
    <SheetExclusiveControl
      group={group}
      selectedPath={selected}
      mark={mark}
      onSelect={(signal) => {
        store.setExclusive(signal === EXCLUSIVE_NONE_VALUE ? null : signal, paths);
      }}
    />
  );
}

interface SheetFooterProps {
  readonly store: SheetEditStore;
  readonly reveal: RevealView;
}

interface FooterStatusProps {
  readonly pending: boolean;
  readonly marks: PreviewMarks | undefined;
  readonly reveal: RevealView;
  readonly children?: ReactNode;
}

/**
 * The footer's single status line: the unsaved state and the last Preview's cascade in one phrase,
 * with the Jump control inline. It owns the sheet's one polite live region, which announces only the
 * cascade; the visible phrase is hidden from assistive technology while a Preview shows, so the
 * summary is read once. With nothing to say the line is not drawn and, with no [children] (the
 * error slots), the status block is empty and takes no room.
 */
function FooterStatus({ pending, marks, reveal, children }: FooterStatusProps) {
  const count = marks === undefined ? undefined : recalculatedCount(marks);
  const summary = count === undefined ? '' : recalculatedSummary(count);
  const status = footerStatus(pending, count);

  return (
    <>
      <p className="lg-visually-hidden" aria-live="polite" aria-atomic="true">
        {summary}
      </p>
      <div className="lg-sheet__status">
        {status !== '' && (
          <div className="lg-sheet__status-line">
            <p
              className="lg-sheet__status-text"
              aria-hidden={count === undefined ? undefined : true}
              data-testid="sheet-status"
            >
              {status}
            </p>
            {count !== undefined && count > 0 && (
              <Button
                variant="ghost"
                size="sm"
                label={message('sheet.jumpToFirst')}
                name={message('sheet.jumpToFirstName')}
                onClick={() => {
                  revealFirstRecalculated(document, reveal);
                }}
              />
            )}
          </div>
        )}
        {children}
      </div>
    </>
  );
}

/** The fixed footer: the dirty indicator, the Preview and Save actions and their inline results (WSC-12, WSC-16, WSC-17, WSC-21, WSC-25). */
function SheetFooter({ store, reveal }: SheetFooterProps) {
  const state = useEditState(store);
  const [savedRevision, setSavedRevision] = useState<number | undefined>(undefined);
  const pending = isPending(state);
  const changes = pendingCount(state);
  const savable = canSave(state);
  const previewable = canPreview(state);
  const showSaved =
    savedRevision !== undefined &&
    savedRevision === state.revision &&
    !state.isSaving &&
    !state.isPreviewing &&
    !pending;

  const runSave = useCallback(async () => {
    setSavedRevision(undefined);
    await store.save();
    const after = store.getState();
    if (after.saveError === undefined && !isPending(after)) {
      setSavedRevision(after.revision);
    }
  }, [store]);

  const quiet =
    !pending &&
    state.previewMarks === undefined &&
    state.previewError === undefined &&
    state.saveError === undefined &&
    !state.isSaving &&
    !state.isPreviewing &&
    !showSaved;

  return (
    <footer
      className={`lg-sheet__footer ${quiet ? 'lg-sheet__footer--clean' : 'lg-sheet__footer--dirty'}`}
    >
      <FooterStatus pending={pending} marks={state.previewMarks} reveal={reveal}>
        {state.previewError !== undefined && (
          <p
            className={errorClass(state.previewErrorKind)}
            role="alert"
            data-testid="sheet-preview-error"
          >
            {state.previewError}
          </p>
        )}
        {state.saveError !== undefined && (
          <p
            className={errorClass(state.saveErrorKind)}
            role="alert"
            data-testid="sheet-save-error"
          >
            {state.saveError}
          </p>
        )}
      </FooterStatus>
      <div className="lg-sheet__actions">
        {changes > 0 && (
          <span className="lg-sheet__count" data-testid="sheet-change-count">
            {changes === 1
              ? message('sheet.changeCountOne')
              : messageWith('sheet.changeCount', { count: changes })}
          </span>
        )}
        <Button
          variant="secondary"
          size="md"
          disabled={!previewable}
          loading={state.isPreviewing}
          label={state.isPreviewing ? message('sheet.previewing') : message('sheet.preview')}
          onClick={() => {
            void store.preview();
          }}
        />
        <Button
          variant="primary"
          size="md"
          disabled={!savable}
          loading={state.isSaving}
          label={
            state.isSaving
              ? message('sheet.saving')
              : showSaved
                ? message('sheet.saved')
                : message('sheet.save')
          }
          onClick={() => {
            void runSave();
          }}
        />
      </div>
    </footer>
  );
}

/** Subscribes to the arrangement state; the header, the footer and the form read this. */
function useLayoutState(store: LayoutStore): LayoutState {
  return useSyncExternalStore(store.subscribe, store.getState);
}

interface ArrangeFooterProps {
  readonly store: LayoutStore;
}

/** The footer while arranging: the pending indicator, the commit failure and the Reset, Cancel and Done actions. */
function ArrangeFooter({ store }: ArrangeFooterProps) {
  const state = useLayoutState(store);
  const pending = isLayoutPending(state);

  return (
    <footer className="lg-sheet__footer" aria-label={message('arrange.toolbar')}>
      <div className="lg-sheet__status">
        {pending && (
          <p className="lg-sheet__dirty" data-testid="arrange-pending">
            {message('arrange.pending')}
          </p>
        )}
        {state.commitError !== undefined && (
          <p className="lg-sheet__error" role="alert" data-testid="arrange-error">
            {state.commitError}
          </p>
        )}
      </div>
      <div className="lg-sheet__actions">
        <Button
          variant="outline"
          size="sm"
          disabled={state.isCommitting}
          label={message('arrange.reset')}
          onClick={() => store.reset()}
        />
        <Button
          variant="secondary"
          size="sm"
          disabled={state.isCommitting}
          label={message('arrange.cancel')}
          onClick={() => store.cancel()}
        />
        <Button
          variant="primary"
          size="sm"
          loading={state.isCommitting}
          label={state.isCommitting ? message('arrange.saving') : message('arrange.done')}
          onClick={() => {
            void store.commit();
          }}
        />
      </div>
    </footer>
  );
}

interface ArrangeToggleProps {
  readonly store: LayoutStore;
  readonly buttonRef: RefObject<HTMLButtonElement | null>;
  readonly onStart: () => void;
}

/** The sheet header's entry to Arrange mode; it is a button, so it takes part in no drag. */
function ArrangeToggle({ store, buttonRef, onStart }: ArrangeToggleProps) {
  const state = useLayoutState(store);
  if (isArranging(state)) {
    return null;
  }
  return (
    <button
      type="button"
      className="lg-button lg-button--ghost lg-button--sm"
      ref={buttonRef}
      onClick={onStart}
    >
      {message('arrange.start')}
    </button>
  );
}

/** The one pending discard the guards can open: a template refresh or a leave (WSC-36, WSC-37). */
interface PendingConfirmation {
  readonly kind: 'refresh' | 'leave';
  /** Runs once the person confirms the discard. */
  readonly proceed: () => void;
}

interface SheetConfirmationProps {
  readonly kind: 'refresh' | 'leave';
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}

/**
 * The one modal both guards open (WSC-36, WSC-37, WSC-39): a labelled, modal
 * dialog whose safe Cancel action carries focus, whose destructive action is a
 * real button (never a confirm-by-Enter default), and where Escape and the
 * overlay barrier both cancel (keeping the edits).
 */
function SheetConfirmation({ kind, onCancel, onConfirm }: SheetConfirmationProps) {
  const cancelRef = useRef<HTMLButtonElement>(null);
  const copy =
    kind === 'refresh'
      ? {
          title: message('sheet.refreshTitle'),
          body: message('sheet.refreshMessage'),
          confirm: message('sheet.refreshConfirm'),
        }
      : {
          title: message('sheet.leaveTitle'),
          body: message('sheet.leaveMessage'),
          confirm: message('sheet.leaveConfirm'),
        };

  useEffect(() => {
    cancelRef.current?.focus();
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onCancel();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onCancel]);

  return (
    <div
      className="lg-overlay"
      role="presentation"
      data-testid="sheet-confirm-overlay"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onCancel();
        }
      }}
    >
      <div
        className="lg-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="sheet-confirm-title"
        aria-describedby="sheet-confirm-message"
        data-testid={`sheet-confirm-${kind}`}
      >
        <div className="lg-dialog__header">
          <h2 className="lg-dialog__title" id="sheet-confirm-title">
            {copy.title}
          </h2>
        </div>
        <p className="lg-dialog__message" id="sheet-confirm-message">
          {copy.body}
        </p>
        <div className="lg-dialog__actions">
          <button type="button" className="lg-button" ref={cancelRef} onClick={onCancel}>
            {message('sheet.cancel')}
          </button>
          <button type="button" className="lg-button lg-button--danger" onClick={onConfirm}>
            {copy.confirm}
          </button>
        </div>
      </div>
    </div>
  );
}

interface LoadedSheetProps {
  readonly sheet: Sheet;
  readonly template: TemplateDetail;
}

function LoadedSheet({ sheet, template }: LoadedSheetProps) {
  const client = useApiClient();
  const storePort = useSheetStore();
  const queryClient = useQueryClient();
  const sheetId = sheet.id;
  const sheetData = sheet.data;
  const [committedLayout] = useState(sheet.layout);
  const editStore = useMemo(
    () =>
      createSheetEditStore(
        { template, sheet: { id: sheetId, data: sheetData } },
        { client, store: storePort },
      ),
    [client, storePort, sheetId, sheetData, template],
  );
  const sections = useMemo(() => buildSections(template.fields), [template]);
  const mainSections = useMemo(
    () => sections.filter((section) => !isTemplateSettingsKey(section.key)),
    [sections],
  );
  const mainFields = useMemo(
    () => fieldsWhere(template.fields, (key) => !isTemplateSettingsKey(key)),
    [template],
  );
  const settingsFields = useMemo(
    () => fieldsWhere(template.fields, isTemplateSettingsKey),
    [template],
  );
  const hasSettings = mainSections.length < sections.length;
  const diceSchema = useMemo(() => diceSchemaFromFields(template.fields), [template]);
  const diceCheck = useCallback<DiceCheck>(
    (target, key, expression) =>
      target === 'row'
        ? checkExpression(expression, diceSchema, key)
        : checkSectionExpression(expression),
    [diceSchema],
  );
  const diceStore = useMemo<DiceStore>(
    () =>
      createDiceStore({
        client,
        sheetId,
        initial: sheet.dice ?? { section: {}, row: {} },
        check: diceCheck,
      }),
    [client, sheetId, sheet.dice, diceCheck],
  );
  const diceValue = useMemo<DiceContextValue>(
    () => ({
      store: diceStore,
      schema: diceSchema,
      check: diceCheck,
      defaults: templateDiceDefaults(template.dice),
      rowKeys: new Set(sections.flatMap((section) => section.rows.map(rowKey))),
      sectionKeys: new Set(sections.map((section) => section.key)),
      savedValues: () => editStore.getState().defaults,
      draftValues: () => editStore.getPendingDelta(),
    }),
    [diceStore, diceSchema, diceCheck, template, sections, editStore],
  );
  const eyebrow = template.name.trim() === '' ? message('sheet.headerTitle') : template.name;
  const layoutStore = useMemo(
    () =>
      createLayoutStore(
        { sheetId, sections: mainSections, layout: committedLayout },
        {
          client,
          store: storePort,
          onCommitted: (layout) => updateSheetLayout(queryClient, sheetId, layout),
        },
      ),
    [client, storePort, queryClient, sheetId, mainSections, committedLayout],
  );
  const layoutState = useLayoutState(layoutStore);
  const arranging = isArranging(layoutState);
  const arrangement = drawnArrangement(layoutState);
  const marks = useSyncExternalStore<PreviewMarks | undefined>(
    editStore.subscribeMarks,
    () => editStore.getState().previewMarks,
  );
  const view = useSheetView({
    sheetId,
    sections: mainSections,
    arrangement,
    arranging,
    marks,
  });
  const hasCalculated = useMemo(
    () => sections.some((section) => section.leaves.some((leaf) => !leaf.schema.isEditable)),
    [sections],
  );
  const [helpOpen, setHelpOpen] = useState(() => hasCalculated && !view.state.explainerSeen);
  const helpToggleRef = useRef<HTMLButtonElement>(null);
  const closeHelp = useCallback(() => {
    setHelpOpen(false);
    view.store.markExplainerSeen();
  }, [view.store]);
  const [confirmation, setConfirmation] = useState<PendingConfirmation | null>(null);
  const arrangeToggleRef = useRef<HTMLButtonElement>(null);
  const arrangeHintRef = useRef<HTMLParagraphElement>(null);
  const focusArrangeHint = useRef(false);
  const startArranging = useCallback(() => {
    focusArrangeHint.current = true;
    layoutStore.enter();
  }, [layoutStore]);
  const wasArranging = useRef(false);

  const controlBuilder = useCallback(
    (field: SheetField, labelId?: string, describedById?: string) => (
      <ConnectedField
        store={editStore}
        field={field}
        labelId={labelId}
        describedById={describedById}
      />
    ),
    [editStore],
  );
  const dependentBuilder = useCallback(
    (dependent: SheetField, source: SheetField) => (
      <ConnectedDependent store={editStore} dependent={dependent} source={source} />
    ),
    [editStore],
  );
  const exclusiveControlBuilder = useCallback(
    (group: SheetExclusiveGroup) => <ConnectedExclusive store={editStore} group={group} />,
    [editStore],
  );
  const rowMarkBuilder = useCallback(
    (paths: readonly string[]) => {
      const mark = markForPaths(marks, paths);
      return mark === undefined ? null : (
        <span className="lg-field-row__mark" data-mark={mark}>
          {markLabel(mark)}
        </span>
      );
    },
    [marks],
  );

  const renderDiceActions = useCallback((info: RowActionsInfo) => <DiceRowActions {...info} />, []);

  const sectionBadgeBuilder = useCallback(
    (rowPaths: readonly (readonly string[])[]) => {
      const count = rowPaths.filter((paths) => markForPaths(marks, paths) !== undefined).length;
      return count === 0 ? null : (
        <span className="lg-field-row__mark">
          {messageWith('engine.sectionChanged', { count })}
        </span>
      );
    },
    [marks],
  );

  const scrollToTop = useCallback(() => {
    revealTop(document);
  }, []);

  const jumpToSection = useCallback(
    (key: string) => {
      revealSection(key, document, arranging ? undefined : view.reveal);
    },
    [arranging, view.reveal],
  );

  const refreshTemplate = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: templateDetailKey(sheet.templateId) });
  }, [queryClient, sheet.templateId]);

  const hasUnsavedWork = useCallback(
    () => isPending(editStore.getState()) || isLayoutPending(layoutStore.getState()),
    [editStore, layoutStore],
  );

  const isWorking = useCallback(
    () => isBusy(editStore.getState()) || layoutStore.getState().isCommitting,
    [editStore, layoutStore],
  );

  const requestRefresh = useCallback(() => {
    if (isWorking()) {
      return;
    }
    if (!hasUnsavedWork()) {
      refreshTemplate();
      return;
    }
    setConfirmation((current) => current ?? { kind: 'refresh', proceed: refreshTemplate });
  }, [hasUnsavedWork, isWorking, refreshTemplate]);

  const resolveConfirmation = useCallback(
    (accepted: boolean) => {
      const current = confirmation;
      setConfirmation(null);
      if (!accepted || current === null) {
        return;
      }
      editStore.discard();
      layoutStore.cancel();
      void discardDraft(storePort, sheet.id);
      void discardLayoutDraft(storePort, sheet.id);
      current.proceed();
    },
    [confirmation, editStore, layoutStore, storePort, sheet.id],
  );

  useEffect(
    () =>
      registerLeaveGuard({
        intercepts: () => hasUnsavedWork() || isWorking(),
        requestLeave: (proceed) => {
          if (isWorking()) {
            return;
          }
          setConfirmation((current) => current ?? { kind: 'leave', proceed });
        },
      }),
    [hasUnsavedWork, isWorking],
  );

  useEffect(() => {
    void editStore.restore();
    return startDraftCadence(storePort, () =>
      isPending(editStore.getState())
        ? { sheetId: sheet.id, delta: editStore.getPendingDelta() }
        : null,
    );
  }, [editStore, storePort, sheet.id]);

  useEffect(() => {
    void layoutStore.restore();
  }, [layoutStore]);

  useEffect(() => {
    if (arranging && focusArrangeHint.current) {
      focusArrangeHint.current = false;
      arrangeHintRef.current?.focus();
    }
    if (!arranging && wasArranging.current) {
      arrangeToggleRef.current?.focus();
    }
    wasArranging.current = arranging;
  }, [arranging]);

  useSheetShortcuts({
    onSave: () => {
      if (!layoutStore.getState().staged) {
        void editStore.save();
      }
    },
    onRefresh: requestRefresh,
  });

  return (
    <>
      <header className="lg-sheet__header">
        <div className="lg-sheet__heading">
          <div className="lg-sheet__heading-text">
            <h1 className="lg-sheet__title" data-testid="sheet-name">
              {sheet.name}
            </h1>
            <p className="lg-sheet__template" data-testid="sheet-eyebrow">
              {eyebrow}
            </p>
          </div>
          {sections.length > 0 && (
            <div className="lg-sheet__header-actions">
              {hasCalculated && (
                <HelpToggle
                  open={helpOpen}
                  buttonRef={helpToggleRef}
                  onToggle={() => (helpOpen ? closeHelp() : setHelpOpen(true))}
                />
              )}
              <ArrangeToggle
                store={layoutStore}
                buttonRef={arrangeToggleRef}
                onStart={startArranging}
              />
            </div>
          )}
        </div>
        {arranging && (
          <p
            className="lg-sheet__hint"
            tabIndex={-1}
            ref={arrangeHintRef}
            data-testid="arrange-hint"
          >
            {message('arrange.hint')}
          </p>
        )}
        {hasCalculated && helpOpen && (
          <HelpPanel
            onDismiss={() => {
              closeHelp();
              helpToggleRef.current?.focus();
            }}
          />
        )}
      </header>
      {sections.length === 0 ? (
        <p className="lg-sheet__message" data-testid="sheet-empty-fields">
          {message('sheet.emptyFields')}
        </p>
      ) : (
        <ArrangeProvider store={layoutStore}>
          <SectionIndex
            sheetName={sheet.name}
            onTop={scrollToTop}
            sections={view.links}
            onlyPinned={view.state.onlyPinned}
            filterDisabled={arranging}
            pinnedCount={arrangement.pinned.length}
            onSetOnlyPinned={view.store.setOnlyPinned}
            onJump={jumpToSection}
            onEditPins={startArranging}
            trailing={
              mainSections.length > 1 && !arranging && !view.filterOn ? (
                <CollapseAllButton view={view} sectionCount={mainSections.length} />
              ) : undefined
            }
            epoch={arranging ? 'arrange' : 'play'}
          />
          {view.filterOn && arrangement.pinned.length === 0 ? (
            <PinnedEmpty onEditPins={startArranging} />
          ) : (
            <DiceProvider value={diceValue}>
              {mainSections.length > 0 && (
                <SheetForm
                  fields={mainFields}
                  controlBuilder={controlBuilder}
                  exclusiveControlBuilder={exclusiveControlBuilder}
                  dependentBuilder={dependentBuilder}
                  rowMarkBuilder={rowMarkBuilder}
                  rowActionsBuilder={arranging ? undefined : renderDiceActions}
                  sectionBadgeBuilder={sectionBadgeBuilder}
                  collapsedSectionKeys={view.collapsible ? view.state.collapsed : undefined}
                  onToggleSection={view.collapsible ? view.store.toggle : undefined}
                  onlyPinned={view.filterOn}
                  arrangement={arrangement}
                  frames={arranging ? ARRANGE_FRAMES : DICE_FRAMES}
                />
              )}
              {hasSettings && !arranging && !view.filterOn && (
                <TemplateSettingsArea
                  open={view.state.settingsOpen}
                  onToggle={view.store.toggleSettings}
                >
                  <SheetForm
                    fields={settingsFields}
                    controlBuilder={controlBuilder}
                    exclusiveControlBuilder={exclusiveControlBuilder}
                    dependentBuilder={dependentBuilder}
                    rowMarkBuilder={rowMarkBuilder}
                    rowActionsBuilder={renderDiceActions}
                    sectionBadgeBuilder={sectionBadgeBuilder}
                    frames={DICE_FRAMES}
                  />
                </TemplateSettingsArea>
              )}
            </DiceProvider>
          )}
        </ArrangeProvider>
      )}
      {arranging ? (
        <ArrangeFooter store={layoutStore} />
      ) : (
        <SheetFooter store={editStore} reveal={view.reveal} />
      )}
      {confirmation !== null && (
        <SheetConfirmation
          kind={confirmation.kind}
          onCancel={() => resolveConfirmation(false)}
          onConfirm={() => resolveConfirmation(true)}
        />
      )}
    </>
  );
}

function fieldsWhere(
  fields: Record<string, FieldNode>,
  keep: (key: string) => boolean,
): Record<string, FieldNode> {
  return Object.fromEntries(Object.entries(fields).filter(([key]) => keep(key)));
}

function SheetBody({ result }: { readonly result: SheetOpenResult }) {
  switch (result.kind) {
    case 'loading':
      return <p className="lg-sheet__message">{message('sheet.openLoading')}</p>;
    case 'notFound':
      return (
        <p className="lg-sheet__message" data-testid="sheet-missing">
          {result.message}
        </p>
      );
    case 'failure':
      return <RetryAction copy={result.message} onRetry={result.retry} />;
    case 'templateLoading':
      return <p className="lg-sheet__message">{message('sheet.templateLoading')}</p>;
    case 'templateFailure':
      return <RetryAction copy={result.message} onRetry={result.retry} />;
    case 'loaded':
      return <LoadedSheet key={result.sheet.id} sheet={result.sheet} template={result.template} />;
  }
}

export function SheetScreen() {
  const params = useParams<{ id: string }>();
  const result = useSheetOpen(params.id ?? '');

  return (
    <section
      className="lg-sheet lg-sheet--editor"
      aria-label={message('sheet.headerTitle')}
      data-testid="sheet-screen"
    >
      <SheetBody result={result} />
    </section>
  );
}
