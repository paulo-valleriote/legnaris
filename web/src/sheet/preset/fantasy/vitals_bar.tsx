import { useEffect, useRef, useState } from 'react';
import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { Field } from '../../../ui/field';
import { useNarrowViewport } from '../../inline_edit';
import { usePresetValue } from '../preset_context';
import { numberValue } from '../values';
import { ARMOR_CLASS_PATH } from './armor_class';
import { formatHitPointsDelta } from './hit_points';
import { lineTone, useHitPointsView } from './hit_points_view';
import './vitals_bar.css';

const ANCHOR_SELECTOR = '[data-vitals-anchor]';
const BAR_ID = 'lg-vitals-bar';
const PANEL_ID = 'lg-vitals-bar-panel';
const AMOUNT_ID = 'lg-vitals-bar-amount';

export function stripOutOfView(
  rect: { readonly height: number; readonly bottom: number } | undefined,
): boolean {
  return rect !== undefined && rect.height > 0 && rect.bottom <= 0;
}

function VitalsBarSurface() {
  const [open, setOpen] = useState(false);
  const toggleRef = useRef<HTMLButtonElement>(null);
  const armorClass = numberValue(usePresetValue(ARMOR_CLASS_PATH));
  const {
    current,
    max,
    temp,
    band,
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
    applyDamageAmount,
    applyHealAmount,
  } = useHitPointsView();

  useEffect(() => {
    if (!open) {
      return undefined;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') {
        return;
      }
      event.preventDefault();
      setOpen(false);
      toggleRef.current?.focus();
    };
    document.addEventListener('keydown', onKeyDown, true);
    return () => document.removeEventListener('keydown', onKeyDown, true);
  }, [open]);

  return (
    <div
      className="lg-vitals-bar"
      data-testid="sheet-vitals-bar"
      id={BAR_ID}
      role="group"
      aria-label={message('sheet.play.vitals')}
    >
      <div className="lg-vitals-bar__row">
        <p className="lg-vitals-bar__stat" data-testid="sheet-vitals-bar-ac">
          <span className="lg-vitals-bar__stat-label">{message('sheet.play.armorClassShort')}</span>
          <span className="lg-vitals-bar__stat-value">
            {armorClass === undefined ? message('sheet.play.noValue') : armorClass}
          </span>
        </p>
        <div className="lg-vitals-bar__hp" data-testid="sheet-vitals-bar-hp">
          <span className="lg-vitals-bar__numbers">
            {messageWith('sheet.play.hitPointsValue', { count: current, max })}
          </span>
          <span
            className="lg-vitals-bar__bar"
            data-testid="sheet-vitals-bar-meter"
            data-band={band}
            role="meter"
            aria-label={message('sheet.play.hitPoints')}
            aria-valuemin={0}
            aria-valuemax={max}
            aria-valuenow={current}
          >
            <span
              className="lg-vitals-bar__fill"
              style={{ width: `${percent}%` }}
              aria-hidden="true"
            />
            {temp > 0 && (
              <span
                className="lg-vitals-bar__temp"
                style={{ left: `${percent}%`, width: `${Math.min(tempPercent, 100 - percent)}%` }}
                aria-hidden="true"
              />
            )}
            {currentChange !== undefined && pendingWidth > 0 && (
              <span
                className="lg-vitals-bar__pending"
                data-tone={lineTone(currentChange.delta)}
                style={{ left: `${pendingLeft}%`, width: `${pendingWidth}%` }}
                aria-hidden="true"
              />
            )}
            {tempChange !== undefined && tempPendingWidth > 0 && (
              <span
                className="lg-vitals-bar__pending lg-vitals-bar__pending--temp"
                data-tone={lineTone(tempChange.delta)}
                style={{ left: `${tempPendingLeft}%`, width: `${tempPendingWidth}%` }}
                aria-hidden="true"
              />
            )}
          </span>
        </div>
        <Button
          ref={toggleRef}
          variant="secondary"
          size="sm"
          className="lg-vitals-bar__toggle"
          label={message('sheet.play.damageOrHeal')}
          ariaExpanded={open}
          ariaControls={open ? PANEL_ID : undefined}
          onClick={() => setOpen(!open)}
        />
      </div>
      {preview !== undefined && (
        <p
          className="lg-vitals-bar__pending-notes"
          data-testid="sheet-vitals-bar-pending"
          data-direction={preview.direction}
        >
          {currentChange !== undefined && (
            <span data-tone={lineTone(currentChange.delta)}>
              {messageWith('sheet.play.change', {
                before: currentChange.before,
                after: currentChange.after,
                delta: formatHitPointsDelta(currentChange.delta),
              })}
            </span>
          )}
          {tempChange !== undefined && (
            <span data-tone={lineTone(tempChange.delta)}>
              {messageWith('sheet.play.tempChange', {
                before: tempChange.before,
                after: tempChange.after,
                delta: formatHitPointsDelta(tempChange.delta),
              })}
            </span>
          )}
        </p>
      )}
      {open && (
        <div className="lg-vitals-bar__panel" id={PANEL_ID} data-testid="sheet-vitals-bar-panel">
          <div className="lg-vitals-bar__amount">
            <Field
              id={AMOUNT_ID}
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
            onClick={applyHealAmount}
          />
        </div>
      )}
    </div>
  );
}

export function VitalsBar() {
  const narrow = useNarrowViewport();
  const [stripOut, setStripOut] = useState(false);

  useEffect(() => {
    const anchor = document.querySelector(ANCHOR_SELECTOR);
    if (anchor === null) {
      return undefined;
    }
    const update = () => {
      setStripOut(stripOutOfView(anchor.getBoundingClientRect()));
    };
    update();
    window.addEventListener('scroll', update, { passive: true });
    window.addEventListener('resize', update);
    return () => {
      window.removeEventListener('scroll', update);
      window.removeEventListener('resize', update);
    };
  }, [narrow]);

  return narrow && stripOut ? <VitalsBarSurface /> : null;
}
