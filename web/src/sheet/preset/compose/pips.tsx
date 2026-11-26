import { message, messageWith } from '../../../l10n/strings';
import { usePresetCommit, usePresetLabel, usePresetValue } from '../preset_context';
import { numberValue } from '../values';
import { GroupTitle } from './group_title';
import type { PresetPipsPiece } from './types';
import './compose.css';

interface PipsBodyProps {
  readonly countPath: string;
  readonly max: number;
  readonly label?: string;
  readonly decoration?: PipsDecoration;
}

type PipsDecoration = Pick<PresetPipsPiece, 'heading' | 'label' | 'critical'>;

function pipLimit(max: number): number {
  if (!Number.isFinite(max)) {
    return 0;
  }
  return Math.max(0, Math.trunc(max));
}

function pipCount(value: unknown, limit: number): number {
  const count = numberValue(value);
  if (count === undefined) {
    return 0;
  }
  return Math.min(limit, Math.max(0, Math.trunc(count)));
}

function PipsBody({ countPath, max, label: shownLabel, decoration }: PipsBodyProps) {
  const fieldLabel = usePresetLabel(countPath);
  const label = shownLabel ?? fieldLabel;
  const value = usePresetValue(countPath);
  const commit = usePresetCommit(countPath);
  const limit = pipLimit(max);
  const count = pipCount(value, limit);
  const indexes = Array.from({ length: limit }, (_, index) => index + 1);

  if (limit === 0) {
    return null;
  }

  return (
    <div className={`lg-compose-pips${decoration?.critical ? ' lg-compose-pips--critical' : ''}`}>
      {decoration?.heading === undefined ? null : (
        <div className="lg-compose-pips__heading">
          <GroupTitle>{message(decoration.heading)}</GroupTitle>
        </div>
      )}
      <span className="lg-compose-pips__label">
        {decoration?.label === undefined ? label : message(decoration.label)}
      </span>
      <div
        className="lg-compose-pips__row"
        role="group"
        aria-label={messageWith('preset.pips.group', { label, count, max: limit })}
      >
        {indexes.map((index) => (
          <button
            key={index}
            type="button"
            className="lg-compose-pip"
            aria-pressed={index <= count}
            aria-label={
              index <= count
                ? messageWith('preset.pips.clear', { label, count: index - 1 })
                : messageWith('preset.pips.set', { label, count: index })
            }
            onClick={() => commit(index <= count ? index - 1 : index)}
          >
            <span className="lg-compose-pip__mark" aria-hidden="true" />
          </button>
        ))}
      </div>
      <span className="lg-compose-pips__count" aria-hidden="true">
        {messageWith('preset.pips.count', { count, max: limit })}
      </span>
    </div>
  );
}

export function PipsWithMaxPath({
  countPath,
  maxPath,
  label,
  decoration,
}: {
  readonly countPath: string;
  readonly maxPath: string;
  readonly label?: string;
  readonly decoration?: PipsDecoration;
}) {
  const max = numberValue(usePresetValue(maxPath)) ?? 0;

  return <PipsBody countPath={countPath} max={max} label={label} decoration={decoration} />;
}

export function Pips({ piece }: { readonly piece: PresetPipsPiece }) {
  if (typeof piece.max === 'number') {
    return <PipsBody countPath={piece.count} max={piece.max} decoration={piece} />;
  }
  return <PipsWithMaxPath countPath={piece.count} maxPath={piece.max} decoration={piece} />;
}
