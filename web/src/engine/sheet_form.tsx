/**
 * Composition of the sheet body (ENG-24, ENG-25, ENG-26): renders the sections
 * and rows derived from a template's field tree. It takes exactly one control
 * source — data plus a change handler, or the injected per-field control
 * builders — and surfaces two optional seams so a caller can own the per-field
 * value subscription and the per-row preview marks without the engine holding
 * any state.
 *
 * The body follows an optional resolved {@link Arrangement}: Sections are drawn in
 * arranged order and each pinned Row rises to the top of its own Section. Arrange-mode
 * chrome (the Section drag handle, the Row pin star) is not known here: a caller
 * replaces the structural frames — root, section, row — through {@link SheetFormFrames}.
 */

import { useId } from 'react';
import type { ComponentType, ReactNode } from 'react';
import type { FieldNode, FieldSchema } from '../api/models/template';
import { valueAtPath } from '../api/models/sheet';
import { message } from '../l10n/strings';
import type {
  SheetExclusiveGroup,
  SheetField,
  NormalRow,
  ExclusiveRow,
  SheetRow,
  TableRow,
} from './field_grouping';
import {
  buildSections,
  fieldColumnAlign,
  fieldColumnName,
  fieldDisplayName,
  groupDisplayName,
  lastSegment,
  needsTableHeader,
  rowFieldPaths,
  rowLabelText,
} from './field_grouping';
import type { ArrangedRow, ArrangedSection, Arrangement } from './layout';
import { arrangementView, resolveArrangement } from './layout';
import { SheetExclusiveControl, exclusiveValueIsOn } from './exclusive_control';
import { DependentList, DependentReadout } from './dependent_readout';
import { buildDependentsIndex, dependentsOf } from './dependents';
import { SheetFieldControl } from './field_control';
import { PinStar } from './pin_star';
import { SectionChevron } from './section_chevron';
import type { SheetMark } from './sheet_mark';
import './engine.css';

/** Props of the frame that wraps the whole body; the default draws only its children. */
export interface RootFrameProps {
  /** The Section keys in display order. */
  readonly sectionKeys: readonly string[];
  readonly children: ReactNode;
}

/** Props of the frame that is one Section; the default draws `<section>` with the title then the rows. */
export interface SectionFrameProps {
  readonly sectionKey: string;
  readonly label: string;
  readonly className: string;
  /** True when the Section's Rows are folded away (they stay mounted, hidden). */
  readonly collapsed: boolean;
  /** The Section heading element. */
  readonly title: ReactNode;
  /** The Section's Rows, inside their `.lg-section__rows` container. */
  readonly children: ReactNode;
}

/** Props of the frame that is one Row; the default draws the keyed `.lg-row` wrapper around the content. */
export interface RowFrameProps {
  readonly rowKey: string;
  readonly kind: SheetRow['kind'];
  readonly label: string;
  readonly sectionKey: string;
  /** True when the Row is pinned and therefore drawn at the top of its Section. */
  readonly pinned: boolean;
  readonly className: string;
  /** The Row's label, controls and any table header. */
  readonly children: ReactNode;
}

/** The structural frames a caller may replace to add chrome around Sections and Rows. */
export interface SheetFormFrames {
  readonly Root?: ComponentType<RootFrameProps>;
  readonly Section?: ComponentType<SectionFrameProps>;
  readonly Row?: ComponentType<RowFrameProps>;
}

function isDerivedRow(row: SheetRow): boolean {
  switch (row.kind) {
    case 'normal':
      return !row.field.schema.isEditable;
    case 'exclusive':
      return false;
    case 'table':
      return row.fields.every((field) => !field.schema.isEditable);
  }
}

function DefaultRoot({ children }: RootFrameProps) {
  return <>{children}</>;
}

function DefaultSection({ sectionKey, className, title, children }: SectionFrameProps) {
  return (
    <section className={className} data-section-key={sectionKey}>
      {title}
      {children}
    </section>
  );
}

function DefaultRow({ rowKey: key, kind, className, children }: RowFrameProps) {
  return (
    <div className={className} data-row-key={key} data-row-kind={kind}>
      {children}
    </div>
  );
}

export interface SheetFormProps {
  /** The template's decoded field tree; the sections are derived from it (ENG-01). */
  readonly fields: Record<string, FieldNode>;
  /** The current values, resolved by dotted path; absent in builder mode (ENG-24). */
  readonly data?: Record<string, unknown>;
  /** Reports an edit as `(path, value)`; absent renders the controls inert (ENG-13, ENG-24). */
  readonly onChanged?: (path: string, value: unknown) => void;
  /**
   * Per-leaf control source; given together with `exclusiveControlBuilder` (ENG-25, ENG-26).
   * `labelId` is the id of the visible Row label when the leaf is a lone Row, absent for a table
   * cell; a control may name itself by it. `describedById` is the id of the derived values drawn
   * beside the input, when there are any; the control points `aria-describedby` at it.
   */
  readonly controlBuilder?: (
    field: SheetField,
    labelId?: string,
    describedById?: string,
  ) => ReactNode;
  /** Per-exclusive-row control source; given together with `controlBuilder` (ENG-25, ENG-26). */
  readonly exclusiveControlBuilder?: (
    group: SheetExclusiveGroup,
    selectedPath: string | undefined,
    onSelect: ((signal: string) => void) | undefined,
  ) => ReactNode;
  /**
   * Draws one derived value beside the editable number that drives it (A12). Giving it turns the
   * feature on in builder mode: the caller owns the value subscription and the stale look. Data mode
   * draws the stored values on its own, never stale. See {@link dependentsOf} for which inputs have any.
   */
  readonly dependentBuilder?: (dependent: SheetField, source: SheetField) => ReactNode;
  /** Optional per-row preview mark source; orthogonal to the control source (ENG-23). */
  readonly rowMarkBuilder?: (paths: readonly string[]) => ReactNode;
  /** Sections whose keys are collapsed; the caller owns the collapse state (F3 progressive disclosure). */
  readonly collapsedSectionKeys?: readonly string[];
  /**
   * Reports a Section toggle. Giving it turns each Section title into a disclosure control;
   * without it, or while {@link SheetFormProps.onlyPinned} is on, the title is plain text.
   */
  readonly onToggleSection?: (key: string) => void;
  /**
   * Draws only the pinned Rows, each Section that has any as its normal card and the rest
   * omitted. A filter over the arrangement, never a separate zone: Rows keep their own Section.
   */
  readonly onlyPinned?: boolean;
  /**
   * Optional badge for a Section title, given the field paths of each of its drawn Rows; the
   * caller counts what it marks. Orthogonal to the control source.
   */
  readonly sectionBadgeBuilder?: (rowPaths: readonly (readonly string[])[]) => ReactNode;
  /** The resolved arrangement to draw; absent draws the Template order with nothing pinned. */
  readonly arrangement?: Arrangement;
  /** Replacement frames for the root, Sections and Rows. */
  readonly frames?: SheetFormFrames;
}

/** The copy a row shows for its preview mark (ENG-22). */
export function markLabel(mark: SheetMark): string {
  return mark === 'modified' ? message('engine.modified') : message('engine.recalculated');
}

/**
 * Renders the whole sheet body from a template field tree and exactly one
 * control source; any other combination is a construction error (ENG-25).
 */
export function SheetForm({
  fields,
  data,
  onChanged,
  controlBuilder,
  exclusiveControlBuilder,
  dependentBuilder,
  rowMarkBuilder,
  collapsedSectionKeys,
  onToggleSection,
  onlyPinned = false,
  sectionBadgeBuilder,
  arrangement,
  frames,
}: SheetFormProps) {
  const hasData = data !== undefined;
  const hasAnyBuilder = controlBuilder !== undefined || exclusiveControlBuilder !== undefined;
  const dataMode = hasData && !hasAnyBuilder;
  const builderMode =
    !hasData && controlBuilder !== undefined && exclusiveControlBuilder !== undefined;

  if (!dataMode && !builderMode) {
    throw new Error(
      'SheetForm requires exactly one control source: data plus onChanged, or both control builders.',
    );
  }

  const RootFrame = frames?.Root ?? DefaultRoot;
  const SectionFrame = frames?.Section ?? DefaultSection;
  const RowFrame = frames?.Row ?? DefaultRow;
  const formId = useId();
  const labelIdFor = (key: string): string => `${formId}-label-${key.replace(/\s/g, '_')}`;

  const rowLabel = (
    text: string,
    paths: readonly string[],
    pinned: boolean,
    id?: string,
  ): ReactNode => (
    <>
      <span className="lg-field-row__label-text" id={id}>
        {text}
      </span>
      {pinned && (
        <span className="lg-pin-mark">
          <PinStar className="lg-pin-mark__icon" />
          <span className="lg-pin-mark__text">{message('engine.pinned')}</span>
        </span>
      )}
      {rowMarkBuilder?.(paths)}
    </>
  );

  const sections = buildSections(fields);
  const dependentsIndex = buildDependentsIndex(sections);
  const beside = dependentBuilder !== undefined || dataMode;

  const dependentsBeside = (field: SheetField): readonly SheetField[] =>
    beside ? dependentsOf(dependentsIndex, field) : [];

  const dependentsId = (path: string): string => `${formId}-deps-${path.replace(/\s/g, '_')}`;

  const buildDependent = (dependent: SheetField, source: SheetField): ReactNode => {
    if (dependentBuilder !== undefined) {
      return dependentBuilder(dependent, source);
    }
    return (
      <DependentReadout
        label={fieldDisplayName(lastSegment(dependent.path), dependent.schema)}
        value={data === undefined ? null : valueAtPath(data, dependent.path)}
        stale={false}
      />
    );
  };

  const renderControl = (
    field: SheetField,
    labelId?: string,
    describedById?: string,
  ): ReactNode => {
    if (controlBuilder !== undefined) {
      return controlBuilder(field, labelId, describedById);
    }
    return (
      <SheetFieldControl
        path={field.path}
        labelId={labelId}
        describedBy={describedById}
        schema={field.schema}
        value={data === undefined ? null : valueAtPath(data, field.path)}
        onChanged={
          onChanged === undefined ? undefined : (value: unknown) => onChanged(field.path, value)
        }
      />
    );
  };

  const activeOption = (group: SheetExclusiveGroup): string | undefined => {
    if (data === undefined) {
      return undefined;
    }
    for (const option of group.options) {
      if (exclusiveValueIsOn(valueAtPath(data, option.path))) {
        return option.path;
      }
    }
    return undefined;
  };

  const cellClass = (schema: FieldSchema): string =>
    `lg-field-table__cell${
      fieldColumnAlign(schema) === 'right' ? ' lg-field-table__cell--number' : ''
    }`;

  const renderNormalRow = (row: NormalRow, pinned: boolean): ReactNode => {
    const field = row.field;
    const labelId = labelIdFor(field.path);
    const dependents = dependentsBeside(field);
    const describedById = dependents.length === 0 ? undefined : dependentsId(field.path);
    return (
      <>
        <span className="lg-field-row__label">
          {rowLabel(
            fieldDisplayName(lastSegment(field.path), field.schema),
            [field.path],
            pinned,
            labelId,
          )}
        </span>
        <span className="lg-field-row__control">
          {renderControl(field, labelId, describedById)}
        </span>
        {describedById !== undefined && (
          <DependentList
            id={describedById}
            source={field}
            dependents={dependents}
            build={buildDependent}
          />
        )}
      </>
    );
  };

  const renderExclusiveRow = (row: ExclusiveRow, pinned: boolean): ReactNode => {
    const group = row.group;
    const selectedPath = activeOption(group);
    return (
      <>
        <span className="lg-field-row__label">
          {rowLabel(
            groupDisplayName(group.id),
            group.options.map((option) => option.path),
            pinned,
          )}
        </span>
        <span className="lg-field-row__control">
          {exclusiveControlBuilder === undefined ? (
            <SheetExclusiveControl group={group} selectedPath={selectedPath} />
          ) : (
            exclusiveControlBuilder(group, selectedPath, undefined)
          )}
        </span>
      </>
    );
  };

  const renderTableHeader = (row: TableRow): ReactNode => (
    <div className="lg-field-table__header">
      <span className="lg-field-table__lead" />
      <span className="lg-field-table__cells">
        {row.fields.map((field) => (
          <span className={cellClass(field.schema)} key={field.path}>
            {fieldColumnName(lastSegment(field.path), field.schema)}
          </span>
        ))}
      </span>
    </div>
  );

  const renderTableRow = (row: TableRow, withHeader: boolean, pinned: boolean): ReactNode => (
    <>
      {withHeader && renderTableHeader(row)}
      <span className="lg-field-table__lead">
        {rowLabel(
          row.label,
          row.fields.map((field) => field.path),
          pinned,
        )}
      </span>
      <span className="lg-field-table__cells">
        {row.fields.map((field) => (
          <span className={cellClass(field.schema)} key={field.path}>
            {renderControl(field)}
          </span>
        ))}
      </span>
    </>
  );

  const frameClass = (row: SheetRow): string => {
    const layout = row.kind === 'table' ? 'lg-field-table__row' : 'lg-field-row';
    const withDependents = row.kind === 'normal' && dependentsBeside(row.field).length > 0;
    return `lg-row ${layout}${isDerivedRow(row) ? ' lg-row--derived' : ''}${
      withDependents ? ' lg-field-row--dependents' : ''
    }`;
  };

  const renderRow = (arranged: ArrangedRow, previous: SheetRow | undefined): ReactNode => {
    const row = arranged.row;
    let content: ReactNode;
    if (row.kind === 'table') {
      content = renderTableRow(row, needsTableHeader(previous, row), arranged.pinned);
    } else if (row.kind === 'exclusive') {
      content = renderExclusiveRow(row, arranged.pinned);
    } else {
      content = renderNormalRow(row, arranged.pinned);
    }
    return (
      <RowFrame
        key={arranged.key}
        rowKey={arranged.key}
        kind={row.kind}
        label={rowLabelText(row)}
        sectionKey={arranged.sectionKey}
        pinned={arranged.pinned}
        className={frameClass(row)}
      >
        {content}
      </RowFrame>
    );
  };

  const arranged = arrangementView(
    sections,
    arrangement ?? resolveArrangement(sections, undefined),
  );
  const view = onlyPinned
    ? arranged
        .map((entry) => ({ ...entry, rows: entry.rows.filter((row) => row.pinned) }))
        .filter((entry) => entry.rows.length > 0)
    : arranged;
  const disclosure = !onlyPinned && onToggleSection !== undefined;

  const renderTitle = (
    { section, rows }: ArrangedSection,
    collapsed: boolean,
    rowsId: string,
  ): ReactNode => {
    if (!disclosure) {
      return <h2 className="lg-section__title">{section.label}</h2>;
    }
    return (
      <h2 className="lg-section__title lg-section__title--toggle">
        <button
          type="button"
          className="lg-section__toggle"
          aria-expanded={!collapsed}
          aria-controls={rowsId}
          onClick={() => onToggleSection(section.key)}
        >
          <span className="lg-section__toggle-label">{section.label}</span>
          {collapsed && sectionBadgeBuilder?.(rows.map((entry) => rowFieldPaths(entry.row)))}
          <SectionChevron className="lg-section__chevron" />
        </button>
      </h2>
    );
  };

  const renderSection = (entry: ArrangedSection): ReactNode => {
    const { section, rows } = entry;
    const collapsed = disclosure && (collapsedSectionKeys?.includes(section.key) ?? false);
    const rowsId = `lg-rows-${section.key}`;
    return (
      <SectionFrame
        key={section.key}
        sectionKey={section.key}
        label={section.label}
        className={collapsed ? 'lg-section lg-section--collapsed' : 'lg-section'}
        collapsed={collapsed}
        title={renderTitle(entry, collapsed, rowsId)}
      >
        <div className="lg-section__rows" id={rowsId} hidden={collapsed}>
          {rows.map((arrangedRow, index) => renderRow(arrangedRow, rows[index - 1]?.row))}
        </div>
      </SectionFrame>
    );
  };

  return (
    <div className="lg-engine">
      <RootFrame sectionKeys={view.map((entry) => entry.section.key)}>
        {view.map(renderSection)}
      </RootFrame>
    </div>
  );
}
