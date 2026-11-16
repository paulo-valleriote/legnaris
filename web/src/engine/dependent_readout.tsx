import { Fragment } from 'react';
import type { ReactNode } from 'react';
import { message } from '../l10n/strings';
import type { SheetField } from './field_grouping';

export interface DependentReadoutProps {
  readonly label: string;
  readonly value: unknown;
  readonly stale: boolean;
}

function readoutText(value: unknown): string {
  return value === null || value === undefined ? message('engine.notSet') : String(value);
}

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

export interface DependentListProps {
  readonly id: string;
  readonly source: SheetField;
  readonly dependents: readonly SheetField[];
  readonly build: (dependent: SheetField, source: SheetField) => ReactNode;
}

export function DependentList({ id, source, dependents, build }: DependentListProps) {
  return (
    <span className="lg-dependents" id={id}>
      {dependents.map((dependent) => (
        <Fragment key={dependent.path}>{build(dependent, source)} </Fragment>
      ))}
    </span>
  );
}
