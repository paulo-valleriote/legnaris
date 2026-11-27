import { useId, useState } from 'react';
import type { FormEvent } from 'react';
import { message, messageWith } from '../../../l10n/strings';
import { Button } from '../../../ui/button';
import { Field } from '../../../ui/field';
import { InlineEdit } from '../../inline_edit';
import { numberValue } from '../values';
import {
  usePresetContext,
  usePresetCommit,
  usePresetLabel,
  usePresetSchema,
  usePresetValue,
} from '../preset_context';
import type { PresetXpBarPiece } from './types';
import { parseXpGain, parseXpTotal, xpProgress } from './xp_progress';
import './xp.css';

const XP_FORMAT = new Intl.NumberFormat('en-US');

function formatXp(value: number): string {
  return XP_FORMAT.format(value);
}

export function XpBar({ piece }: { readonly piece: PresetXpBarPiece }) {
  const [open, setOpen] = useState(false);
  const [gain, setGain] = useState('');
  const panelId = useId();
  const gainId = useId();
  const { editing, readOnly } = usePresetContext();
  const level = numberValue(usePresetValue(piece.level));
  const xp = usePresetValue(piece.xp);
  const xpLabel = usePresetLabel(piece.xp);
  const xpSchema = usePresetSchema(piece.xp);
  const commitXp = usePresetCommit(piece.xp);
  const commitLevel = usePresetCommit(piece.level);
  const progress = xpProgress(level, xp);
  const next = progress.canLevelUp ? progress.nextLevel : null;

  const addGain = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const amount = parseXpGain(gain);
    if (amount === undefined) {
      return;
    }
    commitXp(progress.xp + amount);
    setGain('');
  };

  const readout = (
    <>
      <span className="lg-xp__head">
        <span className="lg-xp__label">{message('sheet.play.experience')}</span>
        <span className="lg-xp__total">
          {progress.nextThreshold === null
            ? messageWith('sheet.play.xpTotal', { count: formatXp(progress.xp) })
            : messageWith('sheet.play.xpProgress', {
                count: formatXp(progress.xp),
                next: formatXp(progress.nextThreshold),
              })}
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
    </>
  );

  return (
    <div className="lg-xp" data-testid="sheet-xp">
      {readOnly ? (
        <div className="lg-xp__trigger" data-testid="sheet-xp-readout">
          {readout}
        </div>
      ) : (
        <button
          type="button"
          className="lg-xp__trigger"
          data-testid="sheet-xp-trigger"
          aria-expanded={open}
          aria-controls={open ? panelId : undefined}
          onClick={() => setOpen((value) => !value)}
        >
          {readout}
        </button>
      )}
      {open && !readOnly && (
        <div
          id={panelId}
          className="lg-xp__panel"
          data-testid="sheet-xp-panel"
          role="group"
          aria-label={message('sheet.play.experience')}
        >
          <p className="lg-xp__panel-total" data-testid="sheet-xp-total">
            <InlineEdit
              className="lg-xp__panel-total-value"
              label={message('sheet.play.setXp')}
              path={piece.xp}
              schema={xpSchema}
              locked={!editing}
              value={String(progress.xp)}
              display={messageWith('sheet.play.xpTotal', { count: progress.xp })}
              inputMode="numeric"
              onCommit={(text) => {
                const nextTotal = parseXpTotal(text);
                if (nextTotal !== undefined) {
                  commitXp(nextTotal);
                }
              }}
            />
          </p>
          <p className="lg-xp__panel-name">{xpLabel}</p>
          <form className="lg-xp__gain" onSubmit={addGain}>
            <Field
              id={gainId}
              label={message('sheet.play.xpGain')}
              type="number"
              inputMode="numeric"
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
          {next !== null && (
            <Button
              className="lg-xp__level-up"
              size="sm"
              label={messageWith('sheet.play.levelUp', { count: next })}
              onClick={() => commitLevel(next)}
            />
          )}
        </div>
      )}
    </div>
  );
}
