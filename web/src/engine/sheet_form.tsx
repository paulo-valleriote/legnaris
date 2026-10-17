/**
 * Composition of the sheet body (ENG-24, ENG-25, ENG-26): renders the sections
 * and rows derived from a template's field tree. It takes exactly one control
 * source — data plus a change handler, or the injected per-field control
 * builders — and surfaces two optional seams so a caller can own the per-field
 * value subscription and the per-row preview marks without the engine holding
 * any state.
 */

import type { ReactNode } from 'react';
import type { FieldNode, FieldSchema } from '../api/models/template';
import { valueAtPath } from '../api/models/sheet';
import { message } from '../l10n/strings';
import type { SheetExclusiveGroup, SheetField, SheetSection, TableRow } from './field_grouping';
import {
  buildSections,
  fieldColumnAlign,
  fieldColumnName,
  fieldDisplayName,
  groupDisplayName,
  lastSegment,
} from './field_grouping';
import { SheetExclusiveControl, exclusiveValueIsOn } from './exclusive_control';
import { SheetFieldControl } from './field_control';
import type { SheetMark } from './sheet_mark';
import './engine.css';

export interface SheetFormProps {
  /** The template's decoded field tree; the sections are derived from it (ENG-01). */
  readonly fields: Record<string, FieldNode>;
  /** The current values, resolved by dotted path; absent in builder mode (ENG-24). */
  readonly data?: Record<string, unknown>;
  /** Reports an edit as `(path, value)`; absent renders the controls inert (ENG-13, ENG-24). */
  readonly onChanged?: (path: string, value: unknown) => void;
  /** Per-leaf control source; given together with `exclusiveControlBuilder` (ENG-25, ENG-26). */
  readonly controlBuilder?: (field: SheetField) => ReactNode;
  /** Per-exclusive-row control source; given together with `controlBuilder` (ENG-25, ENG-26). */
  readonly exclusiveControlBuilder?: (
    group: SheetExclusiveGroup,
    selectedPath: string | undefined,
    onSelect: ((signal: string) => void) | undefined,
  ) => ReactNode;
  /** Optional per-row preview mark source; orthogonal to the control source (ENG-23). */
  readonly rowMarkBuilder?: (paths: readonly string[]) => ReactNode;
  /** Sections whose keys are collapsed; the caller owns the collapse state (F3 progressive disclosure). */
  readonly collapsedSectionKeys?: readonly string[];
  /** Reports a section toggle when progressive disclosure is enabled. */
  readonly onToggleSection?: (key: string) => void;
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
  rowMarkBuilder,
  collapsedSectionKeys,
  onToggleSection,
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

  const rowLabel = (text: string, paths: readonly string[]): ReactNode => (
    <>
      <span className="lg-field-row__label-text">{text}</span>
      {rowMarkBuilder?.(paths)}
    </>
  );

  const renderControl = (field: SheetField): ReactNode => {
    if (controlBuilder !== undefined) {
      return controlBuilder(field);
    }
    return (
      <SheetFieldControl
        path={field.path}
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

  const columnsKey = (row: TableRow): string =>
    row.fields
      .map((field) => fieldColumnName(lastSegment(field.path), field.schema))
      .join('\u0000');

  const renderNormalRow = (field: SheetField, key: string): ReactNode => (
    <div className="lg-field-row" key={key}>
      <span className="lg-field-row__label">
        {rowLabel(fieldDisplayName(lastSegment(field.path), field.schema), [field.path])}
      </span>
      <span className="lg-field-row__control">{renderControl(field)}</span>
    </div>
  );

  const renderExclusiveRow = (group: SheetExclusiveGroup, key: string): ReactNode => {
    const selectedPath = activeOption(group);
    return (
      <div className="lg-field-row" key={key}>
        <span className="lg-field-row__label">
          {rowLabel(
            groupDisplayName(group.id),
            group.options.map((option) => option.path),
          )}
        </span>
        <span className="lg-field-row__control">
          {exclusiveControlBuilder === undefined ? (
            <SheetExclusiveControl group={group} selectedPath={selectedPath} />
          ) : (
            exclusiveControlBuilder(group, selectedPath, undefined)
          )}
        </span>
      </div>
    );
  };

  const renderTableHeader = (
    sectionKey: string,
    tableIndex: number,
    row: TableRow,
    key: string,
  ): ReactNode => (
    <div
      className="lg-field-table__header"
      key={key}
      data-testid={`table-header-${sectionKey}-${tableIndex}`}
    >
      <span className="lg-field-table__lead" />
      {row.fields.map((field) => (
        <span className={cellClass(field.schema)} key={field.path}>
          {fieldColumnName(lastSegment(field.path), field.schema)}
        </span>
      ))}
    </div>
  );

  const renderTableRow = (row: TableRow, key: string): ReactNode => (
    <div className="lg-field-table__row" key={key}>
      <span className="lg-field-table__lead">
        {rowLabel(
          row.label,
          row.fields.map((field) => field.path),
        )}
      </span>
      {row.fields.map((field) => (
        <span className={cellClass(field.schema)} key={field.path}>
          {renderControl(field)}
        </span>
      ))}
    </div>
  );

  const renderSection = (section: SheetSection): ReactNode => {
    const isCollapsed = collapsedSectionKeys?.includes(section.key) ?? false;
    const rows: ReactNode[] = [];
    if (isCollapsed) {
      return (
        <section className="lg-section lg-section--collapsed" key={section.key}>
          <button
            type="button"
            className="lg-button lg-button--secondary lg-button--sm"
            onClick={() => onToggleSection?.(section.key)}
            aria-expanded="false"
            aria-label={`${section.label}: ${message('engine.expand')}`}
          >
            <span className="lg-section__title">{section.label}</span>
            <span>{message('engine.expand')}</span>
          </button>
        </section>
      );
    }
    let activeColumns: string | undefined;
    let tableIndex = 0;
    section.rows.forEach((row, index) => {
      const key = `${section.key}-${index}`;
      if (row.kind === 'exclusive') {
        rows.push(renderExclusiveRow(row.group, key));
        activeColumns = undefined;
      } else if (row.kind === 'table') {
        const key_ = columnsKey(row);
        if (key_ !== activeColumns) {
          rows.push(renderTableHeader(section.key, tableIndex, row, `${key}-header`));
        }
        rows.push(renderTableRow(row, key));
        activeColumns = key_;
        tableIndex += 1;
      } else {
        rows.push(renderNormalRow(row.field, key));
        activeColumns = undefined;
      }
    });

    return (
      <section className="lg-section" key={section.key}>
        <h2 className="lg-section__title">{section.label}</h2>
        {rows}
      </section>
    );
  };

  return <div className="lg-engine">{buildSections(fields).map(renderSection)}</div>;
}
