/**
 * The derived value drawn beside the input that drives it (sheet-object brief, A12): a short label
 * and the value, dimmed while the input has been edited and no Preview has recalculated it. It is
 * a plain description, never a live region: a Preview recalculates dozens of values at once and the
 * sheet footer owns the one announcement of that cascade. The control it belongs to points at the
 * list through `aria-describedby`, so a screen reader hears the label, the value and, when it
 * applies, that it is not recalculated yet. The parts are separated by real spaces, not by layout,
 * so the description reads as words in every engine.
 */

import { Fragment } from 'react';
import type { ReactNode } from 'react';
import { message } from '../l10n/strings';
import type { SheetField } from './field_grouping';

/** The props of {@link DependentReadout}. */
export interface DependentReadoutProps {
  /** What the value is called; the derived leaf's full label. */
  readonly label: string;
  /** The stored value of the derived leaf. */
  readonly value: unknown;
  /** True while the value predates the input's latest edit. */
  readonly stale: boolean;
}

function readoutText(value: unknown): string {
  return value === null || value === undefined ? message('engine.notSet') : String(value);
}

/** One derived value: its label, its value and, while stale, a visually hidden state and a dimmed look. */
export function DependentReadout({ label, value, stale }: DependentReadoutProps) {
  return (
    <span
      className={`lg-dependent${stale ? ' lg-dependent--stale' : ''}`}
      data-stale={stale ? 'true' : undefined}
    >
      <span className="lg-dependent__label" title={label}>
        {label}
      </span>{' '}
      <span className="lg-dependent__value">{readoutText(value)}</span>{' '}
      {stale && <span className="lg-dependent__state">{message('engine.dependentStale')}</span>}
    </span>
  );
}

/** The props of {@link DependentList}. */
export interface DependentListProps {
  /** The id the input's `aria-describedby` names. */
  readonly id: string;
  readonly source: SheetField;
  readonly dependents: readonly SheetField[];
  /** Draws one dependent; the caller owns the value subscription and the stale decision. */
  readonly build: (dependent: SheetField, source: SheetField) => ReactNode;
}

/** The container of the readouts beside one input; it holds the id the input is described by. */
export function DependentList({ id, source, dependents, build }: DependentListProps) {
  return (
    <span className="lg-dependents" id={id}>
      {dependents.map((dependent) => (
        <Fragment key={dependent.path}>{build(dependent, source)} </Fragment>
      ))}
    </span>
  );
}
