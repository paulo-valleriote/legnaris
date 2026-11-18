import { useId, useMemo, useSyncExternalStore } from 'react';
import type { ReactNode } from 'react';
import { ChevronRightIcon } from '../ui/icons';
import { flushSync } from 'react-dom';
import { arrangementView } from '../engine/layout';
import type { Arrangement } from '../engine/layout';
import type { SheetSection } from '../engine/field_grouping';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';
import { EmptyState } from '../ui/state';
import { useViewStorage } from '../app/providers';
import type { RevealView } from './reveal_row';
import type { SectionLink } from './section_index';
import { markedRowCounts } from './preview_summary';
import type { PreviewMarks } from './sheet_edit';
import type { ViewState, ViewStore } from './view_store';
import { createViewStore } from './view_store';

export interface SheetView {
  readonly store: ViewStore;
  readonly state: ViewState;
  readonly filterOn: boolean;
  readonly collapsible: boolean;
  readonly links: readonly SectionLink[];
  readonly reveal: RevealView;
}

export interface SheetViewInputs {
  readonly sheetId: string;
  readonly sections: readonly SheetSection[];
  readonly arrangement: Arrangement;
  readonly arranging: boolean;
  readonly marks?: PreviewMarks;
}

export function useSheetView({
  sheetId,
  sections,
  arrangement,
  arranging,
  marks,
}: SheetViewInputs): SheetView {
  const storage = useViewStorage();
  const store = useMemo(
    () =>
      createViewStore({ sheetId, sectionKeys: sections.map((section) => section.key), storage }),
    [sheetId, sections, storage],
  );
  const state = useSyncExternalStore(store.subscribe, store.getState);
  const filterOn = state.onlyPinned && !arranging;
  const collapsible = !filterOn && !arranging;

  const links = useMemo(() => {
    const drawn = arrangementView(sections, arrangement);
    const changed = markedRowCounts(drawn, marks);
    return drawn
      .filter((entry) => !filterOn || entry.rows.some((row) => row.pinned))
      .map((entry) => ({
        key: entry.section.key,
        label: entry.section.label,
        changed: changed.get(entry.section.key) ?? 0,
      }));
  }, [sections, arrangement, filterOn, marks]);

  const reveal = useMemo<RevealView>(
    () => ({
      reveal: (sectionKey) => {
        flushSync(() => store.reveal(sectionKey));
      },
      showAll: () => {
        flushSync(() => store.setOnlyPinned(false));
      },
    }),
    [store],
  );

  return { store, state, filterOn, collapsible, links, reveal };
}

interface CollapseAllProps {
  readonly view: SheetView;
  readonly sectionCount: number;
}

export function CollapseAllButton({ view, sectionCount }: CollapseAllProps) {
  const allCollapsed = view.state.collapsed.length >= sectionCount;
  return (
    <Button
      variant="ghost"
      size="sm"
      label={allCollapsed ? message('sheet.expandAll') : message('sheet.collapseAll')}
      onClick={() => (allCollapsed ? view.store.expandAll() : view.store.collapseAll())}
    />
  );
}

interface PinnedEmptyProps {
  readonly onEditPins: () => void;
}

export function PinnedEmpty({ onEditPins }: PinnedEmptyProps) {
  return (
    <div data-testid="pinned-empty">
      <EmptyState
        message={message('sheet.pinnedEmpty')}
        action={
          <Button
            variant="outline"
            size="sm"
            label={message('sheet.editPins')}
            onClick={onEditPins}
          />
        }
      />
    </div>
  );
}

interface TemplateSettingsAreaProps {
  readonly open: boolean;
  readonly onToggle: () => void;
  readonly children: ReactNode;
}

export function TemplateSettingsArea({ open, onToggle, children }: TemplateSettingsAreaProps) {
  const bodyId = useId();
  return (
    <div
      className={open ? 'lg-settings' : 'lg-settings lg-settings--closed'}
      data-testid="template-settings"
    >
      <h2 className="lg-settings__title">
        <button
          type="button"
          className="lg-settings__toggle"
          aria-expanded={open}
          aria-controls={bodyId}
          onClick={onToggle}
        >
          <span className="lg-settings__label">{message('sheet.templateSettings')}</span>
          <span className="lg-settings__hint">{message('sheet.templateSettingsHint')}</span>
          <ChevronRightIcon className="lg-settings__chevron" />
        </button>
      </h2>
      <div className="lg-settings__body" id={bodyId} hidden={!open}>
        {children}
      </div>
    </div>
  );
}
