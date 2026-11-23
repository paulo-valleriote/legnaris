import { valueAtPath } from '../api/models/sheet';
import type { ListSchema } from '../api/models/template';
import { message, messageWith } from '../l10n/strings';
import type { ListColumn } from './field_grouping';
import {
  fieldColumnAlign,
  fieldColumnName,
  lastSegment,
  listColumns,
  recordRowKey,
} from './field_grouping';
import { orderedRecords } from './row_identity';
import './engine.css';

export interface ReadOnlyRecordTableProps {
  readonly listPath: string;
  readonly list: ListSchema;
  readonly data: Record<string, unknown>;
}

function cellText(value: unknown): { readonly text: string; readonly empty: boolean } {
  if (value === null || value === undefined || value === '') {
    return { text: message('engine.notSet'), empty: true };
  }
  if (typeof value === 'boolean') {
    return { text: message(value ? 'record.on' : 'record.off'), empty: false };
  }
  return { text: String(value), empty: false };
}

function columnLabel(column: ListColumn): string {
  return column.kind === 'field'
    ? fieldColumnName(lastSegment(column.name), column.schema)
    : column.label;
}

function activeOptionLabel(
  column: Extract<ListColumn, { kind: 'exclusive' }>,
  record: Record<string, unknown>,
): string {
  for (const option of column.options) {
    const value = record[option.name];
    if (typeof value === 'number' && value !== 0) {
      const label = option.schema.optionLabel?.trim();
      return label === undefined || label === ''
        ? fieldColumnName(lastSegment(option.name), option.schema)
        : label;
    }
  }
  return message('engine.none');
}

export function ReadOnlyRecordTable({ listPath, list, data }: ReadOnlyRecordTableProps) {
  const columns = listColumns(list.shape, list.columns);
  const records = orderedRecords(listPath, data);

  return (
    <div className="lg-record-table" data-list-path={listPath}>
      {records.length === 0 ? (
        <p className="lg-record-empty">{message('record.empty')}</p>
      ) : (
        <>
          <div className="lg-record-row lg-record-row__header">
            <span className="lg-record-row__lead" aria-hidden="true" />
            <span className="lg-record-row__cells">
              {columns.map((column, index) => (
                <span className="lg-record-cell" key={`${columnLabel(column)}-${index}`}>
                  <span className="lg-record-cell__label">{columnLabel(column)}</span>
                </span>
              ))}
            </span>
          </div>
          {records.map((entry, index) => {
            const key = recordRowKey(listPath, entry.id);
            const record = valueAtPath(data, key);
            const cells = (record ?? {}) as Record<string, unknown>;
            return (
              <div className="lg-record-row" data-record-key={key} data-row-kind="record" key={key}>
                <span className="lg-record-row__lead">
                  {messageWith('record.rowLabel', { list: list.label, position: index + 1 })}
                </span>
                <span className="lg-record-row__cells">
                  {columns.map((column, columnIndex) => {
                    const cell =
                      column.kind === 'field'
                        ? cellText(cells[column.name])
                        : { text: activeOptionLabel(column, cells), empty: false };
                    const numeric =
                      column.kind === 'field' && fieldColumnAlign(column.schema) === 'right';
                    const derived = column.kind === 'field' && !column.schema.isEditable;
                    return (
                      <span
                        className={`lg-record-cell${numeric ? ' lg-record-cell--number' : ''}`}
                        key={`${columnLabel(column)}-${columnIndex}`}
                      >
                        <span className="lg-record-cell__label">{columnLabel(column)}</span>
                        <span
                          className={`lg-record-cell__value${
                            cell.empty ? ' lg-record-cell__value--empty' : ''
                          }${numeric ? ' lg-record-cell__value--number' : ''}${
                            derived ? ' lg-record-cell__value--derived' : ''
                          }`}
                        >
                          {cell.text}
                        </span>
                      </span>
                    );
                  })}
                </span>
              </div>
            );
          })}
        </>
      )}
    </div>
  );
}
