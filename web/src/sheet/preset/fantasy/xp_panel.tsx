import { useId, useState } from 'react';
import type { FormEvent } from 'react';
import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { Field } from '../../../ui/field';
import { usePresetCommit, usePresetValue } from '../preset_context';
import { numberValue } from '../values';
import { LEVEL_PATH, parseXpGain, XP_PATH, xpProgress } from './xp_progress';
import './xp_progress.css';

export function XpBar() {
  const [open, setOpen] = useState(false);
  const [gain, setGain] = useState('');
  const panelId = useId();
  const level = numberValue(usePresetValue(LEVEL_PATH));
  const xp = usePresetValue(XP_PATH);
  const commitXp = usePresetCommit(XP_PATH);
  const commitLevel = usePresetCommit(LEVEL_PATH);
  const progress = xpProgress(level, xp);
  const nextLevel = progress.canLevelUp ? progress.nextLevel : null;

  const addGain = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const amount = parseXpGain(gain);
    if (amount === undefined) {
      return;
    }
    commitXp(progress.xp + amount);
    setGain('');
  };

  return (
    <div className="lg-xp" data-testid="sheet-xp">
      <button
        type="button"
        className="lg-xp__trigger"
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        onClick={() => setOpen((value) => !value)}
      >
        <span className="lg-xp__head">
          <span className="lg-xp__label">{message('sheet.play.experience')}</span>
          <span className="lg-xp__total">
            {messageWith('sheet.play.xpTotal', { count: progress.xp })}
          </span>
        </span>
        <span
          className="lg-xp__bar"
          role="progressbar"
          aria-label={message('sheet.play.experience')}
          aria-valuemin={progress.floor}
          aria-valuemax={progress.nextThreshold ?? progress.floor}
          aria-valuenow={progress.value}
        >
          <span
            className="lg-xp__fill"
            style={{ width: `${progress.percent}%` }}
            aria-hidden="true"
          />
        </span>
      </button>
      {open && (
        <div
          id={panelId}
          className="lg-xp__panel"
          data-testid="sheet-xp-panel"
          role="group"
          aria-label={message('sheet.play.experience')}
        >
          <p className="lg-xp__panel-total" data-testid="sheet-xp-total">
            {messageWith('sheet.play.xpTotal', { count: progress.xp })}
          </p>
          <form className="lg-xp__gain" onSubmit={addGain}>
            <Field
              id="sheet-xp-gain"
              label={message('sheet.play.xpGain')}
              type="number"
              inputMode="decimal"
              numeric
              value={gain}
              onChange={setGain}
            />
            <Button
              type="submit"
              variant="secondary"
              size="sm"
              label={message('sheet.play.xpAdd')}
            />
          </form>
          {nextLevel !== null && (
            <Button
              className="lg-xp__level-up"
              size="sm"
              label={messageWith('sheet.play.levelUp', { count: nextLevel })}
              onClick={() => commitLevel(nextLevel)}
            />
          )}
        </div>
      )}
    </div>
  );
}
