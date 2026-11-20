import { useCallback, useState, useSyncExternalStore } from 'react';
import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { Field } from '../../../ui/field';
import { InlineEdit } from '../../inline_edit';
import {
  usePresetCommit,
  usePresetContext,
  usePresetLabel,
  usePresetSchema,
} from '../preset_context';
import {
  applyDamage,
  applyHeal,
  applyTemp,
  formatHitPointsDelta,
  HIT_POINTS_CURRENT,
  HIT_POINTS_MAX,
  HIT_POINTS_TEMP,
  hitPointsBand,
  hitPointsFrom,
  hitPointsPercent,
  hitPointsPreview,
  parseHitPointsAmount,
  parseHitPointsCurrent,
  parseHitPointsMaximum,
  parseHitPointsTemp,
} from './hit_points';
import type { HitPointsChange, HitPointsDirection } from './hit_points';

function withinRange(value: number, max: number): number {
  return Math.min(max, Math.max(0, value));
}

function lineTone(delta: number): HitPointsDirection {
  return delta < 0 ? 'damage' : 'heal';
}

export function HitPointsCard() {
  const { store } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const [amount, setAmount] = useState('');
  const currentLabel = usePresetLabel(HIT_POINTS_CURRENT);
  const maxLabel = usePresetLabel(HIT_POINTS_MAX);
  const tempLabel = usePresetLabel(HIT_POINTS_TEMP);
  const currentSchema = usePresetSchema(HIT_POINTS_CURRENT);
  const maxSchema = usePresetSchema(HIT_POINTS_MAX);
  const tempSchema = usePresetSchema(HIT_POINTS_TEMP);
  const commitCurrent = usePresetCommit(HIT_POINTS_CURRENT);
  const commitMax = usePresetCommit(HIT_POINTS_MAX);
  const commitTemp = usePresetCommit(HIT_POINTS_TEMP);

  const hitPoints = hitPointsFrom(state.data);
  const stored = hitPointsFrom(state.defaults);
  const max = Math.max(0, hitPoints.max);
  const current = withinRange(hitPoints.current, max);
  const storedCurrent = withinRange(stored.current, max);
  const temp = Math.max(0, hitPoints.temp);
  const storedTemp = Math.max(0, stored.temp);
  const points = { current, max, temp };

  const percent = hitPointsPercent(current, max);
  const storedPercent = hitPointsPercent(storedCurrent, max);
  const tempPercent = hitPointsPercent(temp, max);
  const storedTempPercent = hitPointsPercent(storedTemp, max);

  const preview = hitPointsPreview(
    { current: storedCurrent, max, temp: storedTemp },
    { current, max, temp },
  );
  const currentChange = preview?.current;
  const tempChange = preview?.temp;
  const pendingLeft = Math.min(percent, storedPercent);
  const pendingWidth = Math.abs(percent - storedPercent);
  const currentEnd = Math.max(percent, storedPercent);
  const tempEnd = Math.min(100, percent + tempPercent);
  const storedTempEnd = Math.min(100, storedPercent + storedTempPercent);
  const tempPendingLeft = Math.min(
    100,
    Math.max(
      currentEnd,
      Math.min(percent, storedPercent) + Math.min(tempPercent, storedTempPercent),
    ),
  );
  const tempPendingWidth = Math.min(
    100 - tempPendingLeft,
    Math.max(0, Math.max(tempEnd, storedTempEnd) - tempPendingLeft),
  );

  const commit = (change: HitPointsChange | undefined) => {
    if (change === undefined) {
      return;
    }
    store.setPath(HIT_POINTS_CURRENT, change.current);
    store.setPath(HIT_POINTS_TEMP, change.temp);
  };

  const applyAmount = (change: HitPointsChange | undefined) => {
    if (change === undefined) {
      return;
    }
    commit(change);
    setAmount('');
  };

  const amountValue = parseHitPointsAmount(amount);
  const applyDamageAmount = () => {
    applyAmount(amountValue === undefined ? undefined : applyDamage(points, amountValue));
  };
  const applyTempAmount = () => {
    const change = amountValue === undefined ? undefined : applyTemp(points, amountValue);
    if (change === undefined) {
      return;
    }
    store.setPath(HIT_POINTS_TEMP, change.temp);
    setAmount('');
  };

  return (
    <div className="lg-hp-card" data-testid="sheet-hp-card">
      <div className="lg-hp-card__head">
        <p className="lg-hp-card__label">{message('sheet.play.hitPoints')}</p>
        <p className="lg-hp-card__numbers">
          <span className="lg-hp-card__value" data-testid="sheet-hp-value">
            <InlineEdit
              className="lg-hp-card__number"
              label={currentLabel}
              path={HIT_POINTS_CURRENT}
              schema={currentSchema}
              value={String(current)}
              inputMode="numeric"
              onCommit={(text) => {
                const next = parseHitPointsCurrent(text);
                if (next !== undefined) {
                  commitCurrent(withinRange(next, max));
                }
              }}
            />
            <span className="lg-hp-card__slash" aria-hidden="true">
              {' / '}
            </span>
            <InlineEdit
              className="lg-hp-card__number"
              label={maxLabel}
              path={HIT_POINTS_MAX}
              schema={maxSchema}
              value={String(hitPoints.max)}
              inputMode="numeric"
              onCommit={(text) => {
                const next = parseHitPointsMaximum(text);
                if (next !== undefined) {
                  commitMax(next);
                  const clamped = withinRange(hitPoints.current, next);
                  if (clamped !== hitPoints.current) {
                    commitCurrent(clamped);
                  }
                }
              }}
            />
          </span>
          {temp > 0 ? (
            <span className="lg-hp-card__temp-value" data-testid="sheet-hp-temp-value">
              <InlineEdit
                className="lg-hp-card__number lg-hp-card__number--temp"
                label={tempLabel}
                path={HIT_POINTS_TEMP}
                schema={tempSchema}
                value={String(temp)}
                display={messageWith('sheet.play.tempValue', { count: temp })}
                inputMode="numeric"
                onCommit={(text) => {
                  const next = parseHitPointsTemp(text);
                  if (next !== undefined) {
                    commitTemp(next);
                  }
                }}
              />
            </span>
          ) : (
            <InlineEdit
              className="lg-hp-card__temp-add"
              label={tempLabel}
              path={HIT_POINTS_TEMP}
              schema={tempSchema}
              value=""
              fallback={message('sheet.play.tempShort')}
              inputMode="numeric"
              onCommit={(text) => {
                const next = parseHitPointsTemp(text);
                if (next !== undefined) {
                  commitTemp(next);
                }
              }}
            />
          )}
        </p>
      </div>
      <div
        className="lg-hp-card__bar"
        data-testid="sheet-hp-meter"
        data-band={hitPointsBand(percent)}
        role="meter"
        aria-label={message('sheet.play.hitPoints')}
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={current}
      >
        <span className="lg-hp-card__fill" style={{ width: `${percent}%` }} aria-hidden="true" />
        {temp > 0 && (
          <span
            className="lg-hp-card__temp"
            data-testid="sheet-hp-temp"
            style={{ left: `${percent}%`, width: `${Math.min(tempPercent, 100 - percent)}%` }}
            aria-hidden="true"
          />
        )}
        {currentChange !== undefined && pendingWidth > 0 && (
          <span
            className="lg-hp-card__pending"
            data-testid="sheet-hp-pending"
            data-tone={lineTone(currentChange.delta)}
            style={{ left: `${pendingLeft}%`, width: `${pendingWidth}%` }}
            aria-hidden="true"
          />
        )}
        {tempChange !== undefined && tempPendingWidth > 0 && (
          <span
            className="lg-hp-card__pending lg-hp-card__pending--temp"
            data-testid="sheet-hp-temp-pending"
            data-tone={lineTone(tempChange.delta)}
            style={{ left: `${tempPendingLeft}%`, width: `${tempPendingWidth}%` }}
            aria-hidden="true"
          />
        )}
      </div>
      {preview !== undefined && (
        <p className="lg-hp-card__pending-notes" data-direction={preview.direction}>
          {currentChange !== undefined && (
            <span
              className="lg-hp-card__change"
              data-tone={lineTone(currentChange.delta)}
              data-testid="sheet-hp-change"
            >
              {messageWith('sheet.play.change', {
                before: currentChange.before,
                after: currentChange.after,
                delta: formatHitPointsDelta(currentChange.delta),
              })}
            </span>
          )}
          {tempChange !== undefined && (
            <span
              className="lg-hp-card__change"
              data-tone={lineTone(tempChange.delta)}
              data-testid="sheet-hp-temp-change"
            >
              {messageWith('sheet.play.tempChange', {
                before: tempChange.before,
                after: tempChange.after,
                delta: formatHitPointsDelta(tempChange.delta),
              })}
            </span>
          )}
        </p>
      )}
      <div className="lg-hp-card__actions">
        <div className="lg-hp-card__quick">
          <Button
            variant="secondary"
            size="sm"
            className="lg-hp-card__quick-damage"
            name={messageWith('sheet.play.damageAmount', { count: 5 })}
            onClick={() => commit(applyDamage(points, 5))}
          >
            {message('sheet.play.damage5Short')}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            className="lg-hp-card__quick-damage"
            name={messageWith('sheet.play.damageAmount', { count: 1 })}
            onClick={() => commit(applyDamage(points, 1))}
          >
            {message('sheet.play.damage1Short')}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            className="lg-hp-card__quick-heal"
            name={messageWith('sheet.play.healAmount', { count: 1 })}
            onClick={() => commit(applyHeal(points, 1))}
          >
            {message('sheet.play.heal1Short')}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            className="lg-hp-card__quick-heal"
            name={messageWith('sheet.play.healAmount', { count: 5 })}
            onClick={() => commit(applyHeal(points, 5))}
          >
            {message('sheet.play.heal5Short')}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            name={message('sheet.play.addTemp')}
            disabled={amountValue === undefined}
            onClick={applyTempAmount}
          >
            {message('sheet.play.tempAddShort')}
          </Button>
        </div>
        <div className="lg-hp-card__entry">
          <div className="lg-hp-card__amount">
            <Field
              label={message('sheet.play.amount')}
              type="number"
              inputMode="numeric"
              numeric
              value={amount}
              onChange={setAmount}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  applyDamageAmount();
                }
              }}
            />
          </div>
          <Button
            variant="danger"
            size="sm"
            label={message('sheet.play.damage')}
            onClick={applyDamageAmount}
          />
          <Button
            variant="secondary"
            size="sm"
            label={message('sheet.play.heal')}
            onClick={() =>
              applyAmount(amountValue === undefined ? undefined : applyHeal(points, amountValue))
            }
          />
        </div>
      </div>
    </div>
  );
}
