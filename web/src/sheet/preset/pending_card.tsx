import { useCallback, useMemo, useSyncExternalStore } from 'react';
import { fieldDisplayName, lastSegment } from '../../engine/field_grouping';
import { message, messageWith } from '../../l10n/strings';
import { Button } from '../../ui/button';
import { areValuesPending, canPreview, canSave } from '../sheet_edit';
import { pendingChanges } from './pending_changes';
import { usePresetContext } from './preset_context';

export function PendingChangesCard() {
  const { store, fields } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const changes = useMemo(
    () =>
      pendingChanges(state.defaults, state.data, state.editedPaths).map((change) => ({
        ...change,
        label: fieldDisplayName(lastSegment(change.path), fields.get(change.path)?.schema),
      })),
    [state.defaults, state.data, state.editedPaths, fields],
  );

  if (!areValuesPending(state)) {
    return null;
  }

  const busy = state.isSaving || state.isPreviewing;

  return (
    <section
      className="lg-pending-card"
      role="status"
      aria-live="polite"
      data-testid="sheet-pending-card"
    >
      <div className="lg-pending-card__summary">
        <ul className="lg-pending-card__changes" data-testid="sheet-pending-changes">
          {changes.map((change) => (
            <li
              className="lg-pending-card__change"
              key={change.path}
              aria-label={messageWith('sheet.pending.change', change)}
            >
              <span className="lg-pending-card__label">{change.label}</span>{' '}
              <span className="lg-pending-card__before">{change.before}</span>{' '}
              <span className="lg-pending-card__arrow" aria-hidden="true">
                {message('sheet.pending.arrow')}
              </span>{' '}
              <span className="lg-pending-card__after">{change.after}</span>
            </li>
          ))}
        </ul>
        <p className="lg-pending-card__note">{message('sheet.pending.notSavedYet')}</p>
      </div>
      <p className="lg-pending-card__sync" data-testid="sheet-pending-sync" aria-hidden="true">
        {message('sheet.pending.tableSync')}
      </p>
      <div className="lg-pending-card__actions">
        <Button
          variant="ghost"
          size="sm"
          disabled={busy}
          label={message('sheet.pending.discard')}
          onClick={() => store.discard()}
        />
        <Button
          variant="secondary"
          size="sm"
          disabled={!canPreview(state)}
          loading={state.isPreviewing}
          label={
            state.isPreviewing ? message('sheet.previewing') : message('sheet.pending.preview')
          }
          onClick={() => {
            void store.preview();
          }}
        />
        <Button
          variant="primary"
          size="sm"
          disabled={!canSave(state)}
          loading={state.isSaving}
          label={state.isSaving ? message('sheet.saving') : message('sheet.pending.save')}
          onClick={() => {
            void store.save();
          }}
        />
      </div>
      {state.saveError !== undefined && (
        <p className="lg-pending-card__error" data-testid="sheet-pending-error">
          {state.saveError}
        </p>
      )}
      {state.previewError !== undefined && (
        <p className="lg-pending-card__error" data-testid="sheet-pending-preview-error">
          {state.previewError}
        </p>
      )}
    </section>
  );
}
