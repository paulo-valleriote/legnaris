import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import type { RefObject } from 'react';
import { fieldDisplayName, lastSegment } from '../../engine/field_grouping';
import { message, messageWith } from '../../l10n/strings';
import { Button } from '../../ui/button';
import { areValuesPending, canPreview, canSave } from '../sheet_edit';
import { pendingChanges } from './pending_changes';
import { usePresetContext } from './preset_context';

export const PENDING_ANNOUNCE_MS = 600;

const SPACE_PROPERTY = '--lg-pending-card-space';

function useDebouncedAnnouncement(summary: string): string {
  const [announced, setAnnounced] = useState('');

  useEffect(() => {
    if (summary === '') {
      return undefined;
    }
    const timer = setTimeout(() => setAnnounced(summary), PENDING_ANNOUNCE_MS);
    return () => clearTimeout(timer);
  }, [summary]);

  return summary !== '' && announced === summary ? announced : '';
}

function useReservedBottomSpace(card: RefObject<HTMLElement | null>, active: boolean): void {
  useEffect(() => {
    if (!active) {
      return undefined;
    }
    const element = card.current;
    if (element === null) {
      return undefined;
    }
    const preset = element.closest('.lg-preset');
    if (!(preset instanceof HTMLElement)) {
      return undefined;
    }
    const apply = () => {
      preset.style.setProperty(SPACE_PROPERTY, `${element.getBoundingClientRect().height}px`);
    };
    apply();
    if (typeof ResizeObserver === 'undefined') {
      return () => {
        preset.style.removeProperty(SPACE_PROPERTY);
      };
    }
    const observer = new ResizeObserver(apply);
    observer.observe(element);
    return () => {
      observer.disconnect();
      preset.style.removeProperty(SPACE_PROPERTY);
    };
  }, [card, active]);
}

export function PendingChangesCard() {
  const { store, fields } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const card = useRef<HTMLElement>(null);
  const changes = useMemo(
    () =>
      pendingChanges(state.defaults, state.data, state.editedPaths).map((change) => ({
        ...change,
        label: fieldDisplayName(lastSegment(change.path), fields.get(change.path)?.schema),
      })),
    [state.defaults, state.data, state.editedPaths, fields],
  );
  const summary = changes
    .map((change) => messageWith('sheet.pending.change', change))
    .join(message('sheet.pending.separator'));
  const pending = areValuesPending(state);
  const announcement = useDebouncedAnnouncement(summary);
  useReservedBottomSpace(card, pending);

  if (!pending) {
    return null;
  }

  const busy = state.isSaving || state.isPreviewing;

  return (
    <section
      ref={card}
      className="lg-pending-card"
      aria-label={message('sheet.pendingEdits')}
      data-testid="sheet-pending-card"
    >
      <p
        className="lg-visually-hidden"
        aria-live="polite"
        aria-atomic="true"
        data-testid="sheet-pending-announcement"
      >
        {announcement === ''
          ? ''
          : messageWith('sheet.pending.announce', { summary: announcement })}
      </p>
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
      <p className="lg-pending-card__sync" data-testid="sheet-pending-sync">
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
