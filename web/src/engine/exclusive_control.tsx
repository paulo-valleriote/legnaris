import type { ChangeEvent } from 'react';
import { message } from '../l10n/strings';
import type { SheetExclusiveGroup, SheetField } from './field_grouping';
import { fieldDisplayName, groupDisplayName, lastSegment } from './field_grouping';
import type { SheetMark } from './sheet_mark';
import { FIELD_MARKED_CLASS } from './sheet_mark';
import './engine.css';

export const EXCLUSIVE_NONE_VALUE = '__none__';

export function exclusiveValueIsOn(value: unknown): boolean {
  return typeof value === 'number' && value !== 0;
}

export interface SheetExclusiveControlProps {
  readonly group: SheetExclusiveGroup;
  readonly selectedPath?: string;
  readonly mark?: SheetMark;
  readonly controlId?: string;
  readonly onSelect?: (signal: string) => void;
  readonly label?: string;
}

export function SheetExclusiveControl({
  group,
  selectedPath,
  mark,
  controlId,
  onSelect,
  label: shownLabel,
}: SheetExclusiveControlProps) {
  const label = shownLabel ?? groupDisplayName(group.id);
  const allowsNone = group.options.every((option) => option.schema.defaultValue === 0);
  const active =
    selectedPath !== undefined && group.options.some((option) => option.path === selectedPath)
      ? selectedPath
      : undefined;
  const showsHint = active === undefined && !allowsNone;
  const value = active ?? (allowsNone ? EXCLUSIVE_NONE_VALUE : '');
  const id = controlId ?? `lg-exclusive-${label}`;

  return (
    <label htmlFor={id} className="lg-field__label-wrapper">
      <select
        id={id}
        className={`lg-field${mark !== undefined ? ` ${FIELD_MARKED_CLASS}` : ''}`}
        aria-label={label}
        value={value}
        disabled={onSelect === undefined}
        onChange={(event: ChangeEvent<HTMLSelectElement>) => onSelect?.(event.target.value)}
      >
        {showsHint && (
          <option value="" disabled>
            {message('engine.none')}
          </option>
        )}
        {allowsNone && <option value={EXCLUSIVE_NONE_VALUE}>{message('engine.none')}</option>}
        {group.options.map((option) => (
          <option key={option.path} value={option.path}>
            {optionLabel(option)}
          </option>
        ))}
      </select>
    </label>
  );
}

function optionLabel(option: SheetField): string {
  const label = option.schema.optionLabel?.trim();
  if (label !== undefined && label !== '') {
    return label;
  }
  return fieldDisplayName(lastSegment(option.path), option.schema);
}
