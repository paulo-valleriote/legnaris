import { useEffect, useId, useRef, useState } from 'react';
import type { ComponentType, ReactNode } from 'react';
import type { FieldNode, FieldSchema } from '../api/models/template';
import { valueAtPath } from '../api/models/sheet';
import { message, messageWith } from '../l10n/strings';
import type {
  RecordCell,
  RecordRow,
  RecordRows,
  SheetExclusiveGroup,
  SheetField,
  SheetList,
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
  needsRecordHeader,
  needsTableHeader,
  recordCellFields,
  recordRowKey,
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
import { recordsByList as recordsFromData } from './row_identity';
import { Button } from '../ui/button';
import { ChevronDownIcon, ChevronRightIcon, CloseIcon, PlusIcon } from '../ui/icons';
import type { SheetMark } from './sheet_mark';
import './engine.css';

export interface RecordListActions {
  readonly add: (listPath: string) => string | undefined;
  readonly remove: (listPath: string, recordID: string) => void;
  readonly move: (listPath: string, recordID: string, direction: -1 | 1) => void;
}

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
    case 'record':
      return false;
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
    controlId?: string,
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
  readonly records?: RecordRows;
  readonly recordActions?: RecordListActions;
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

type RecordFocus =
  | { readonly kind: 'add'; readonly id: string }
  | {
      readonly kind: 'record';
      readonly key: string;
      readonly action?: 'up' | 'down' | 'remove';
    };

function recordRowElement(root: HTMLElement, key: string): HTMLElement | undefined {
  return Array.from(root.querySelectorAll<HTMLElement>('[data-row-key]')).find(
    (element) => element.dataset.rowKey === key,
  );
}

function focusableControls(row: HTMLElement): HTMLElement[] {
  return Array.from(
    row.querySelectorAll<HTMLElement>(
      '.lg-record-row__cells input, .lg-record-row__cells select, .lg-record-row__cells textarea, .lg-record-row__cells button',
    ),
  ).filter((element) => !(element as HTMLButtonElement).disabled);
}

function actionButton(row: HTMLElement, action: 'up' | 'down' | 'remove'): HTMLElement | undefined {
  const button = row.querySelector<HTMLButtonElement>(`[data-record-action="${action}"] button`);
  return button !== null && !button.disabled ? button : undefined;
}

function focusRecordRequest(root: HTMLElement, request: RecordFocus): void {
  if (request.kind === 'add') {
    root.querySelector<HTMLElement>(`[id="${request.id}"]`)?.focus();
    return;
  }
  const row = recordRowElement(root, request.key);
  if (row === undefined) {
    return;
  }
  const target =
    (request.action === undefined ? undefined : actionButton(row, request.action)) ??
    focusableControls(row)[0] ??
    actionButton(row, 'remove') ??
    actionButton(row, 'up');
  target?.focus();
}

function recordColumnLabel(cell: RecordCell): string {
  return cell.kind === 'field'
    ? fieldColumnName(lastSegment(cell.field.path), cell.field.schema)
    : cell.label;
}

function recordCellKey(cell: RecordCell, index: number): string {
  return cell.kind === 'field' ? cell.field.path : `${cell.group.id}-${index}`;
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
  records,
  recordActions,
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
  const rootRef = useRef<HTMLDivElement>(null);
  const [pendingFocus, setPendingFocus] = useState<RecordFocus | null>(null);
  const labelIdFor = (key: string): string => `${formId}-label-${key.replace(/\s/g, '_')}`;
  const addIdFor = (listPath: string): string => `${formId}-add-${listPath.replace(/\s/g, '_')}`;
  const recordRows = records ?? (data === undefined ? undefined : recordsFromData(fields, data));

  useEffect(() => {
    if (pendingFocus === null) {
      return;
    }
    const root = rootRef.current;
    if (root !== null) {
      focusRecordRequest(root, pendingFocus);
    }
  }, [pendingFocus]);

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

  const sections = buildSections(fields, recordRows);
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

  const renderRecordCell = (cell: RecordCell): ReactNode => {
    if (cell.kind === 'field') {
      return renderControl(cell.field);
    }
    const selectedPath = activeOption(cell.group);
    const controlId = `${formId}-${cell.group.options[0]?.path.replace(/\s/g, '_')}`;
    return exclusiveControlBuilder === undefined ? (
      <SheetExclusiveControl group={cell.group} selectedPath={selectedPath} controlId={controlId} />
    ) : (
      exclusiveControlBuilder(cell.group, selectedPath, undefined, controlId)
    );
  };

  const renderRecordHeader = (row: RecordRow): ReactNode => (
    <div className="lg-record-row__header">
      <span className="lg-record-row__lead" aria-hidden="true" />
      <span className="lg-record-row__cells">
        {row.columns.map((cell, index) => (
          <span className="lg-record-cell" key={recordCellKey(cell, index)}>
            <span className="lg-record-cell__label">{recordColumnLabel(cell)}</span>
          </span>
        ))}
      </span>
    </div>
  );

  const runAdd = (list: SheetList): void => {
    const recordID = recordActions?.add(list.path);
    setPendingFocus(
      recordID === undefined ? null : { kind: 'record', key: recordRowKey(list.path, recordID) },
    );
  };

  const runRemove = (row: RecordRow): void => {
    const ids = recordRows?.[row.listPath] ?? [];
    const index = ids.indexOf(row.recordID);
    const next = ids[index + 1] ?? ids[index - 1];
    recordActions?.remove(row.listPath, row.recordID);
    setPendingFocus(
      next === undefined
        ? { kind: 'add', id: addIdFor(row.listPath) }
        : { kind: 'record', key: recordRowKey(row.listPath, next) },
    );
  };

  const runMove = (row: RecordRow, direction: -1 | 1): void => {
    recordActions?.move(row.listPath, row.recordID, direction);
    setPendingFocus({
      kind: 'record',
      key: recordRowKey(row.listPath, row.recordID),
      action: direction === 1 ? 'down' : 'up',
    });
  };

  const renderRecordActions = (row: RecordRow): ReactNode => {
    if (recordActions === undefined) {
      return null;
    }
    const ids = recordRows?.[row.listPath] ?? [];
    const index = ids.indexOf(row.recordID);
    const label = rowLabelText(row);
    return (
      <span className="lg-record-row__actions">
        <span data-record-action="up">
          <Button
            variant="ghost"
            size="sm"
            className="lg-record__action"
            icon={<ChevronDownIcon className="lg-record__action-icon lg-record__action-icon--up" />}
            label={messageWith('record.moveUp', { label })}
            disabled={index <= 0}
            onClick={() => runMove(row, -1)}
          >
            <span aria-hidden="true" />
          </Button>
        </span>
        <span data-record-action="down">
          <Button
            variant="ghost"
            size="sm"
            className="lg-record__action"
            icon={<ChevronDownIcon className="lg-record__action-icon" />}
            label={messageWith('record.moveDown', { label })}
            disabled={index < 0 || index >= ids.length - 1}
            onClick={() => runMove(row, 1)}
          >
            <span aria-hidden="true" />
          </Button>
        </span>
        <span data-record-action="remove">
          <Button
            variant="ghost"
            size="sm"
            className="lg-record__action"
            icon={<CloseIcon className="lg-record__action-icon" />}
            label={messageWith('record.remove', { label })}
            onClick={() => runRemove(row)}
          >
            <span aria-hidden="true" />
          </Button>
        </span>
      </span>
    );
  };

  const renderRecordRow = (
    row: RecordRow,
    key: string,
    withHeader: boolean,
    pinned: boolean,
    actions: ReactNode,
  ): ReactNode => (
    <>
      {withHeader && renderRecordHeader(row)}
      <span className="lg-record-row__lead">
        {rowLabel(
          rowLabelText(row),
          recordCellFields(row).map((field) => field.path),
          key,
          pinned,
        )}
        {renderRecordActions(row)}
        {actions}
      </span>
      <span className="lg-record-row__cells">
        {row.columns.map((cell, index) => (
          <span
            className={`lg-record-cell${
              cell.kind === 'field' && fieldColumnAlign(cell.field.schema) === 'right'
                ? ' lg-record-cell--number'
                : ''
            }`}
            key={recordCellKey(cell, index)}
          >
            <span className="lg-record-cell__label">{recordColumnLabel(cell)}</span>
            {renderRecordCell(cell)}
          </span>
        ))}
      </span>
    </>
  );

  const renderAddRow = (list: SheetList): ReactNode => {
    if (onlyPinned) {
      return null;
    }
    const count = recordRows?.[list.path]?.length ?? 0;
    if (recordActions === undefined) {
      return count === 0 ? (
        <div className="lg-record-add" key={`add-${list.path}`}>
          <p className="lg-record-empty">{message('record.empty')}</p>
        </div>
      ) : null;
    }
    return (
      <div className="lg-record-add" key={`add-${list.path}`}>
        {count === 0 && <p className="lg-record-empty">{message('record.empty')}</p>}
        <Button
          id={addIdFor(list.path)}
          variant="ghost"
          size="sm"
          icon={<PlusIcon />}
          label={messageWith('record.add', { list: list.label })}
          disabled={count >= list.capacity}
          disabledReason={
            count >= list.capacity
              ? messageWith('record.atCapacity', { count, capacity: list.capacity })
              : undefined
          }
          onClick={() => runAdd(list)}
        />
      </div>
    );
  };

  const frameClass = (row: SheetRow): string => {
    const layout =
      row.kind === 'table'
        ? 'lg-field-table__row'
        : row.kind === 'record'
          ? 'lg-record-row'
          : 'lg-field-row';
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
    if (row.kind === 'record') {
      content = renderRecordRow(
        row,
        arranged.key,
        needsRecordHeader(previous, row),
        arranged.pinned,
        actions,
      );
    } else if (row.kind === 'table') {
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
          {!collapsed && section.lists.map((list) => renderAddRow(list))}
        </div>
      </SectionFrame>
    );
  };

  return (
    <div className="lg-engine" ref={rootRef}>
      <RootFrame sectionKeys={view.map((entry) => entry.section.key)}>
        {view.map(renderSection)}
      </RootFrame>
    </div>
  );
}
