import { useState } from 'react';
import { sectionKeyOfRow } from '../../../engine/field_grouping';
import { message } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { Field } from '../../../ui/field';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import { RollableValue } from '../rollable_value';
import {
  applyDamage,
  applyHeal,
  hitPointValue,
  hitPointsPercent,
  parseHitPointsAmount,
  parseHitPointsValue,
} from './hit_points';
import type { PresetHitPointsPiece } from './types';
import './compose.css';

interface HitPointsTrackBodyProps {
  readonly piece: PresetHitPointsPiece;
  readonly max: number;
  readonly maxPath?: string;
  readonly maxLabel?: string;
  readonly maxEditable: boolean;
  readonly commitMax?: (value: number) => void;
}

function HitPointsTrackBody({
  piece,
  max,
  maxPath,
  maxLabel,
  maxEditable,
  commitMax,
}: HitPointsTrackBodyProps) {
  const current = hitPointValue(usePresetValue(piece.current));
  const currentLabel = usePresetLabel(piece.current);
  const commitCurrent = usePresetCommit(piece.current);
  const tempLabel = usePresetLabel(piece.temp ?? piece.current);
  const tempValue = usePresetValue(piece.temp ?? piece.current);
  const commitTemp = usePresetCommit(piece.temp ?? piece.current);
  const temp = piece.temp === undefined ? 0 : hitPointValue(tempValue);
  const [amountText, setAmountText] = useState('');
  const percent = hitPointsPercent(current, max);
  const meterNow = max > 0 ? Math.min(max, current) : 0;

  const apply = (heal: boolean) => {
    const amount = parseHitPointsAmount(amountText);
    const hitPoints = { current, max, temp };
    const next = heal ? applyHeal(hitPoints, amount) : applyDamage(hitPoints, amount);
    if (next === undefined) {
      return;
    }
    if (piece.temp !== undefined && next.temp !== temp) {
      commitTemp(next.temp);
    }
    if (next.current !== current) {
      commitCurrent(next.current);
    }
    setAmountText('');
  };

  const commitParsed = (commit: (value: number) => void) => (text: string) => {
    const next = parseHitPointsValue(text);
    if (next !== undefined) {
      commit(next);
    }
  };

  return (
    <div className="lg-compose-hp" data-testid="compose-hit-points">
      <p className="lg-compose-hp__numbers">
        <span className="lg-compose-hp__value">
          <InlineEdit
            className="lg-compose-hp__number"
            label={currentLabel}
            path={piece.current}
            value={String(current)}
            inputMode="numeric"
            onCommit={commitParsed(commitCurrent)}
          />
          <span className="lg-compose-hp__slash" aria-hidden="true">
            {' / '}
          </span>
          {maxEditable && maxLabel !== undefined && commitMax !== undefined ? (
            <InlineEdit
              className="lg-compose-hp__number"
              label={maxLabel}
              path={maxPath}
              value={String(max)}
              inputMode="numeric"
              onCommit={commitParsed(commitMax)}
            />
          ) : (
            <span className="lg-compose-hp__max" data-testid="compose-hit-points-max">
              {maxLabel === undefined ? null : (
                <span className="lg-compose-hp__max-label">{maxLabel}</span>
              )}
              {maxPath === undefined || maxLabel === undefined ? (
                <span className="lg-compose-hp__max-value">{String(max)}</span>
              ) : (
                <RollableValue
                  className="lg-compose-hp__max-value"
                  rowKey={maxPath}
                  sectionKey={sectionKeyOfRow(maxPath)}
                  label={maxLabel}
                  text={String(max)}
                />
              )}
            </span>
          )}
        </span>
        {piece.temp === undefined ? null : (
          <span className="lg-compose-hp__temp">
            <span className="lg-compose-hp__temp-label">{tempLabel}</span>
            <InlineEdit
              className="lg-compose-hp__number"
              label={tempLabel}
              path={piece.temp}
              value={String(temp)}
              inputMode="numeric"
              onCommit={commitParsed(commitTemp)}
            />
          </span>
        )}
      </p>
      <div
        className="lg-compose-hp__bar"
        role="meter"
        aria-label={message('sheet.play.hitPoints')}
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={meterNow}
      >
        <span
          className="lg-compose-hp__fill"
          style={{ transform: `scaleX(${percent / 100})` }}
          aria-hidden="true"
        />
      </div>
      <div className="lg-compose-hp__entry">
        <div className="lg-compose-hp__amount">
          <Field
            label={message('sheet.play.amount')}
            inputMode="numeric"
            numeric
            value={amountText}
            onChange={setAmountText}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                apply(false);
              }
            }}
          />
        </div>
        <Button
          variant="danger"
          size="sm"
          label={message('sheet.play.damage')}
          onClick={() => apply(false)}
        />
        <Button
          variant="secondary"
          size="sm"
          label={message('sheet.play.heal')}
          onClick={() => apply(true)}
        />
      </div>
    </div>
  );
}

function HitPointsTrackWithBoundMax({
  piece,
  maxPath,
}: {
  readonly piece: PresetHitPointsPiece;
  readonly maxPath: string;
}) {
  const maxLabel = usePresetLabel(maxPath);
  const maxSchema = usePresetSchema(maxPath);
  const commitMax = usePresetCommit(maxPath);
  const max = hitPointValue(usePresetValue(maxPath));
  const maxEditable = maxSchema?.isEditable === true;

  return (
    <HitPointsTrackBody
      piece={piece}
      max={max}
      maxPath={maxPath}
      maxLabel={maxLabel}
      maxEditable={maxEditable}
      commitMax={commitMax}
    />
  );
}

function HitPointsTrackWithLiteralMax({
  piece,
  max,
}: {
  readonly piece: PresetHitPointsPiece;
  readonly max: number;
}) {
  return <HitPointsTrackBody piece={piece} max={hitPointValue(max)} maxEditable={false} />;
}

export function HitPointsTrack({ piece }: { readonly piece: PresetHitPointsPiece }) {
  if (typeof piece.max === 'number') {
    return <HitPointsTrackWithLiteralMax piece={piece} max={piece.max} />;
  }
  return <HitPointsTrackWithBoundMax piece={piece} maxPath={piece.max} />;
}
