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
 * through one modal that never stacks.
 */

import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
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
import { SheetFieldControl } from '../engine/field_control';
import type { SheetExclusiveGroup, SheetField } from '../engine/field_grouping';
import { buildSections } from '../engine/field_grouping';
import type { SheetMark } from '../engine/sheet_mark';
import { markLabel, SheetForm } from '../engine/sheet_form';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';
import { useApiClient, useSheetStore } from '../app/providers';
import { templateDetailKey } from '../catalog/queries';
import { startDraftCadence } from '../library/draft_backup';
import { discardDraft } from '../library/sheet_library';
import { registerLeaveGuard } from '../shell/leave_guard';
import { useSheetShortcuts } from '../shell/shortcut_registry';
import type { PreviewMarks, SheetEditState, SheetEditStore } from './sheet_edit';
import {
  canPreview,
  canSave,
  createSheetEditStore,
  isBusy,
  isPending,
  markForPaths,
} from './sheet_edit';
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
}

/** One editable control, subscribed to its own path so an unrelated edit never re-renders it (AD-001). */
function ConnectedField({ store, field }: ConnectedFieldProps) {
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
      onChanged={(next) => store.setPath(field.path, next)}
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
}

/** The fixed footer: the dirty indicator, the Preview and Save actions and their inline results (WSC-12, WSC-16, WSC-17, WSC-21, WSC-25). */
function SheetFooter({ store }: SheetFooterProps) {
  const state = useEditState(store);
  const [savedRevision, setSavedRevision] = useState<number | undefined>(undefined);
  const pending = isPending(state);
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

  return (
    <footer className="lg-sheet__footer">
      <div className="lg-sheet__status">
        {pending && (
          <p className="lg-sheet__dirty" data-testid="sheet-pending">
            {message('sheet.pendingEdits')}
          </p>
        )}
        {state.previewError !== undefined && (
          <p className="lg-sheet__error" role="alert" data-testid="sheet-preview-error">
            {state.previewError}
          </p>
        )}
        {state.saveError !== undefined && (
          <p className="lg-sheet__error" role="alert" data-testid="sheet-save-error">
            {state.saveError}
          </p>
        )}
      </div>
      <div className="lg-sheet__actions">
        <Button
          variant="secondary"
          size="sm"
          disabled={!previewable}
          loading={state.isPreviewing}
          label={state.isPreviewing ? message('sheet.previewing') : message('sheet.preview')}
          onClick={() => {
            void store.preview();
          }}
        />
        <Button
          variant="primary"
          size="sm"
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
  const editStore = useMemo(
    () => createSheetEditStore({ template, sheet }, { client, store: storePort }),
    [client, storePort, sheet, template],
  );
  const [confirmation, setConfirmation] = useState<PendingConfirmation | null>(null);
  const sections = buildSections(template.fields);
  const marks = useSyncExternalStore<PreviewMarks | undefined>(
    editStore.subscribeMarks,
    () => editStore.getState().previewMarks,
  );

  const controlBuilder = useCallback(
    (field: SheetField) => <ConnectedField store={editStore} field={field} />,
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
        <span className="lg-field-row__mark">{markLabel(mark)}</span>
      );
    },
    [marks],
  );

  const refreshTemplate = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: templateDetailKey(sheet.templateId) });
  }, [queryClient, sheet.templateId]);

  const requestRefresh = useCallback(() => {
    const state = editStore.getState();
    if (isBusy(state)) {
      return;
    }
    if (!isPending(state)) {
      refreshTemplate();
      return;
    }
    setConfirmation((current) => current ?? { kind: 'refresh', proceed: refreshTemplate });
  }, [editStore, refreshTemplate]);

  const resolveConfirmation = useCallback(
    (accepted: boolean) => {
      const current = confirmation;
      setConfirmation(null);
      if (!accepted || current === null) {
        return;
      }
      editStore.discard();
      void discardDraft(storePort, sheet.id);
      current.proceed();
    },
    [confirmation, editStore, storePort, sheet.id],
  );

  useEffect(
    () =>
      registerLeaveGuard({
        intercepts: () => {
          const state = editStore.getState();
          return isPending(state) || isBusy(state);
        },
        requestLeave: (proceed) => {
          if (isBusy(editStore.getState())) {
            return;
          }
          setConfirmation((current) => current ?? { kind: 'leave', proceed });
        },
      }),
    [editStore],
  );

  useEffect(() => {
    void editStore.restore();
    return startDraftCadence(storePort, () =>
      isPending(editStore.getState())
        ? { sheetId: sheet.id, delta: editStore.getPendingDelta() }
        : null,
    );
  }, [editStore, storePort, sheet.id]);

  useSheetShortcuts({
    onSave: () => {
      void editStore.save();
    },
    onRefresh: requestRefresh,
  });

  return (
    <>
      <header className="lg-sheet__header">
        <p className="lg-sheet__eyebrow">{message('sheet.headerTitle')}</p>
        <h1 className="lg-sheet__title" data-testid="sheet-name">
          {sheet.name}
        </h1>
      </header>
      {sections.length === 0 ? (
        <p className="lg-sheet__message" data-testid="sheet-empty-fields">
          {message('sheet.emptyFields')}
        </p>
      ) : (
        <SheetForm
          fields={template.fields}
          controlBuilder={controlBuilder}
          exclusiveControlBuilder={exclusiveControlBuilder}
          rowMarkBuilder={rowMarkBuilder}
        />
      )}
      <SheetFooter store={editStore} />
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
      return <LoadedSheet sheet={result.sheet} template={result.template} />;
  }
}

export function SheetScreen() {
  const params = useParams<{ id: string }>();
  const result = useSheetOpen(params.id ?? '');

  return (
    <section
      className="lg-sheet"
      aria-label={message('sheet.headerTitle')}
      data-testid="sheet-screen"
    >
      <SheetBody result={result} />
    </section>
  );
}
