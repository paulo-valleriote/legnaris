/**
 * The sheet screen (WSC-05 … WSC-20, WSC-32, WSC-35): reads the `:id` route
 * parameter, resolves the open situation and renders it. A loaded sheet shows
 * its name, mounts the F4 engine in builder mode over an edit store, and renders
 * a footer with the dirty indicator and the Save action; the loading, missing,
 * retryable and template situations each render their own mapped copy.
 *
 * The screen owns the per-field subscription through the engine's builder seams
 * (AD-001): a control reads its own path from the store and reports an edit back,
 * so the engine stays stateless.
 */

import { useCallback, useMemo, useState, useSyncExternalStore } from 'react';
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
import { SheetForm } from '../engine/sheet_form';
import { message } from '../l10n/strings';
import { useApiClient, useSheetStore } from '../app/providers';
import { useSheetShortcuts } from '../shell/shortcut_registry';
import type { SheetEditState, SheetEditStore } from './sheet_edit';
import { canSave, createSheetEditStore, isPending } from './sheet_edit';
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
      <button type="button" className="lg-button" onClick={onRetry}>
        {message('sheet.retryOpen')}
      </button>
    </div>
  );
}

/** Subscribes to the whole edit state; the footer and the marks read this (WSC-12, WSC-23). */
function useEditState(store: SheetEditStore): SheetEditState {
  return useSyncExternalStore(store.subscribe, store.getState);
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

  return (
    <SheetFieldControl
      path={field.path}
      schema={field.schema}
      value={value}
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

  return (
    <SheetExclusiveControl
      group={group}
      selectedPath={selected}
      onSelect={(signal) => {
        store.setExclusive(signal === EXCLUSIVE_NONE_VALUE ? null : signal, paths);
      }}
    />
  );
}

interface SheetFooterProps {
  readonly store: SheetEditStore;
}

/** The fixed footer: the dirty indicator, the Save action and its inline result (WSC-12, WSC-16, WSC-17). */
function SheetFooter({ store }: SheetFooterProps) {
  const state = useEditState(store);
  const [savedRevision, setSavedRevision] = useState<number | undefined>(undefined);
  const pending = isPending(state);
  const savable = canSave(state);
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
        {state.saveError !== undefined && (
          <p className="lg-sheet__error" role="alert" data-testid="sheet-save-error">
            {state.saveError}
          </p>
        )}
      </div>
      <button
        type="button"
        className="lg-button lg-button--primary"
        disabled={!savable}
        aria-busy={state.isSaving}
        onClick={() => {
          void runSave();
        }}
      >
        {state.isSaving
          ? message('sheet.saving')
          : showSaved
            ? message('sheet.saved')
            : message('sheet.save')}
      </button>
    </footer>
  );
}

interface LoadedSheetProps {
  readonly sheet: Sheet;
  readonly template: TemplateDetail;
}

function LoadedSheet({ sheet, template }: LoadedSheetProps) {
  const client = useApiClient();
  const storePort = useSheetStore();
  const editStore = useMemo(
    () => createSheetEditStore({ template, sheet }, { client, store: storePort }),
    [client, storePort, sheet, template],
  );
  const sections = buildSections(template.fields);

  const controlBuilder = useCallback(
    (field: SheetField) => <ConnectedField store={editStore} field={field} />,
    [editStore],
  );
  const exclusiveControlBuilder = useCallback(
    (group: SheetExclusiveGroup) => <ConnectedExclusive store={editStore} group={group} />,
    [editStore],
  );

  useSheetShortcuts({
    onSave: () => {
      void editStore.save();
    },
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
        />
      )}
      <SheetFooter store={editStore} />
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
