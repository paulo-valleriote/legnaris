import { useCallback, useState, useSyncExternalStore } from 'react';
import { usePresetContext } from '../preset_context';
import type {
  HitPoints,
  HitPointsBand,
  HitPointsChange,
  HitPointsDirection,
  HitPointsPreview,
  HitPointsPreviewLine,
} from './hit_points';
import {
  HIT_POINTS_CURRENT,
  HIT_POINTS_TEMP,
  applyDamage,
  applyHeal,
  applyTemp,
  hitPointsBand,
  hitPointsFrom,
  hitPointsPercent,
  hitPointsPreview,
  parseHitPointsAmount,
} from './hit_points';

export function withinRange(value: number, max: number): number {
  return Math.min(max, Math.max(0, value));
}

export function lineTone(delta: number): HitPointsDirection {
  return delta < 0 ? 'damage' : 'heal';
}

export interface HitPointsView {
  readonly points: HitPoints;
  readonly current: number;
  readonly currentValue: number;
  readonly max: number;
  readonly maxValue: number;
  readonly temp: number;
  readonly band: HitPointsBand;
  readonly percent: number;
  readonly tempPercent: number;
  readonly preview: HitPointsPreview | undefined;
  readonly currentChange: HitPointsPreviewLine | undefined;
  readonly tempChange: HitPointsPreviewLine | undefined;
  readonly pendingLeft: number;
  readonly pendingWidth: number;
  readonly tempPendingLeft: number;
  readonly tempPendingWidth: number;
  readonly amount: string;
  readonly setAmount: (value: string) => void;
  readonly amountValue: number | undefined;
  readonly commit: (change: HitPointsChange | undefined) => void;
  readonly applyDamageAmount: () => void;
  readonly applyHealAmount: () => void;
  readonly applyTempAmount: () => void;
}

export function useHitPointsView(): HitPointsView {
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
  const applyAmountWith =
    (apply: (points: HitPoints, value: number) => HitPointsChange | undefined) => () => {
      applyAmount(amountValue === undefined ? undefined : apply(points, amountValue));
    };
  const applyDamageAmount = applyAmountWith(applyDamage);
  const applyHealAmount = applyAmountWith(applyHeal);
  const applyTempAmount = () => {
    const change = amountValue === undefined ? undefined : applyTemp(points, amountValue);
    if (change === undefined) {
      return;
    }
    store.setPath(HIT_POINTS_TEMP, change.temp);
    setAmount('');
  };

  return {
    points,
    current,
    currentValue: hitPoints.current,
    max,
    maxValue: hitPoints.max,
    temp,
    band: hitPointsBand(percent),
    percent,
    tempPercent,
    preview,
    currentChange,
    tempChange,
    pendingLeft,
    pendingWidth,
    tempPendingLeft,
    tempPendingWidth,
    amount,
    setAmount,
    amountValue,
    commit,
    applyDamageAmount,
    applyHealAmount,
    applyTempAmount,
  };
}
