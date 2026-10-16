/**
 * Renders one mutually exclusive group as a single dense single-choice control
 * (ENG-14 … ENG-18): the group's members are the options, a "no choice" entry
 * appears only when no member carries a non-zero default, and a default-active
 * group with no active member shows the "no choice" hint instead of rendering
 * blank. Controlled: it owns no state and reports a selection through
 * `onSelect`.
 */

import type { ChangeEvent } from 'react';
import { message } from '../l10n/strings';
import type { SheetExclusiveGroup, SheetField } from './field_grouping';
import { fieldDisplayName, groupDisplayName, lastSegment } from './field_grouping';
import type { SheetMark } from './sheet_mark';
import { FIELD_MARKED_CLASS } from './sheet_mark';
import './engine.css';

/**
 * The sentinel reported when the "no choice" entry is selected; a real dotted
 * member path can never collide with it (ENG-18).
 */
export const EXCLUSIVE_NONE_VALUE = '__none__';

/** A value is active iff it is a non-zero number; the backend contract is numeric 0/1 proxies (ENG-16). */
export function exclusiveValueIsOn(value: unknown): boolean {
  return typeof value === 'number' && value !== 0;
}

export interface SheetExclusiveControlProps {
  /** The group to render: its id and its member leaves in API order (ENG-14). */
  readonly group: SheetExclusiveGroup;
  /** Dotted path of the active member, or `undefined` when no member is active (ENG-16). */
  readonly selectedPath?: string;
  /** The last preview's marking of the group; tints the control (ENG-22). */
  readonly mark?: SheetMark;
  /** Reports a selected member's path or the clear-group signal; absent renders it inert (ENG-17). */
  readonly onSelect?: (signal: string) => void;
}

/** Renders one mutually exclusive group as a single single-choice control (ENG-14 … ENG-18). */
export function SheetExclusiveControl({
  group,
  selectedPath,
  mark,
  onSelect,
}: SheetExclusiveControlProps) {
  const label = groupDisplayName(group.id);
  const allowsNone = group.options.every((option) => option.schema.defaultValue === 0);
  const active =
    selectedPath !== undefined && group.options.some((option) => option.path === selectedPath)
      ? selectedPath
      : undefined;
  const showsHint = active === undefined && !allowsNone;
  const value = active ?? (allowsNone ? EXCLUSIVE_NONE_VALUE : '');

  return (
    <select
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
  );
}

/** The option text: the member's non-empty `optionLabel`, else its display name (ENG-15). */
function optionLabel(option: SheetField): string {
  const label = option.schema.optionLabel?.trim();
  if (label !== undefined && label !== '') {
    return label;
  }
  return fieldDisplayName(lastSegment(option.path), option.schema);
}
