import { useEffect, useState } from 'react';
import { sectionKeyOfRow } from '../../../engine/field_grouping';
import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { Field } from '../../../ui/field';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetContext,
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

interface HitPointsUndo {
  readonly current: number;
  readonly temp: number | undefined;
  readonly amount: number;
  readonly healed: boolean;
}

const UNDO_TIMEOUT_MS = 8000;

const QUICK_CHANGES = [
  { short: 'sheet.play.damage5Short', amount: 5, heal: false },
  { short: 'sheet.play.damage1Short', amount: 1, heal: false },
  { short: 'sheet.play.heal1Short', amount: 1, heal: true },
  { short: 'sheet.play.heal5Short', amount: 5, heal: true },
] as const;

function HitPointsTrackBody({
  piece,
  max,
  maxPath,
  maxLabel,
  maxEditable,
  commitMax,
}: HitPointsTrackBodyProps) {
  const { editing } = usePresetContext();
  const current = hitPointValue(usePresetValue(piece.current));
  const currentLabel = usePresetLabel(piece.current);
  const commitCurrent = usePresetCommit(piece.current);
  const tempLabel = usePresetLabel(piece.temp ?? piece.current);
  const tempValue = usePresetValue(piece.temp ?? piece.current);
  const commitTemp = usePresetCommit(piece.temp ?? piece.current);
  const temp = piece.temp === undefined ? 0 : hitPointValue(tempValue);
  const [amountText, setAmountText] = useState('');
  const [undo, setUndo] = useState<HitPointsUndo>();
  const amountReady = parseHitPointsAmount(amountText) !== undefined;
  const percent = hitPointsPercent(current, max);
  const meterNow = max > 0 ? Math.min(max, current) : 0;

  useEffect(() => {
    if (undo === undefined) {
      return;
    }
    const timer = window.setTimeout(() => setUndo(undefined), UNDO_TIMEOUT_MS);
    return () => window.clearTimeout(timer);
  }, [undo]);

  const apply = (heal: boolean, amount: number | undefined) => {
    if (amount === undefined) {
      return false;
    }
    const hitPoints = { current, max, temp };
    const next = heal ? applyHeal(hitPoints, amount) : applyDamage(hitPoints, amount);
    if (next === undefined) {
      return false;
    }
    const tempChanged = piece.temp !== undefined && next.temp !== temp;
    if (tempChanged) {
      commitTemp(next.temp);
    }
    const currentChanged = next.current !== current;
    if (currentChanged) {
      commitCurrent(next.current);
    }
    if (!tempChanged && !currentChanged) {
      return true;
    }
    setUndo({
      current,
      temp: piece.temp === undefined ? undefined : temp,
      amount,
      healed: heal,
    });
    return true;
  };

  const applyTyped = (heal: boolean) => {
    if (apply(heal, parseHitPointsAmount(amountText))) {
      setAmountText('');
    }
  };

  const down = piece.downNotice !== undefined && max > 0 && current === 0;

  const undoApply = () => {
    if (undo === undefined) {
      return;
    }
    commitCurrent(undo.current);
    if (undo.temp !== undefined) {
      commitTemp(undo.temp);
    }
    setUndo(undefined);
  };

  const commitParsed = (commit: (value: number) => void) => (text: string) => {
    const next = parseHitPointsValue(text);
    if (next !== undefined) {
      commit(next);
    }
  };

  return (
    <div
      className={`lg-compose-hp${down ? ' lg-compose-hp--down' : ''}`}
      data-testid="compose-hit-points"
    >
      <div className="lg-compose-hp__top">
        <div className="lg-compose-hp__readout">
          <span className="lg-compose-hp__title" aria-hidden="true">
            {message('sheet.play.hitPoints')}
          </span>
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
                  className="lg-compose-hp__number lg-compose-hp__number--max"
                  label={maxLabel}
                  path={maxPath}
                  locked={!editing}
                  value={String(max)}
                  inputMode="numeric"
                  onCommit={commitParsed(commitMax)}
                />
              ) : (
                <span className="lg-compose-hp__max" data-testid="compose-hit-points-max">
                  {maxLabel === undefined ? null : (
                    <span className="lg-visually-hidden">{maxLabel}</span>
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
          </p>
        </div>
        <div className="lg-compose-hp__entry">
          <div className="lg-compose-hp__amount">
            <Field
              label={message('sheet.play.amount')}
              placeholder={message('sheet.play.amountPlaceholder')}
              inputMode="numeric"
              numeric
              value={amountText}
              onChange={setAmountText}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  applyTyped(false);
                }
              }}
            />
          </div>
          <Button
            variant="danger"
            size="sm"
            label={message('sheet.play.damage')}
            disabled={!amountReady}
            onClick={() => applyTyped(false)}
          />
          <Button
            variant="secondary"
            size="sm"
            label={message('sheet.play.heal')}
            className="lg-compose-hp__heal"
            disabled={!amountReady}
            onClick={() => applyTyped(true)}
          />
        </div>
      </div>
      <div
        className="lg-compose-hp__quick"
        role="group"
        aria-label={message('sheet.play.damageOrHeal')}
      >
        {QUICK_CHANGES.map((change) => (
          <Button
            key={change.short}
            variant="outline"
            size="sm"
            className={`lg-compose-hp__chip lg-compose-hp__chip--${change.heal ? 'heal' : 'damage'}`}
            name={messageWith(change.heal ? 'sheet.play.healAmount' : 'sheet.play.damageAmount', {
              count: change.amount,
            })}
            onClick={() => apply(change.heal, change.amount)}
          >
            {message(change.short)}
          </Button>
        ))}
      </div>
      <p className="lg-compose-hp__down" role="status">
        {down && piece.downNotice !== undefined ? message(piece.downNotice) : ''}
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
      <div className="lg-compose-hp__foot">
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
        <div className="lg-compose-hp__undo" aria-live="polite">
          {undo === undefined ? null : (
            <>
              <span>
                {messageWith(undo.healed ? 'sheet.play.healed' : 'sheet.play.tookDamage', {
                  count: undo.amount,
                })}
              </span>
              <Button
                variant="ghost"
                size="sm"
                label={message('sheet.play.hpUndo')}
                onClick={undoApply}
              />
            </>
          )}
        </div>
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
