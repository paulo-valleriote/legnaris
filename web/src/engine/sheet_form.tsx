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
import { ChevronRightIcon } from '../ui/icons';
import type { SheetMark } from './sheet_mark';
import './engine.css';

export interface RootFrameProps {
  readonly sectionKeys: readonly string[];
  readonly children: ReactNode;
}

export interface SectionFrameProps {
  readonly sectionKey: string;
  readonly label: string;
  readonly className: string;
  readonly collapsed: boolean;
  readonly title: ReactNode;
  readonly children: ReactNode;
}

export interface RowFrameProps {
  readonly rowKey: string;
  readonly kind: SheetRow['kind'];
  readonly label: string;
  readonly sectionKey: string;
  readonly pinned: boolean;
  readonly className: string;
  readonly children: ReactNode;
}

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
  readonly fields: Record<string, FieldNode>;
  readonly data?: Record<string, unknown>;
  readonly onChanged?: (path: string, value: unknown) => void;
  readonly controlBuilder?: (
    field: SheetField,
    labelId?: string,
    describedById?: string,
  ) => ReactNode;
  readonly exclusiveControlBuilder?: (
    group: SheetExclusiveGroup,
    selectedPath: string | undefined,
    onSelect: ((signal: string) => void) | undefined,
  ) => ReactNode;
  readonly dependentBuilder?: (dependent: SheetField, source: SheetField) => ReactNode;
  readonly rowMarkBuilder?: (paths: readonly string[], rowKey: string) => ReactNode;
  readonly collapsedSectionKeys?: readonly string[];
  readonly onToggleSection?: (key: string) => void;
  readonly onlyPinned?: boolean;
  readonly sectionBadgeBuilder?: (
    rowPaths: readonly (readonly string[])[],
    rowKeys: readonly string[],
  ) => ReactNode;
  readonly rowActionsBuilder?: (row: RowActionsInfo) => ReactNode;
  readonly arrangement?: Arrangement;
  readonly frames?: SheetFormFrames;
}

export interface RowActionsInfo {
  readonly rowKey: string;
  readonly sectionKey: string;
  readonly label: string;
}

export function markLabel(mark: SheetMark): string {
  return mark === 'modified' ? message('engine.modified') : message('engine.recalculated');
}

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
  rowActionsBuilder,
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
    key: string,
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
      {rowMarkBuilder?.(paths, key)}
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

  const renderNormalRow = (
    row: NormalRow,
    key: string,
    pinned: boolean,
    actions: ReactNode,
  ): ReactNode => {
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
            key,
            pinned,
            labelId,
          )}
          {actions}
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

  const renderExclusiveRow = (
    row: ExclusiveRow,
    key: string,
    pinned: boolean,
    actions: ReactNode,
  ): ReactNode => {
    const group = row.group;
    const selectedPath = activeOption(group);
    return (
      <>
        <span className="lg-field-row__label">
          {rowLabel(
            groupDisplayName(group.id),
            group.options.map((option) => option.path),
            key,
            pinned,
          )}
          {actions}
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

  const renderTableRow = (
    row: TableRow,
    key: string,
    withHeader: boolean,
    pinned: boolean,
    actions: ReactNode,
  ): ReactNode => (
    <>
      {withHeader && renderTableHeader(row)}
      <span className="lg-field-table__lead">
        {rowLabel(
          row.label,
          row.fields.map((field) => field.path),
          key,
          pinned,
        )}
        {actions}
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
    const actions = rowActionsBuilder?.({
      rowKey: arranged.key,
      sectionKey: arranged.sectionKey,
      label: rowLabelText(row),
    });
    if (row.kind === 'table') {
      content = renderTableRow(
        row,
        arranged.key,
        needsTableHeader(previous, row),
        arranged.pinned,
        actions,
      );
    } else if (row.kind === 'exclusive') {
      content = renderExclusiveRow(row, arranged.key, arranged.pinned, actions);
    } else {
      content = renderNormalRow(row, arranged.key, arranged.pinned, actions);
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
          {collapsed &&
            sectionBadgeBuilder?.(
              rows.map((entry) => rowFieldPaths(entry.row)),
              rows.map((entry) => entry.key),
            )}
          <ChevronRightIcon className="lg-section__chevron" />
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
