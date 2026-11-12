/**
 * The sheet screen's navigation and view wiring: the per-viewer view store, what it
 * means for the strip and the form, and the controls that belong to it.
 *
 * Collapsing and filtering are view state, remembered per viewer and per sheet. They send no
 * request, never count as unsaved work and never reach the leave guard. While the person
 * arranges, the pinned filter reads as off and Sections draw open; while they pin, the filter
 * reads as off too (every Row must show its star) and Sections stay collapsible. In both the view
 * state is left untouched, so leaving the mode returns to exactly what the person left.
 */

import { useId, useMemo, useSyncExternalStore } from 'react';
import type { ReactNode } from 'react';
import { SectionChevron } from '../engine/section_chevron';
import { flushSync } from 'react-dom';
import { arrangementView } from '../engine/layout';
import type { Arrangement } from '../engine/layout';
import type { SheetSection } from '../engine/field_grouping';
import { message } from '../l10n/strings';
import { useViewStorage } from '../app/providers';
import type { RevealView } from './reveal_row';
import type { SectionLink } from './section_index';
import { markedRowCounts } from './preview_summary';
import type { PreviewMarks } from './sheet_edit';
import type { ViewState, ViewStore } from './view_store';
import { createViewStore } from './view_store';

/** What the screen needs from the view state, already resolved against Arrange mode. */
export interface SheetView {
  readonly store: ViewStore;
  readonly state: ViewState;
  /** True when only pinned Rows are drawn: the filter is on and the person is not arranging. */
  readonly filterOn: boolean;
  /** True when Section titles act as disclosure controls. */
  readonly collapsible: boolean;
  /** The Sections the strip links to, in drawn order. */
  readonly links: readonly SectionLink[];
  /** The view controls a reveal drives; they render synchronously. */
  readonly reveal: RevealView;
}

/** What {@link useSheetView} resolves the view state against. */
export interface SheetViewInputs {
  readonly sheetId: string;
  readonly sections: readonly SheetSection[];
  /** The arrangement being drawn. */
  readonly arrangement: Arrangement;
  readonly arranging: boolean;
  /** The last Preview's marks, counted per Section for the links' dots. */
  readonly marks?: PreviewMarks;
}

/** Builds the view store for one sheet and resolves it against the arrangement being drawn. */
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

/**
 * One text button that collapses every Section, or expands them all when every one is
 * collapsed. The screen does not draw it while the pinned filter is on or Arrange mode is.
 */
export function CollapseAllButton({ view, sectionCount }: CollapseAllProps) {
  const allCollapsed = view.state.collapsed.length >= sectionCount;
  return (
    <button
      type="button"
      className="lg-button lg-button--ghost lg-button--sm"
      onClick={() => (allCollapsed ? view.store.expandAll() : view.store.collapseAll())}
    >
      {allCollapsed ? message('sheet.expandAll') : message('sheet.collapseAll')}
    </button>
  );
}

interface PinnedEmptyProps {
  readonly onEditPins: () => void;
}

/** What the filter shows when nothing is pinned: one line of plain text and the way to pin. */
export function PinnedEmpty({ onEditPins }: PinnedEmptyProps) {
  return (
    <div className="lg-sheet__pinned-empty" data-testid="pinned-empty">
      <p className="lg-sheet__message">{message('sheet.pinnedEmpty')}</p>
      <button
        type="button"
        className="lg-button lg-button--outline lg-button--sm"
        onClick={onEditPins}
      >
        {message('sheet.editPins')}
      </button>
    </div>
  );
}

interface TemplateSettingsAreaProps {
  readonly open: boolean;
  readonly onToggle: () => void;
  readonly children: ReactNode;
}

/**
 * The one disclosure at the end of the sheet that holds the Template's own settings (its rules
 * and switches). Closed until the person opens it; its content stays mounted so edits inside
 * survive closing it.
 */
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
          <SectionChevron className="lg-settings__chevron" />
        </button>
      </h2>
      <div className="lg-settings__body" id={bodyId} hidden={!open}>
        {children}
      </div>
    </div>
  );
}
