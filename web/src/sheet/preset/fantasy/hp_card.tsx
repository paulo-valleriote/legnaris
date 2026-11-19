import { useCallback, useState, useSyncExternalStore } from 'react';
import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { Field } from '../../../ui/field';
import { usePresetContext } from '../preset_context';
import {
  applyDamage,
  applyHeal,
  HIT_POINTS_CURRENT,
  HIT_POINTS_TEMP,
  hitPointsBand,
  hitPointsFrom,
  hitPointsPercent,
  parseHitPointsAmount,
} from './hit_points';
import type { HitPointsChange } from './hit_points';

function withinRange(value: number, max: number): number {
  return Math.min(max, Math.max(0, value));
}

export function HitPointsCard() {
  const { store } = usePresetContext();
  const subscribe = useCallback((listener: () => void) => store.subscribe(listener), [store]);
  const state = useSyncExternalStore(subscribe, store.getState);
  const [amount, setAmount] = useState('');

  const hitPoints = hitPointsFrom(state.data);
  const stored = hitPointsFrom(state.defaults);
  const max = Math.max(0, hitPoints.max);
  const current = withinRange(hitPoints.current, max);
  const storedCurrent = withinRange(stored.current, max);
  const temp = Math.max(0, hitPoints.temp);
  const storedTemp = Math.max(0, stored.temp);

  const percent = hitPointsPercent(current, max);
  const storedPercent = hitPointsPercent(storedCurrent, max);
  const tempPercent = hitPointsPercent(temp, max);
  const storedTempPercent = hitPointsPercent(storedTemp, max);

  const currentPending = current !== storedCurrent;
  const tempPending = temp !== storedTemp;
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
    setAmount('');
  };

  const amountValue = parseHitPointsAmount(amount);

  return (
    <div className="lg-hp-card" data-testid="sheet-hp-card">
      <div className="lg-hp-card__head">
        <p className="lg-hp-card__label">{message('sheet.play.hitPoints')}</p>
        <p className="lg-hp-card__numbers">
          <span className="lg-hp-card__value" data-testid="sheet-hp-value">
            {messageWith('sheet.play.hitPointsValue', { count: current, max })}
          </span>
          {temp > 0 && (
            <span className="lg-hp-card__temp-value" data-testid="sheet-hp-temp-value">
              {messageWith('sheet.play.tempValue', { count: temp })}
            </span>
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
        {currentPending && pendingWidth > 0 && (
          <span
            className="lg-hp-card__pending"
            data-testid="sheet-hp-pending"
            style={{ left: `${pendingLeft}%`, width: `${pendingWidth}%` }}
            aria-hidden="true"
          />
        )}
        {tempPending && tempPendingWidth > 0 && (
          <span
            className="lg-hp-card__pending lg-hp-card__pending--temp"
            data-testid="sheet-hp-temp-pending"
            style={{ left: `${tempPendingLeft}%`, width: `${tempPendingWidth}%` }}
            aria-hidden="true"
          />
        )}
      </div>
      {(currentPending || tempPending) && (
        <p className="lg-hp-card__pending-notes">
          {currentPending && (
            <span className="lg-hp-card__was" data-testid="sheet-hp-was">
              {messageWith('sheet.play.was', { count: storedCurrent })}
            </span>
          )}
          {tempPending && (
            <span className="lg-hp-card__was" data-testid="sheet-hp-temp-was">
              {messageWith('sheet.play.tempWas', { count: storedTemp })}
            </span>
          )}
        </p>
      )}
      <div className="lg-hp-card__actions">
        <div className="lg-hp-card__amount">
          <Field
            label={message('sheet.play.amount')}
            type="number"
            inputMode="decimal"
            numeric
            value={amount}
            onChange={setAmount}
          />
        </div>
        <Button
          variant="danger"
          size="sm"
          label={message('sheet.play.damage')}
          onClick={() =>
            commit(amountValue === undefined ? undefined : applyDamage(hitPoints, amountValue))
          }
        />
        <Button
          variant="secondary"
          size="sm"
          label={message('sheet.play.heal')}
          onClick={() =>
            commit(amountValue === undefined ? undefined : applyHeal(hitPoints, amountValue))
          }
        />
      </div>
    </div>
  );
}
