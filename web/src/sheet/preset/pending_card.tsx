import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react';
import type { ReactNode, RefObject } from 'react';
import type { TemplateDetail } from '../../api/models/template';
import { fieldDisplayName, lastSegment } from '../../engine/field_grouping';
import type { SheetField } from '../../engine/field_grouping';
import { listInfos, orderedRecordIDs, RECORD_POSITION_FIELD } from '../../engine/row_identity';
import { message, messageWith } from '../../l10n/strings';
import { Button } from '../../ui/button';
import { ChevronDownIcon } from '../../ui/icons';
import { FLOATING_FLOOR_ATTRIBUTE } from '../dice_panel';
import { canPreview, canSave, isDicePending, isPending } from '../sheet_edit';
import { pendingChanges } from './pending_changes';
import { usePresetContext } from './preset_context';

export const PENDING_ANNOUNCE_MS = 600;

export const PENDING_CARD_SPACE_CLASS = 'lg-preset--pending-card';
export const PENDING_CARD_SPACE_PROPERTY = '--lg-pending-card-space';

export const PENDING_CARD_SLACK = 12;

export interface PendingCardOverride {
  readonly note: string;
  readonly action: ReactNode;
}

const PendingCardOverrideContext = createContext<PendingCardOverride | undefined>(undefined);

export const PendingCardOverrideProvider = PendingCardOverrideContext.Provider;

export function reservedBottomSpace(height: number, inset: number): number {
  const card = Number.isFinite(height) ? Math.max(0, height) : 0;
  const floor = Number.isFinite(inset) ? Math.max(0, inset) : 0;
  return card + floor + PENDING_CARD_SLACK;
}

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
  useLayoutEffect(() => {
    const element = card.current;
    const preset = element?.closest('.lg-preset');
    if (!active || element === null || !(preset instanceof HTMLElement)) {
      return undefined;
    }
    const apply = () => {
      const height = element.getBoundingClientRect().height;
      const inset = Number.parseFloat(window.getComputedStyle(element).bottom);
      preset.style.setProperty(
        PENDING_CARD_SPACE_PROPERTY,
        `${reservedBottomSpace(height, inset)}px`,
      );
    };
    const release = () => {
      preset.classList.remove(PENDING_CARD_SPACE_CLASS);
      preset.style.removeProperty(PENDING_CARD_SPACE_PROPERTY);
    };
    preset.classList.add(PENDING_CARD_SPACE_CLASS);
    apply();
    window.addEventListener('resize', apply);
    if (typeof ResizeObserver === 'undefined') {
      return () => {
        window.removeEventListener('resize', apply);
        release();
      };
    }
    const observer = new ResizeObserver(apply);
    observer.observe(element);
    return () => {
      window.removeEventListener('resize', apply);
      observer.disconnect();
      release();
    };
  }, [card, active]);
}

function changeLabel(
  path: string,
  templateFields: TemplateDetail['fields'],
  fields: ReadonlyMap<string, SheetField>,
  data: Record<string, unknown>,
): string {
  const list = listInfos(templateFields).find((entry) => path.startsWith(`${entry.path}.`));
  if (list === undefined) {
    return fieldDisplayName(lastSegment(path), fields.get(path)?.schema);
  }
  const [recordID = '', field] = path.slice(list.path.length + 1).split('.');
  const position = orderedRecordIDs(templateFields, list.path, data).indexOf(recordID) + 1;
  if (position === 0) {
    return messageWith('sheet.pending.recordRemoved', { list: list.label });
  }
  const record = messageWith('record.rowLabel', { list: list.label, position });
  if (field === undefined) {
    return record;
  }
  return messageWith('sheet.pending.recordField', {
    record,
    field:
      field === RECORD_POSITION_FIELD
        ? message('sheet.pending.position')
        : fieldDisplayName(field, list.shape[field]),
  });
}

export function PendingChangesCard() {
  const { store, fields, template } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const card = useRef<HTMLElement>(null);
  const override = useContext(PendingCardOverrideContext);
  const changes = useMemo(
    () =>
      pendingChanges(state.defaults, state.data, state.editedPaths).map((change) => ({
        ...change,
        label: changeLabel(change.path, template.fields, fields, state.data),
      })),
    [state.defaults, state.data, state.editedPaths, fields, template],
  );
  const dicePending = isDicePending(state);
  const summary = [
    ...changes.map((change) => messageWith('sheet.pending.change', change)),
    ...(dicePending ? [message('sheet.pending.dice')] : []),
  ].join(message('sheet.pending.separator'));
  const pending = isPending(state);
  const announcement = useDebouncedAnnouncement(summary);
  const [expanded, setExpanded] = useState(false);
  const listId = useId();
  useReservedBottomSpace(card, pending);

  if (!pending) {
    return null;
  }

  const busy = state.isSaving || state.isPreviewing;
  const count = changes.length + (dicePending ? 1 : 0);

  return (
    <section
      ref={card}
      className="lg-pending-card"
      aria-label={message('sheet.pendingEdits')}
      data-testid="sheet-pending-card"
      {...{ [FLOATING_FLOOR_ATTRIBUTE]: '' }}
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
      <ul
        id={listId}
        className="lg-pending-card__changes"
        data-testid="sheet-pending-changes"
        aria-label={message('sheet.pendingEdits')}
        hidden={!expanded}
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
        tabIndex={0}
      >
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
        {dicePending && (
          <li className="lg-pending-card__change">
            <span className="lg-pending-card__label">{message('sheet.pending.dice')}</span>
          </li>
        )}
      </ul>
      <div className="lg-pending-card__row">
        <button
          type="button"
          className="lg-pending-card__toggle"
          aria-expanded={expanded}
          aria-controls={listId}
          onClick={() => setExpanded((open) => !open)}
        >
          <ChevronDownIcon className="lg-pending-card__chevron" />
          <span className="lg-pending-card__summary">
            <span className="lg-pending-card__count">
              {count === 1
                ? message('sheet.changeCountOne')
                : messageWith('sheet.changeCount', { count })}
            </span>
            <span className="lg-pending-card__note">
              {override?.note ?? message('sheet.pending.notSavedYet')}
            </span>
          </span>
        </button>
        <div className="lg-pending-card__actions">
          <Button
            variant="ghost"
            size="sm"
            disabled={busy}
            name={message('sheet.pending.discard')}
            onClick={() => store.discard()}
          >
            {message('sheet.pending.discardShort')}
          </Button>
          {override === undefined ? (
            <>
              <Button
                variant="ghost"
                size="sm"
                disabled={!canPreview(state)}
                loading={state.isPreviewing}
                name={
                  state.isPreviewing
                    ? message('sheet.previewing')
                    : message('sheet.pending.preview')
                }
                onClick={() => {
                  void store.preview();
                }}
              >
                {message('sheet.preview')}
              </Button>
              <Button
                variant="primary"
                size="sm"
                disabled={!canSave(state)}
                loading={state.isSaving}
                name={state.isSaving ? message('sheet.saving') : message('sheet.pending.save')}
                onClick={() => {
                  void store.save();
                }}
              >
                {message('sheet.save')}
              </Button>
            </>
          ) : (
            override.action
          )}
        </div>
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
