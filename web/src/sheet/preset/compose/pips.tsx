import { messageWith } from '../../../l10n/strings';
import { usePresetCommit, usePresetLabel, usePresetValue } from '../preset_context';
import { numberValue } from '../values';
import type { PresetPipsPiece } from './types';
import './compose.css';

interface PipsBodyProps {
  readonly countPath: string;
  readonly max: number;
}

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

function PipsBody({ countPath, max }: PipsBodyProps) {
  const label = usePresetLabel(countPath);
  const value = usePresetValue(countPath);
  const commit = usePresetCommit(countPath);
  const limit = pipLimit(max);
  const count = pipCount(value, limit);
  const indexes = Array.from({ length: limit }, (_, index) => index + 1);

  return (
    <div className="lg-compose-pips">
      <span className="lg-compose-pips__label">{label}</span>
      {limit === 0 ? null : (
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
              aria-label={messageWith('preset.pips.set', { label, count: index })}
              onClick={() => commit(index <= count ? index - 1 : index)}
            >
              <span className="lg-compose-pip__mark" aria-hidden="true" />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function PipsWithMaxPath({
  countPath,
  maxPath,
}: {
  readonly countPath: string;
  readonly maxPath: string;
}) {
  const max = numberValue(usePresetValue(maxPath)) ?? 0;

  return <PipsBody countPath={countPath} max={max} />;
}

export function Pips({ piece }: { readonly piece: PresetPipsPiece }) {
  if (typeof piece.max === 'number') {
    return <PipsBody countPath={piece.count} max={piece.max} />;
  }
  return <PipsWithMaxPath countPath={piece.count} maxPath={piece.max} />;
}
