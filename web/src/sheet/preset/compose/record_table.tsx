import { useMemo, useSyncExternalStore } from 'react';
import { valueAtPath } from '../../../api/models/sheet';
import type { FieldNode } from '../../../api/models/template';
import { EXCLUSIVE_NONE_VALUE, SheetExclusiveControl } from '../../../engine/exclusive_control';
import { SheetFieldControl } from '../../../engine/field_control';
import type { ListColumn, SheetList } from '../../../engine/field_grouping';
import {
  buildSections,
  fieldColumnAlign,
  fieldColumnName,
  lastSegment,
  sectionKeyOfRow,
} from '../../../engine/field_grouping';
import { listInfos, orderedRecords, recordRowKey } from '../../../engine/row_identity';
import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { CloseIcon, PlusIcon } from '../../../ui/icons';
import { usePresetContext } from '../preset_context';
import { RollableValue } from '../rollable_value';
import type { PresetRecordTablePiece } from './types';
import './compose.css';

function useEditData(): Record<string, unknown> {
  const { store } = usePresetContext();
  return useSyncExternalStore(store.subscribe, () => store.getState().data);
}

function sheetList(
  fields: Readonly<Record<string, FieldNode>>,
  path: string,
): SheetList | undefined {
  for (const section of buildSections(fields)) {
    const found = section.lists.find((list) => list.path === path);
    if (found !== undefined) {
      return found;
    }
  }
  return undefined;
}

function chosenColumns(
  list: SheetList,
  chosen: readonly string[] | undefined,
): readonly ListColumn[] {
  if (chosen === undefined) {
    return list.columns;
  }
  return list.columns.filter((column) =>
    column.kind === 'field'
      ? chosen.includes(column.name)
      : chosen.includes(column.id) || column.options.some((option) => chosen.includes(option.name)),
  );
}

function columnLabel(column: ListColumn): string {
  return column.kind === 'field'
    ? fieldColumnName(lastSegment(column.name), column.schema)
    : column.label;
}

function selectedPath(
  options: readonly { readonly path: string }[],
  data: Record<string, unknown>,
) {
  for (const option of options) {
    const value = valueAtPath(data, option.path);
    if (value === true || (typeof value === 'number' && value !== 0)) {
      return option.path;
    }
  }
  return undefined;
}

interface RecordRowProps {
  readonly list: SheetList;
  readonly recordID: string;
  readonly position: number;
  readonly columns: readonly ListColumn[];
  readonly data: Record<string, unknown>;
  readonly sectionKey: string;
  readonly rowDefault: boolean;
}

function RecordRowView({
  list,
  recordID,
  position,
  columns,
  data,
  sectionKey,
  rowDefault,
}: RecordRowProps) {
  const { store } = usePresetContext();
  const label = messageWith('record.rowLabel', { list: list.label, position });
  const key = recordRowKey(list.path, recordID);

  return (
    <div className="lg-compose-record__row" data-testid="compose-record-row" data-record-key={key}>
      <span className="lg-compose-record__lead">
        {rowDefault ? (
          <RollableValue rowKey={key} sectionKey={sectionKey} label={label} text={label} />
        ) : (
          label
        )}
        <Button
          variant="ghost"
          size="sm"
          icon={<CloseIcon />}
          label={messageWith('record.remove', { label })}
          onClick={() => store.removeRecord?.(list.path, recordID)}
        />
      </span>
      <span className="lg-compose-record__cells">
        {columns.map((column, index) => {
          const name = column.kind === 'field' ? column.name : column.id;
          const options =
            column.kind === 'field'
              ? []
              : column.options.map((option) => ({
                  path: `${key}.${option.name}`,
                  schema: option.schema,
                }));
          return (
            <span
              className={`lg-compose-record__cell${
                column.kind === 'field' && fieldColumnAlign(column.schema) === 'right'
                  ? ' lg-compose-record__cell--number'
                  : ''
              }`}
              key={`${name}-${index}`}
            >
              <span className="lg-compose-record__label">{columnLabel(column)}</span>
              {column.kind === 'field' ? (
                <SheetFieldControl
                  path={`${key}.${column.name}`}
                  schema={column.schema}
                  value={valueAtPath(data, `${key}.${column.name}`)}
                  onChanged={(value) => store.setPath(`${key}.${column.name}`, value)}
                />
              ) : (
                <SheetExclusiveControl
                  group={{ id: column.id, options }}
                  selectedPath={selectedPath(options, data)}
                  controlId={`compose-${key.replace(/[^a-zA-Z0-9_-]/g, '-')}-${column.id}`}
                  onSelect={(selected) =>
                    store.setExclusive(
                      selected === EXCLUSIVE_NONE_VALUE ? null : selected,
                      options.map((option) => option.path),
                    )
                  }
                />
              )}
            </span>
          );
        })}
      </span>
    </div>
  );
}

export function RecordTable({ piece }: { readonly piece: PresetRecordTablePiece }) {
  const { template, store } = usePresetContext();
  const data = useEditData();
  const list = useMemo(() => sheetList(template.fields, piece.list), [template, piece.list]);
  const info = useMemo(
    () => listInfos(template.fields).find((entry) => entry.path === piece.list),
    [template, piece.list],
  );

  if (list === undefined) {
    return null;
  }

  const columns = chosenColumns(list, piece.columns);
  const records = orderedRecords(list.path, data);
  const sectionKey = info?.section ?? sectionKeyOfRow(list.path);
  const rowDefault = info?.dice !== undefined;
  const addDisabledReason =
    records.length >= list.capacity
      ? messageWith('record.atCapacity', { count: records.length, capacity: list.capacity })
      : undefined;

  return (
    <div
      className="lg-compose-record"
      role="group"
      aria-label={list.label}
      data-testid="compose-record-table"
    >
      {records.length === 0 ? (
        <p className="lg-compose-record__empty">{message('record.empty')}</p>
      ) : (
        records.map((entry, index) => (
          <RecordRowView
            key={entry.id}
            list={list}
            recordID={entry.id}
            position={index + 1}
            columns={columns}
            data={data}
            sectionKey={sectionKey}
            rowDefault={rowDefault}
          />
        ))
      )}
      {store.addRecord === undefined ? null : (
        <div className="lg-compose-record__add">
          <Button
            variant="ghost"
            size="sm"
            icon={<PlusIcon />}
            label={messageWith('record.add', { list: list.label })}
            disabledReason={addDisabledReason}
            onClick={() => store.addRecord?.(list.path)}
          />
        </div>
      )}
    </div>
  );
}
