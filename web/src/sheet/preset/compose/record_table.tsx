import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type CSSProperties,
} from 'react';
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
import { ChevronDownIcon, CloseIcon, PlusIcon } from '../../../ui/icons';
import { DieIcon } from '../../die_icon';
import { useExpressionRoll } from '../../dice_row';
import { usePresetContext, usePresetValue } from '../preset_context';
import { numberValue, textValue } from '../values';
import { RollableValue } from '../rollable_value';
import type { PresetRecordTablePiece } from './types';
import { formatAttackModifier, parseDamageExpression } from './record_rolls';
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

function FieldCell({
  path,
  schema,
  live,
}: {
  readonly path: string;
  readonly schema: FieldSchema;
  readonly live: boolean;
}) {
  const { store } = usePresetContext();
  const value = usePresetValue(path);

  return (
    <SheetFieldControl
      path={path}
      schema={schema}
      value={value}
      onChanged={live ? (next) => store.setPath(path, next) : undefined}
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
  const { store, editing } = usePresetContext();
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
      onSelect={
        editing
          ? (next) =>
              store.setExclusive(
                next === EXCLUSIVE_NONE_VALUE ? null : next,
                options.map((option) => option.path),
              )
          : undefined
      }
    />
  );
}

interface AttackColumns {
  readonly name: string;
  readonly roll: string;
  readonly damage: string;
}

function AttackRollValue({
  recordKey,
  rowKey,
  sectionKey,
  attack,
  fallbackName,
  label,
}: {
  readonly recordKey: string;
  readonly rowKey: string;
  readonly sectionKey: string;
  readonly attack: AttackColumns;
  readonly fallbackName: string;
  readonly label: string;
}) {
  const storedName = textValue(usePresetValue(`${recordKey}.${attack.name}`)).trim();
  const modifier = numberValue(usePresetValue(`${recordKey}.${attack.roll}`));
  const name = storedName === '' ? fallbackName : storedName;
  const value =
    modifier === undefined ? message('sheet.play.noValue') : formatAttackModifier(modifier);
  const accessibleName = messageWith('sheet.play.attackRoll', { name, value });

  return (
    <RollableValue
      rowKey={rowKey}
      sectionKey={sectionKey}
      label={label}
      text={value}
      accessibleName={accessibleName}
      icon={<DieIcon shape="d20" />}
      className="lg-compose-record__roll"
    />
  );
}

function DamageValue({
  recordKey,
  rowKey,
  attack,
  fallbackName,
  label,
}: {
  readonly recordKey: string;
  readonly rowKey: string;
  readonly attack: AttackColumns;
  readonly fallbackName: string;
  readonly label: string;
}) {
  const text = textValue(usePresetValue(`${recordKey}.${attack.damage}`));
  const storedName = textValue(usePresetValue(`${recordKey}.${attack.name}`)).trim();
  const attackName = storedName === '' ? fallbackName : storedName;
  const roll = useExpressionRoll(rowKey, label);
  const expression = useMemo(() => parseDamageExpression(text), [text]);
  const damageRollName = messageWith('sheet.play.damageRoll', { name: attackName, value: text });

  if (expression === undefined) {
    return (
      <span className="lg-rollable lg-rollable--still" data-rollable="false">
        {text.trim() === '' ? message('sheet.play.noValue') : text}
      </span>
    );
  }

  return (
    <button
      type="button"
      className="lg-rollable lg-rollable--button lg-compose-record__roll"
      aria-label={damageRollName}
      title={damageRollName}
      onClick={() => roll(expression)}
    >
      {text}
    </button>
  );
}

function RecordCell({
  column,
  recordKey,
  playColumns,
  attack,
  rowKey,
  sectionKey,
  fallbackName,
}: {
  readonly column: ListColumn;
  readonly recordKey: string;
  readonly playColumns: readonly string[];
  readonly attack?: AttackColumns;
  readonly rowKey: string;
  readonly sectionKey: string;
  readonly fallbackName: string;
}) {
  const { editing } = usePresetContext();

  if (column.kind === 'field') {
    const isRoll = attack?.roll === column.name;
    const isDamage = attack?.damage === column.name;
    return (
      <>
        <span className="lg-compose-record__label">{columnLabel(column)}</span>
        <span className="lg-compose-record__control">
          {isRoll && attack !== undefined ? (
            <AttackRollValue
              recordKey={recordKey}
              rowKey={rowKey}
              sectionKey={sectionKey}
              attack={attack}
              fallbackName={fallbackName}
              label={columnLabel(column)}
            />
          ) : isDamage && !editing && attack !== undefined ? (
            <DamageValue
              recordKey={recordKey}
              rowKey={rowKey}
              attack={attack}
              fallbackName={fallbackName}
              label={columnLabel(column)}
            />
          ) : (
            <FieldCell
              path={`${recordKey}.${column.name}`}
              schema={column.schema}
              live={editing || playColumns.includes(column.name)}
            />
          )}
        </span>
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
  readonly count: number;
  readonly playColumns: readonly string[];
  readonly attack?: AttackColumns;
  readonly onMove?: (recordID: string, direction: -1 | 1) => void;
}

function RecordRowView({
  list,
  recordID,
  position,
  columns,
  sectionKey,
  rowDefault,
  count,
  playColumns,
  attack,
  onMove,
}: RecordRowProps) {
  const { store, editing } = usePresetContext();
  const label = messageWith('record.rowLabel', { list: list.label, position });
  const key = recordRowKey(list.path, recordID);
  const attackColumns = rowDefault ? attack : undefined;

  return (
    <div className="lg-compose-record__row" data-testid="compose-record-row" data-record-key={key}>
      <span className="lg-compose-record__lead">
        {rowDefault && attackColumns === undefined ? (
          <RollableValue rowKey={key} sectionKey={sectionKey} label={label} text={label} />
        ) : (
          label
        )}
        {onMove === undefined || count < 2 ? null : (
          <>
            <Button
              variant="ghost"
              size="sm"
              className="lg-compose-record__move lg-compose-record__move--up"
              icon={<ChevronDownIcon />}
              disabled={!editing || position === 1}
              name={messageWith('record.moveUp', { label })}
              onClick={() => onMove(recordID, -1)}
            >
              <span aria-hidden="true" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="lg-compose-record__move lg-compose-record__move--down"
              icon={<ChevronDownIcon />}
              disabled={!editing || position === count}
              name={messageWith('record.moveDown', { label })}
              onClick={() => onMove(recordID, 1)}
            >
              <span aria-hidden="true" />
            </Button>
          </>
        )}
        {store.removeRecord === undefined ? null : (
          <Button
            variant="ghost"
            size="sm"
            className="lg-compose-record__remove"
            disabled={!editing}
            icon={<CloseIcon />}
            label={messageWith('record.remove', { label })}
            onClick={() => store.removeRecord?.(list.path, recordID)}
          />
        )}
      </span>
      <span className="lg-compose-record__cells">
        {columns.map((column, index) => (
          <span className={cellClass(column)} key={`${column.kind}-${index}`}>
            <RecordCell
              column={column}
              recordKey={key}
              playColumns={playColumns}
              attack={attackColumns}
              rowKey={key}
              sectionKey={sectionKey}
              fallbackName={label}
            />
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
  const { template, store, editing } = usePresetContext();
  const list = useMemo(() => sheetList(template.fields, piece.list), [template, piece.list]);
  const info = useMemo(
    () => listInfos(template.fields).find((entry) => entry.path === piece.list),
    [template, piece.list],
  );
  const recordIDs = useRecordIDs(piece.list);
  const root = useRef<HTMLDivElement>(null);
  const pendingFocus = useRef<{ readonly key: string; readonly direction: -1 | 1 } | null>(null);
  const [notice, setNotice] = useState('');
  const move = useCallback(
    (recordID: string, direction: -1 | 1) => {
      const position = store.moveRecord?.(piece.list, recordID, direction);
      if (position !== undefined) {
        pendingFocus.current = { key: recordRowKey(piece.list, recordID), direction };
        setNotice(messageWith('record.moved', { position }));
      }
    },
    [store, piece.list],
  );

  useEffect(() => {
    const target = pendingFocus.current;
    if (target === null) {
      return;
    }
    pendingFocus.current = null;
    const row = root.current?.querySelector(`[data-record-key="${target.key}"]`);
    const [preferred, other] = target.direction === -1 ? ['up', 'down'] : ['down', 'up'];
    const button = (name: string) =>
      row?.querySelector<HTMLButtonElement>(`.lg-compose-record__move--${name}:not(:disabled)`);
    (button(preferred) ?? button(other))?.focus();
  });

  if (list === undefined) {
    return null;
  }

  const columns = chosenColumns(list, piece.columns);
  const sectionKey = info?.section ?? sectionKeyOfRow(list.path);
  const rowDefault = info?.dice !== undefined;
  const attack =
    piece.nameColumn === undefined ||
    piece.rollColumn === undefined ||
    piece.damageColumn === undefined
      ? undefined
      : {
          name: piece.nameColumn,
          roll: piece.rollColumn,
          damage: piece.damageColumn,
        };
  const addDisabledReason =
    recordIDs.length >= list.capacity
      ? messageWith('record.atCapacity', { count: recordIDs.length, capacity: list.capacity })
      : undefined;

  return (
    <div
      ref={root}
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
              count={recordIDs.length}
              playColumns={piece.playColumns ?? []}
              attack={attack}
              onMove={store.moveRecord === undefined ? undefined : move}
            />
          ))}
        </>
      )}
      <p className="lg-visually-hidden" role="status" aria-live="polite">
        {notice}
      </p>
      {store.addRecord === undefined ? null : (
        <div className="lg-compose-record__add">
          <Button
            variant="ghost"
            size="sm"
            disabled={!editing}
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
