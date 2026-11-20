import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { Field } from '../../../ui/field';
import { InlineEdit } from '../../inline_edit';
import { usePresetCommit, usePresetLabel, usePresetSchema } from '../preset_context';
import {
  applyDamage,
  applyHeal,
  formatHitPointsDelta,
  HIT_POINTS_CURRENT,
  HIT_POINTS_MAX,
  HIT_POINTS_TEMP,
  parseHitPointsCurrent,
  parseHitPointsMaximum,
  parseHitPointsTemp,
} from './hit_points';
import { lineTone, useHitPointsView, withinRange } from './hit_points_view';

const QUICK_ACTIONS = [
  {
    kind: 'damage',
    count: 5,
    short: 'sheet.play.damage5Short',
    className: 'lg-hp-card__quick-damage',
  },
  {
    kind: 'damage',
    count: 1,
    short: 'sheet.play.damage1Short',
    className: 'lg-hp-card__quick-damage',
  },
  {
    kind: 'heal',
    count: 1,
    short: 'sheet.play.heal1Short',
    className: 'lg-hp-card__quick-heal',
  },
  {
    kind: 'heal',
    count: 5,
    short: 'sheet.play.heal5Short',
    className: 'lg-hp-card__quick-heal',
  },
] as const;

function quickActionName(action: (typeof QUICK_ACTIONS)[number]) {
  return messageWith(
    action.kind === 'damage' ? 'sheet.play.damageAmount' : 'sheet.play.healAmount',
    { count: action.count },
  );
}

export function HitPointsCard() {
  const {
    points,
    current,
    currentValue,
    max,
    maxValue,
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
    amountValue,
    commit,
    applyDamageAmount,
    applyHealAmount,
    applyTempAmount,
  } = useHitPointsView();
  const currentLabel = usePresetLabel(HIT_POINTS_CURRENT);
  const maxLabel = usePresetLabel(HIT_POINTS_MAX);
  const tempLabel = usePresetLabel(HIT_POINTS_TEMP);
  const currentSchema = usePresetSchema(HIT_POINTS_CURRENT);
  const maxSchema = usePresetSchema(HIT_POINTS_MAX);
  const tempSchema = usePresetSchema(HIT_POINTS_TEMP);
  const commitCurrent = usePresetCommit(HIT_POINTS_CURRENT);
  const commitMax = usePresetCommit(HIT_POINTS_MAX);
  const commitTemp = usePresetCommit(HIT_POINTS_TEMP);

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
              value={String(maxValue)}
              inputMode="numeric"
              onCommit={(text) => {
                const next = parseHitPointsMaximum(text);
                if (next !== undefined) {
                  commitMax(next);
                  const clamped = withinRange(currentValue, next);
                  if (clamped !== currentValue) {
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
        data-band={band}
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
            className="lg-hp-card__pending lg-pending-stripe"
            data-testid="sheet-hp-pending"
            data-tone={lineTone(currentChange.delta)}
            style={{ left: `${pendingLeft}%`, width: `${pendingWidth}%` }}
            aria-hidden="true"
          />
        )}
        {tempChange !== undefined && tempPendingWidth > 0 && (
          <span
            className="lg-hp-card__pending lg-hp-card__pending--temp lg-pending-stripe"
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
          {QUICK_ACTIONS.map((action) => (
            <Button
              key={`${action.kind}-${action.count}`}
              variant="secondary"
              size="sm"
              className={action.className}
              name={quickActionName(action)}
              title={quickActionName(action)}
              onClick={() =>
                commit(
                  action.kind === 'damage'
                    ? applyDamage(points, action.count)
                    : applyHeal(points, action.count),
                )
              }
            >
              {message(action.short)}
            </Button>
          ))}
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
              placeholder={message('sheet.play.amountPlaceholder')}
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
      </div>
    </div>
  );
}
