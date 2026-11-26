import type { FailureKind } from '../api/errors';
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import type { ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router-dom';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { Sheet } from '../api/models/sheet';
import { isLinkableSheetKind, valueAtPath } from '../api/models/sheet';
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
import type { FieldNode, ResetAction } from '../api/models/template';
import { buildSections, fieldDisplayName, lastSegment, rowKey } from '../engine/field_grouping';
import {
  isRowKey as isTemplateRowKey,
  listInfos,
  orderedRecordIDs,
  recordsByList,
} from '../engine/row_identity';
import type { SheetMark } from '../engine/sheet_mark';
import { markLabel, SheetForm } from '../engine/sheet_form';
import type { RecordListActions, RowActionsInfo } from '../engine/sheet_form';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { CheckIcon, PencilIcon } from '../ui/icons';
import { EmptyState, ErrorState, LoadingState } from '../ui/state';
import { TemplateName } from '../catalog/template_name';
import { useApiClient, useSheetStore } from '../app/providers';
import {
  templateDetailKey,
  PLAN_QUERY_KEY,
  seedCreatedSheet,
  updateSheetLayout,
} from '../catalog/queries';
import { startDraftCadence } from '../library/draft_backup';
import { cacheSheet, discardDraft, discardLayoutDraft } from '../library/sheet_library';
import { registerLeaveGuard, requestLeave } from '../shell/leave_guard';
import { useSheetShortcuts } from '../shell/shortcut_registry';
import type { PendingDiceKeys, PreviewMarks, SheetEditState, SheetEditStore } from './sheet_edit';
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
import {
  DICE_FRAMES,
  DiceProvider,
  DiceRowActions,
  usePendingDiceKeys,
  type DiceContextValue,
} from './dice_row';
import { FLOATING_FLOOR_ATTRIBUTE } from './dice_panel';
import { fieldLabelFromFields } from './dice_label';
import {
  checkExpression,
  checkSectionExpression,
  diceSectionRows,
  diceSchemaFromFields,
  templateDiceDefaults,
  templateDiceMarking,
} from './dice_view';
import type { LayoutState, LayoutStore } from './layout_store';
import { createLayoutStore, drawnArrangement, isArranging, isLayoutPending } from './layout_store';
import { footerStatus, recalculatedCount, recalculatedSummary } from './preview_summary';
import type { RevealView } from './reveal_row';
import { revealFirstRecalculated, revealSection, revealTop } from './reveal_row';
import { SectionIndex } from './section_index';
import { CollapseAllButton, PinnedEmpty, TemplateSettingsArea, useSheetView } from './sheet_view';
import { isTemplateSettingsKey } from './view_store';
import { PresetPlayView } from './preset/play_view';
import { PresetProvider } from './preset/preset_context';
import { presetFor } from './preset/registry';
import { HelpPanel, HelpToggle } from './sheet_help';
import type { SheetOpenResult } from './sheet_open';
import { useSheetOpen } from './sheet_open';
import { ResetActions, newResetRequestID } from './reset_actions';
import './sheet.css';

function errorClass(kind: FailureKind | undefined): string {
  return kind === 'offline' ? 'lg-sheet__error lg-sheet__error--offline' : 'lg-sheet__error';
}

function useEditState(store: SheetEditStore): SheetEditState {
  return useSyncExternalStore(store.subscribe, store.getState);
}

function rowMark(
  marks: PreviewMarks | undefined,
  pendingDice: PendingDiceKeys,
  paths: readonly string[],
  key: string,
): SheetMark | undefined {
  return markForPaths(marks, paths) ?? (pendingDice.row.has(key) ? 'modified' : undefined);
}

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
  readonly controlId?: string;
}

function ConnectedExclusive({ store, group, controlId }: ConnectedExclusiveProps) {
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
      controlId={controlId}
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
      {...{ [FLOATING_FLOOR_ATTRIBUTE]: '' }}
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

function useLayoutState(store: LayoutStore): LayoutState {
  return useSyncExternalStore(store.subscribe, store.getState);
}

interface ArrangeFooterProps {
  readonly store: LayoutStore;
}

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
  readonly onStart: () => void;
}

function ArrangeToggle({ store, onStart }: ArrangeToggleProps) {
  const state = useLayoutState(store);
  if (isArranging(state)) {
    return null;
  }
  return (
    <Button
      variant="ghost"
      size="sm"
      className="lg-sheet__arrange-toggle"
      label={message('arrange.start')}
      onClick={onStart}
    />
  );
}

interface PendingConfirmation {
  readonly kind: 'refresh' | 'leave';
  readonly proceed: () => void;
}

interface SheetConfirmationProps {
  readonly kind: 'refresh' | 'leave';
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}

function SheetConfirmation({ kind, onCancel, onConfirm }: SheetConfirmationProps) {
  const safeRef = useRef<HTMLButtonElement>(null);
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

  return (
    <Dialog
      open
      tone="danger"
      title={copy.title}
      description={copy.body}
      onClose={onCancel}
      initialFocusRef={safeRef}
      actions={
        <>
          <Button
            ref={safeRef}
            variant="secondary"
            size="md"
            label={message('sheet.cancel')}
            onClick={onCancel}
          />
          <Button variant="danger" size="md" label={copy.confirm} onClick={onConfirm} />
        </>
      }
    />
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
  const sheetDice = sheet.dice;
  const [committedLayout] = useState(sheet.layout);
  const editStore = useMemo(
    () =>
      createSheetEditStore(
        { template, sheet: { id: sheetId, data: sheetData, dice: sheetDice } },
        { client, store: storePort },
      ),
    [client, storePort, sheetId, sheetData, sheetDice, template],
  );
  const preset = useMemo(() => presetFor(template), [template]);
  const generic = preset === undefined;
  const [editing, setEditing] = useState(false);
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
  const diceLabels = useMemo(() => fieldLabelFromFields(template.fields), [template]);
  const diceSectionRowKeys = useMemo(() => diceSectionRows(template.fields), [template]);
  const diceCheck = useCallback<DiceCheck>(
    (target, key, expression) =>
      target === 'row'
        ? checkExpression(expression, diceSchema, key)
        : checkSectionExpression(expression, diceSchema, diceSectionRowKeys.get(key) ?? []),
    [diceSchema, diceSectionRowKeys],
  );
  const diceStore = useMemo<DiceStore>(
    () => createDiceStore({ edit: editStore, check: diceCheck }),
    [editStore, diceCheck],
  );
  const pendingDice = usePendingDiceKeys(diceStore);
  const diceValue = useMemo<DiceContextValue>(
    () => ({
      store: diceStore,
      schema: diceSchema,
      check: diceCheck,
      defaults: templateDiceDefaults(template.dice, template.fields),
      rowKeys: new Set(sections.flatMap((section) => section.rows.map(rowKey))),
      sectionRows: diceSectionRowKeys,
      isRowKey: (key) => isTemplateRowKey(template.fields, key),
      notD20: templateDiceMarking(template.dice, template.fields),
      labelForPath: diceLabels,
      savedValues: () => editStore.getState().defaults,
      draftValues: () => editStore.getPendingDelta(),
    }),
    [
      diceStore,
      diceSchema,
      diceSectionRowKeys,
      diceCheck,
      template,
      sections,
      editStore,
      diceLabels,
    ],
  );
  const eyebrow = template.name.trim() === '' ? message('sheet.headerTitle') : template.name;
  const notice = template.gameSystem?.notice;
  const templateLine = (
    <p className="lg-sheet__template" data-testid="sheet-eyebrow">
      <TemplateName name={eyebrow} notice={notice} />
    </p>
  );
  const presetValue = useMemo(
    () => ({ store: editStore, template, sheet, dice: diceValue }),
    [editStore, template, sheet, diceValue],
  );
  const layoutStore = useMemo(
    () =>
      createLayoutStore(
        {
          sheetId,
          sections: mainSections,
          layout: committedLayout,
          isRowKey: (key) => isTemplateRowKey(template.fields, key),
        },
        {
          client,
          store: storePort,
          onCommitted: (layout) => updateSheetLayout(queryClient, sheetId, layout),
        },
      ),
    [client, storePort, queryClient, sheetId, mainSections, committedLayout, template.fields],
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
  const focusHelpToggle = useCallback(() => {
    helpToggleRef.current?.focus();
  }, []);
  const closeHelp = useCallback(() => {
    setHelpOpen(false);
    view.store.markExplainerSeen();
  }, [view.store]);
  const hasRecordLists = useMemo(() => listInfos(template.fields).length > 0, [template]);
  const editData = useSyncExternalStore(editStore.subscribe, () => editStore.getState().data);
  const recordRows = useMemo(
    () => recordsByList(template.fields, editData),
    [template.fields, editData],
  );
  const [recordNotice, setRecordNotice] = useState('');
  const recordActions = useMemo<RecordListActions>(
    () => ({
      add: (listPath) => {
        const recordID = editStore.addRecord(listPath);
        if (recordID !== undefined) {
          const count = orderedRecordIDs(
            template.fields,
            listPath,
            editStore.getState().data,
          ).length;
          setRecordNotice(messageWith('record.added', { count }));
        }
        return recordID;
      },
      remove: (listPath, recordID) => {
        editStore.removeRecord(listPath, recordID);
        const count = orderedRecordIDs(template.fields, listPath, editStore.getState().data).length;
        setRecordNotice(messageWith('record.removed', { count }));
      },
      move: (listPath, recordID, direction) => {
        const position = editStore.moveRecord(listPath, recordID, direction);
        if (position !== undefined) {
          setRecordNotice(messageWith('record.moved', { position }));
        }
      },
    }),
    [editStore, template.fields],
  );
  const [confirmation, setConfirmation] = useState<PendingConfirmation | null>(null);
  const [duplicating, setDuplicating] = useState(false);
  const [duplicateError, setDuplicateError] = useState<string | undefined>(undefined);
  const navigate = useNavigate();
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
    (
      group: SheetExclusiveGroup,
      _selectedPath?: string,
      _onSelect?: (signal: string) => void,
      controlId?: string,
    ) => <ConnectedExclusive store={editStore} group={group} controlId={controlId} />,
    [editStore],
  );
  const rowMarkBuilder = useCallback(
    (paths: readonly string[], key: string) => {
      const mark = rowMark(marks, pendingDice, paths, key);
      return mark === undefined ? null : (
        <span className="lg-field-row__mark" data-mark={mark}>
          {markLabel(mark)}
        </span>
      );
    },
    [marks, pendingDice],
  );

  const renderDiceActions = useCallback((info: RowActionsInfo) => <DiceRowActions {...info} />, []);

  const sectionBadgeBuilder = useCallback(
    (rowPaths: readonly (readonly string[])[], rowKeys: readonly string[]) => {
      const count = rowPaths.filter(
        (paths, index) => rowMark(marks, pendingDice, paths, rowKeys[index] ?? '') !== undefined,
      ).length;
      return count === 0 ? null : (
        <span className="lg-field-row__mark">
          {messageWith('engine.sectionChanged', { count })}
        </span>
      );
    },
    [marks, pendingDice],
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

  const runDuplicate = useCallback(async () => {
    setDuplicating(true);
    setDuplicateError(undefined);
    try {
      const copy = await client.duplicateSheet(sheet.id);
      seedCreatedSheet(queryClient, copy);
      void queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
      void navigate(`/sheets/${encodeURIComponent(copy.id)}`);
    } catch (error) {
      setDuplicateError(
        failureKind(error) === 'planLimitReached'
          ? message('sheet.limitReached')
          : failureCopy(error),
      );
      setDuplicating(false);
    }
  }, [client, navigate, queryClient, sheet.id]);

  const requestDuplicate = useCallback(() => {
    if (duplicating || isWorking()) {
      return;
    }
    requestLeave(() => {
      void runDuplicate();
    });
  }, [duplicating, isWorking, runDuplicate]);

  const runReset = useCallback(
    (action: ResetAction, requestId: string) => client.resetSheet(sheet.id, action.id, requestId),
    [client, sheet.id],
  );

  const applyReset = useCallback(
    (_action: ResetAction, cascade: Record<string, unknown>) => {
      editStore.applyReset(cascade);
      const after = editStore.getState();
      void cacheSheet(storePort, { ...sheet, data: after.defaults, dice: after.savedDice });
    },
    [editStore, sheet, storePort],
  );

  const refreshSheet = useCallback(async () => {
    await editStore.refresh();
  }, [editStore]);

  const presetReset = useMemo(
    () => ({ run: runReset, onApplied: applyReset, refresh: refreshSheet }),
    [runReset, applyReset, refreshSheet],
  );

  const resetActions = template.actions ?? [];
  const showResetActions = generic && !arranging && resetActions.length > 0;

  useEffect(() => {
    void editStore.restore();
    return startDraftCadence(storePort, () =>
      isPending(editStore.getState())
        ? {
            sheetId: sheet.id,
            delta: editStore.getPendingDelta(),
            dice: editStore.getPendingDice(),
          }
        : null,
    );
  }, [editStore, storePort, sheet.id]);

  useEffect(() => {
    if (generic) {
      void layoutStore.restore();
    }
  }, [generic, layoutStore]);

  useEffect(() => {
    if (arranging && focusArrangeHint.current) {
      focusArrangeHint.current = false;
      arrangeHintRef.current?.focus();
    }
    if (!arranging && wasArranging.current) {
      document.querySelector<HTMLButtonElement>('.lg-sheet__arrange-toggle')?.focus();
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
    <PresetProvider {...presetValue} reset={presetReset} editing={editing}>
      <header className="lg-sheet__header">
        <div className="lg-sheet__heading">
          <div className="lg-sheet__heading-text">
            {preset !== undefined ? (
              <>
                <preset.Header />
                {notice === undefined ? null : templateLine}
              </>
            ) : (
              <>
                <h1 className="lg-sheet__title" data-testid="sheet-name">
                  {sheet.name}
                </h1>
                {templateLine}
              </>
            )}
          </div>
          {sections.length > 0 && (
            <div className="lg-sheet__header-actions" data-testid="sheet-header-actions">
              {isLinkableSheetKind(sheet.kind) && (
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={arranging}
                  loading={duplicating}
                  label={duplicating ? message('sheet.duplicating') : message('sheet.duplicate')}
                  onClick={requestDuplicate}
                />
              )}
              {!generic && (
                <Button
                  variant="ghost"
                  size="sm"
                  icon={editing ? <CheckIcon /> : <PencilIcon />}
                  label={message(editing ? 'sheet.editDone' : 'sheet.edit')}
                  onClick={() => setEditing((current) => !current)}
                />
              )}
              {hasCalculated && (
                <HelpToggle
                  open={helpOpen}
                  buttonRef={helpToggleRef}
                  onToggle={() => (helpOpen ? closeHelp() : setHelpOpen(true))}
                />
              )}
              {generic && <ArrangeToggle store={layoutStore} onStart={startArranging} />}
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
            boxed={!generic}
            onDismiss={() => {
              closeHelp();
              focusHelpToggle();
            }}
          />
        )}
      </header>
      {showResetActions && (
        <ResetActions
          actions={resetActions}
          store={editStore}
          requestId={newResetRequestID}
          run={runReset}
          onApplied={applyReset}
          refresh={refreshSheet}
        />
      )}
      {duplicateError !== undefined ? (
        <div data-testid="sheet-duplicate-error">
          <ErrorState message={duplicateError} />
        </div>
      ) : null}
      {hasRecordLists && (
        <p
          className="lg-visually-hidden"
          role="status"
          aria-live="polite"
          data-testid="record-live"
        >
          {recordNotice}
        </p>
      )}
      {sections.length === 0 ? (
        <div data-testid="sheet-empty-fields">
          <EmptyState message={message('sheet.emptyFields')} />
        </div>
      ) : preset !== undefined ? (
        <DiceProvider value={diceValue}>
          <PresetPlayView preset={preset} />
        </DiceProvider>
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
                  records={recordRows}
                  recordActions={arranging ? undefined : recordActions}
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
      ) : generic ? (
        <SheetFooter store={editStore} reveal={view.reveal} />
      ) : null}
      {confirmation !== null && (
        <SheetConfirmation
          kind={confirmation.kind}
          onCancel={() => resolveConfirmation(false)}
          onConfirm={() => resolveConfirmation(true)}
        />
      )}
    </PresetProvider>
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
      return <LoadingState label={message('sheet.openLoading')} />;
    case 'notFound':
      return (
        <div data-testid="sheet-missing">
          <ErrorState message={result.message} />
        </div>
      );
    case 'failure':
      return (
        <ErrorState
          message={result.message}
          onRetry={result.retry}
          retryLabel={message('common.retry')}
        />
      );
    case 'templateLoading':
      return <LoadingState label={message('sheet.templateLoading')} />;
    case 'templateFailure':
      return (
        <ErrorState
          message={result.message}
          onRetry={result.retry}
          retryLabel={message('common.retry')}
        />
      );
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
