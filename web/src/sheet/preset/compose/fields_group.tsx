import { useCallback, useId, useSyncExternalStore } from 'react';
import { valueAtPath } from '../../../api/models/sheet';
import {
  EXCLUSIVE_NONE_VALUE,
  exclusiveValueIsOn,
  SheetExclusiveControl,
} from '../../../engine/exclusive_control';
import { SheetFieldControl } from '../../../engine/field_control';
import type { SheetField } from '../../../engine/field_grouping';
import { fieldDisplayName, lastSegment } from '../../../engine/field_grouping';
import { message } from '../../../l10n/strings';
import { ChevronDownIcon } from '../../../ui/icons';
import { usePresetContext, usePresetLabel, usePresetValue } from '../preset_context';
import type { PresetFieldsPiece } from './types';
import './compose.css';

type FieldsRow =
  | { readonly kind: 'field'; readonly field: SheetField }
  | { readonly kind: 'exclusive'; readonly id: string; readonly options: SheetField[] };

function parentPath(path: string): string {
  return path.slice(0, path.lastIndexOf('.'));
}

export function fieldsRows(
  paths: readonly string[],
  fields: ReadonlyMap<string, SheetField>,
): readonly FieldsRow[] {
  const rows: FieldsRow[] = [];
  const groups = new Map<string, SheetField[]>();
  for (const path of paths) {
    const field = fields.get(path);
    if (field === undefined) {
      continue;
    }
    const group = field.schema.exclusiveGroup;
    if (group === undefined || group === '') {
      rows.push({ kind: 'field', field });
      continue;
    }
    const id = `${parentPath(path)}#${group}`;
    const options = groups.get(id);
    if (options === undefined) {
      const created = [field];
      groups.set(id, created);
      rows.push({ kind: 'exclusive', id: group, options: created });
    } else {
      options.push(field);
    }
  }
  return rows;
}

function FieldRow({ field }: { readonly field: SheetField }) {
  const { store, editing } = usePresetContext();
  const value = usePresetValue(field.path);
  const labelId = useId();

  return (
    <li
      className={`lg-compose-fields__row${
        field.schema.isEditable ? '' : ' lg-compose-fields__row--derived'
      }`}
      data-field-path={field.path}
    >
      <span className="lg-compose-fields__label" id={labelId}>
        {fieldDisplayName(lastSegment(field.path), field.schema)}
      </span>
      <span className="lg-compose-fields__control">
        <SheetFieldControl
          path={field.path}
          schema={field.schema}
          value={value}
          labelId={labelId}
          onChanged={editing ? (next) => store.setPath(field.path, next) : undefined}
        />
      </span>
    </li>
  );
}

function ExclusiveRow({
  id,
  options,
}: {
  readonly id: string;
  readonly options: readonly SheetField[];
}) {
  const { store, editing } = usePresetContext();
  const controlId = useId();
  const paths = options.map((option) => option.path);
  const pathsKey = paths.join('\u0000');
  const subscribe = useCallback(
    (listener: () => void) => {
      const unsubscribes = pathsKey
        .split('\u0000')
        .map((path) => store.subscribePath(path, listener));
      return () => {
        for (const unsubscribe of unsubscribes) {
          unsubscribe();
        }
      };
    },
    [store, pathsKey],
  );
  const selected = useSyncExternalStore(subscribe, () =>
    paths.find((path) => exclusiveValueIsOn(valueAtPath(store.getState().data, path))),
  );
  const label = options[0]?.schema.label;
  return (
    <li className="lg-compose-fields__row" data-field-path={paths.join(' ')}>
      <label className="lg-compose-fields__label" htmlFor={controlId}>
        {label}
      </label>
      <span className="lg-compose-fields__control">
        <SheetExclusiveControl
          group={{ id, options }}
          label={label}
          selectedPath={selected}
          controlId={controlId}
          onSelect={
            editing
              ? (next) => store.setExclusive(next === EXCLUSIVE_NONE_VALUE ? null : next, paths)
              : undefined
          }
        />
      </span>
    </li>
  );
}

function SummaryValue({ path }: { readonly path: string }) {
  const label = usePresetLabel(path);
  const value = usePresetValue(path);

  return (
    <span className="lg-compose-fields__summary">
      {label}{' '}
      <strong>
        {typeof value === 'number' || typeof value === 'string'
          ? value
          : message('sheet.play.noValue')}
      </strong>
    </span>
  );
}

export function FieldsGroup({ piece }: { readonly piece: PresetFieldsPiece }) {
  const { fields } = usePresetContext();
  const rows = fieldsRows(
    piece.fields.map((entry) => entry.path),
    fields,
  );

  if (rows.length === 0) {
    return null;
  }

  const list = (
    <ul className="lg-compose-fields__list">
      {rows.map((row) =>
        row.kind === 'field' ? (
          <FieldRow key={row.field.path} field={row.field} />
        ) : (
          <ExclusiveRow key={row.options[0]?.path} id={row.id} options={row.options} />
        ),
      )}
    </ul>
  );

  return (
    <details
      className="lg-compose-fields lg-compose-fields--disclosure"
      data-testid={`compose-fields-${piece.key}`}
    >
      <summary className="lg-compose-fields__toggle">
        <ChevronDownIcon className="lg-compose-fields__chevron" />
        <span className="lg-compose-fields__title">{message(piece.title)}</span>
        {piece.summary === undefined ? null : <SummaryValue path={piece.summary} />}
      </summary>
      {list}
    </details>
  );
}
