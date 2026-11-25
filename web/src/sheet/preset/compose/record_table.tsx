import { useCallback, useMemo, useSyncExternalStore, type CSSProperties } from 'react';
import { valueAtPath } from '../../../api/models/sheet';
import type { FieldNode, FieldSchema } from '../../../api/models/template';
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
import { usePresetContext, usePresetValue } from '../preset_context';
import { RollableValue } from '../rollable_value';
import type { PresetRecordTablePiece } from './types';
import { GroupTitle } from './group_title';
import './compose.css';

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

function isSelected(value: unknown): boolean {
  return value === true || (typeof value === 'number' && value !== 0);
}

function useRecordIDs(listPath: string): readonly string[] {
  const { store } = usePresetContext();
  const serialized = useSyncExternalStore(store.subscribe, () =>
    orderedRecords(listPath, store.getState().data)
      .map((entry) => entry.id)
      .join(','),
  );
  return serialized === '' ? [] : serialized.split(',');
}

function usePresetValues(paths: readonly string[]): readonly unknown[] {
  const { store } = usePresetContext();
  const subscribe = useCallback(
    (listener: () => void) => {
      const unsubscribes = paths.map((path) => store.subscribePath(path, listener));
      return () => {
        for (const unsubscribe of unsubscribes) {
          unsubscribe();
        }
      };
    },
    [store, paths],
  );
  const serialized = useSyncExternalStore(subscribe, () =>
    JSON.stringify(paths.map((path) => valueAtPath(store.getState().data, path))),
  );
  return JSON.parse(serialized) as readonly unknown[];
}

function cellClass(column: ListColumn): string {
  return `lg-compose-record__cell${
    column.kind === 'field' && fieldColumnAlign(column.schema) === 'right'
      ? ' lg-compose-record__cell--number'
      : ''
  }`;
}

function columnTrack(column: ListColumn): string {
  if (column.kind !== 'field') {
    return 'minmax(8.5rem, 1.25fr)';
  }
  return fieldColumnAlign(column.schema) === 'right' ? 'minmax(6.5rem, 1fr)' : 'minmax(0, 2fr)';
}

function recordTemplate(columns: readonly ListColumn[]): CSSProperties {
  return { '--lg-record-template': columns.map(columnTrack).join(' ') } as CSSProperties;
}

function FieldCell({ path, schema }: { readonly path: string; readonly schema: FieldSchema }) {
  const { store } = usePresetContext();
  const value = usePresetValue(path);

  return (
    <SheetFieldControl
      path={path}
      schema={schema}
      value={value}
      onChanged={(next) => store.setPath(path, next)}
    />
  );
}

function ExclusiveCell({
  recordKey,
  column,
}: {
  readonly recordKey: string;
  readonly column: Extract<ListColumn, { kind: 'exclusive' }>;
}) {
  const { store } = usePresetContext();
  const options = column.options.map((option) => ({
    path: `${recordKey}.${option.name}`,
    schema: option.schema,
  }));
  const paths = options.map((option) => option.path);
  const values = usePresetValues(paths);
  const selected = options.find((_, index) => isSelected(values[index]))?.path;

  return (
    <SheetExclusiveControl
      group={{ id: column.id, options }}
      selectedPath={selected}
      controlId={`compose-${recordKey.replace(/[^a-zA-Z0-9_-]/g, '-')}-${column.id}`}
      onSelect={(next) =>
        store.setExclusive(
          next === EXCLUSIVE_NONE_VALUE ? null : next,
          options.map((option) => option.path),
        )
      }
    />
  );
}

function RecordCell({
  column,
  recordKey,
}: {
  readonly column: ListColumn;
  readonly recordKey: string;
}) {
  if (column.kind === 'field') {
    return (
      <>
        <span className="lg-compose-record__label">{columnLabel(column)}</span>
        <FieldCell path={`${recordKey}.${column.name}`} schema={column.schema} />
      </>
    );
  }

  return (
    <>
      <span className="lg-compose-record__label">{columnLabel(column)}</span>
      <ExclusiveCell recordKey={recordKey} column={column} />
    </>
  );
}

interface RecordRowProps {
  readonly list: SheetList;
  readonly recordID: string;
  readonly position: number;
  readonly columns: readonly ListColumn[];
  readonly sectionKey: string;
  readonly rowDefault: boolean;
}

function RecordRowView({
  list,
  recordID,
  position,
  columns,
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
        {store.removeRecord === undefined ? null : (
          <Button
            variant="ghost"
            size="sm"
            icon={<CloseIcon />}
            label={messageWith('record.remove', { label })}
            onClick={() => store.removeRecord?.(list.path, recordID)}
          />
        )}
      </span>
      <span className="lg-compose-record__cells">
        {columns.map((column, index) => (
          <span className={cellClass(column)} key={`${column.kind}-${index}`}>
            <RecordCell column={column} recordKey={key} />
          </span>
        ))}
      </span>
    </div>
  );
}

function RecordHeader({ columns }: { readonly columns: readonly ListColumn[] }) {
  return (
    <div className="lg-compose-record__header" aria-hidden="true">
      <span className="lg-compose-record__lead" />
      <span className="lg-compose-record__cells">
        {columns.map((column, index) => (
          <span className={cellClass(column)} key={`${column.kind}-${index}`}>
            <span className="lg-compose-record__label">{columnLabel(column)}</span>
          </span>
        ))}
      </span>
    </div>
  );
}

export function RecordTable({ piece }: { readonly piece: PresetRecordTablePiece }) {
  const { template, store } = usePresetContext();
  const list = useMemo(() => sheetList(template.fields, piece.list), [template, piece.list]);
  const info = useMemo(
    () => listInfos(template.fields).find((entry) => entry.path === piece.list),
    [template, piece.list],
  );
  const recordIDs = useRecordIDs(piece.list);

  if (list === undefined) {
    return null;
  }

  const columns = chosenColumns(list, piece.columns);
  const sectionKey = info?.section ?? sectionKeyOfRow(list.path);
  const rowDefault = info?.dice !== undefined;
  const addDisabledReason =
    recordIDs.length >= list.capacity
      ? messageWith('record.atCapacity', { count: recordIDs.length, capacity: list.capacity })
      : undefined;

  return (
    <div
      className="lg-compose-record"
      role="group"
      aria-label={list.label}
      data-testid="compose-record-table"
      style={recordTemplate(columns)}
    >
      <GroupTitle>{list.label}</GroupTitle>
      {recordIDs.length === 0 ? (
        <p className="lg-compose-record__empty">{message('record.empty')}</p>
      ) : (
        <>
          <RecordHeader columns={columns} />
          {recordIDs.map((recordID, index) => (
            <RecordRowView
              key={recordID}
              list={list}
              recordID={recordID}
              position={index + 1}
              columns={columns}
              sectionKey={sectionKey}
              rowDefault={rowDefault}
            />
          ))}
        </>
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
