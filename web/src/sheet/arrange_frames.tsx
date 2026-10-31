/**
 * The Arrange-mode and pin-mode chrome (ADR 0001, ADR 0002): the structural frames the engine
 * renders Sections and Rows through.
 *
 * While the player arranges, only Sections move, by dragging a focusable handle on the Section
 * header (`@dnd-kit`: pointer, touch and keyboard — Space lifts, the arrows move, Space drops,
 * Escape cancels), and a live region announces every drag and every pin. A Row is drawn compact —
 * its label and a star toggle that pins it to the top of its Section — so no value is edited by
 * accident. Nothing is saved until Done.
 *
 * Outside Arrange mode no handle, no star and no drag listener exists.
 */

import type { ReactNode } from 'react';
import { createContext, useContext, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import type { Announcements, DragEndEvent, UniqueIdentifier } from '@dnd-kit/core';
import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  MouseSensor,
  TouchSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type {
  RootFrameProps,
  RowFrameProps,
  SectionFrameProps,
  SheetFormFrames,
} from '../engine/sheet_form';
import { moveSection, pinRow, unpinRow } from '../engine/arrange_ops';
import { PinStar } from '../engine/pin_star';
import { messageWith, message } from '../l10n/strings';
import type { LayoutStore } from './layout_store';

interface ArrangeApi {
  moveSectionTo(key: string, index: number): void;
  setPinned(key: string, label: string, pinned: boolean): void;
  readonly reducedMotion: boolean;
}

const ArrangeContext = createContext<ArrangeApi | undefined>(undefined);
const AnnounceContext = createContext<(text: string) => void>(() => undefined);

/**
 * The sheet's one polite announcer: speaks [text] through the same live region as the pins, so
 * the page never carries a second persistent live region. A no-op outside {@link ArrangeProvider}.
 */
export function useAnnounce(): (text: string) => void {
  return useContext(AnnounceContext);
}

function useArrange(): ArrangeApi {
  const api = useContext(ArrangeContext);
  if (api === undefined) {
    throw new Error('Arrange frames must render inside <ArrangeProvider>.');
  }
  return api;
}

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';

function subscribeReducedMotion(listener: () => void): () => void {
  if (typeof window.matchMedia !== 'function') {
    return () => undefined;
  }
  const query = window.matchMedia(REDUCED_MOTION_QUERY);
  query.addEventListener('change', listener);
  return () => query.removeEventListener('change', listener);
}

function readReducedMotion(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

/** True while the person asks for reduced motion; re-evaluated live. */
export function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(subscribeReducedMotion, readReducedMotion, () => false);
}

interface ArrangeProviderProps {
  readonly store: LayoutStore;
  readonly children: ReactNode;
}

/** Feeds the arrange frames from [store] and renders the polite live region that announces pins and drags. */
export function ArrangeProvider({ store, children }: ArrangeProviderProps) {
  const [announcement, setAnnouncement] = useState('');
  const reducedMotion = usePrefersReducedMotion();

  const api = useMemo<ArrangeApi>(
    () => ({
      moveSectionTo: (key, index) => {
        store.apply((staged) => moveSection(staged, key, index));
      },
      setPinned: (key, label, pinned) => {
        store.apply((staged) => (pinned ? pinRow(staged, key) : unpinRow(staged, key)));
        setAnnouncement(messageWith(pinned ? 'arrange.pinned' : 'arrange.unpinned', { label }));
      },
      reducedMotion,
    }),
    [store, reducedMotion],
  );

  return (
    <ArrangeContext.Provider value={api}>
      <AnnounceContext.Provider value={setAnnouncement}>{children}</AnnounceContext.Provider>
      <p className="lg-visually-hidden" role="status" aria-live="polite" data-testid="arrange-live">
        {announcement}
      </p>
    </ArrangeContext.Provider>
  );
}

interface DragData {
  readonly key: string;
  readonly label: string;
  readonly sortable?: { readonly index: number; readonly items: readonly UniqueIdentifier[] };
}

function dragData(
  source: { data: { current?: unknown } } | null | undefined,
): DragData | undefined {
  const data = source?.data.current;
  return data === undefined || data === null ? undefined : (data as DragData);
}

const sectionId = (key: string): string => `section:${key}`;

function placement(data: DragData | undefined): { position: number; count: number } {
  return {
    position: (data?.sortable?.index ?? 0) + 1,
    count: data?.sortable?.items.length ?? 0,
  };
}

function ArrangeRoot({ sectionKeys, children }: RootFrameProps) {
  const api = useArrange();
  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 4 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 8 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
      scrollBehavior: api.reducedMotion ? 'auto' : 'smooth',
    }),
  );

  const hasMovedOver = useRef(false);
  const announcements = useMemo<Announcements>(
    () => ({
      onDragStart: ({ active }) => {
        hasMovedOver.current = false;
        const data = dragData(active);
        return data === undefined
          ? undefined
          : messageWith('arrange.pickedUp', { label: data.label, ...placement(data) });
      },
      onDragOver: ({ active, over }) => {
        const data = dragData(active);
        if (over?.id === active.id && !hasMovedOver.current) {
          return undefined;
        }
        hasMovedOver.current = true;
        return data === undefined || over === null
          ? undefined
          : messageWith('arrange.position', { label: data.label, ...placement(dragData(over)) });
      },
      onDragEnd: ({ active, over }) => {
        const data = dragData(active);
        return data === undefined
          ? undefined
          : messageWith('arrange.dropped', {
              label: data.label,
              ...placement(over === null ? data : dragData(over)),
            });
      },
      onDragCancel: ({ active }) => {
        const data = dragData(active);
        return data === undefined
          ? undefined
          : messageWith('arrange.dragCancelled', { label: data.label });
      },
    }),
    [],
  );

  const handleDragEnd = ({ active, over }: DragEndEvent): void => {
    const from = dragData(active);
    const index = dragData(over)?.sortable?.index;
    if (from === undefined || index === undefined || active.id === over?.id) {
      return;
    }
    api.moveSectionTo(from.key, index);
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragEnd={handleDragEnd}
      accessibility={{
        announcements,
        screenReaderInstructions: { draggable: message('arrange.instructions') },
      }}
    >
      <SortableContext items={sectionKeys.map(sectionId)} strategy={verticalListSortingStrategy}>
        {children}
      </SortableContext>
    </DndContext>
  );
}

function GripIcon() {
  return (
    <svg
      className="lg-arrange-handle__icon"
      viewBox="0 0 12 18"
      aria-hidden="true"
      focusable="false"
    >
      {[3, 9].flatMap((x) =>
        [3, 9, 15].map((y) => <circle key={`${x}-${y}`} cx={x} cy={y} r="1.5" />),
      )}
    </svg>
  );
}

function ArrangeSection({ sectionKey, label, title, children }: SectionFrameProps) {
  const data: DragData = { key: sectionKey, label };
  const {
    setNodeRef,
    setActivatorNodeRef,
    attributes,
    listeners,
    transform,
    isDragging,
    isSorting,
  } = useSortable({ id: sectionId(sectionKey), data, transition: null });
  const className = [
    'lg-section',
    'lg-arrange-item',
    isDragging ? 'lg-arrange-item--dragging' : '',
    isSorting && !isDragging ? 'lg-arrange-item--shifting' : '',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <section
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform) }}
      className={className}
      data-section-key={sectionKey}
    >
      <div className="lg-arrange-head">
        <button
          type="button"
          className="lg-arrange-handle"
          ref={setActivatorNodeRef}
          aria-label={messageWith('arrange.handle', { label })}
          {...attributes}
          {...listeners}
        >
          <GripIcon />
        </button>
        {title}
      </div>
      {children}
    </section>
  );
}

function ArrangeRow({ rowKey, kind, label, pinned }: RowFrameProps) {
  const api = useArrange();

  return (
    <div className="lg-row lg-arrange-row" data-row-key={rowKey} data-row-kind={kind}>
      <span className="lg-arrange-row__label">{label}</span>
      <button
        type="button"
        className="lg-arrange-star"
        aria-pressed={pinned}
        aria-label={messageWith('arrange.pinLabel', { label })}
        onClick={() => api.setPinned(rowKey, label, !pinned)}
      >
        <PinStar className="lg-arrange-star__icon" />
      </button>
    </div>
  );
}

/** The frames SheetForm renders through while Arrange mode is on. */
export const ARRANGE_FRAMES: SheetFormFrames = {
  Root: ArrangeRoot,
  Section: ArrangeSection,
  Row: ArrangeRow,
};
