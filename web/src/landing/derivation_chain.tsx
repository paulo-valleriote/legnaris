import { useState } from 'react';
import { message, messageWith } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';

export const MIGHT_MIN = 8;
export const MIGHT_MAX = 20;
const MIGHT_START = 16;
const TRAINING_BONUS = 2;
const CARRY_PER_POINT = 15;

export interface Derived {
  readonly modifier: number;
  readonly attack: number;
  readonly carry: number;
}

export function deriveFromMight(might: number): Derived {
  const modifier = Math.floor((might - 10) / 2);
  return { modifier, attack: modifier + TRAINING_BONUS, carry: might * CARRY_PER_POINT };
}

export function signed(value: number): string {
  return value >= 0 ? `+${value}` : `${value}`;
}

interface Row {
  readonly label: MessageKey;
  readonly formula: MessageKey;
  readonly value: string;
}

export function DerivationChain() {
  const [might, setMight] = useState(MIGHT_START);
  const derived = deriveFromMight(might);
  const rows: readonly Row[] = [
    {
      label: 'landing.chainModifier',
      formula: 'landing.chainModifierFormula',
      value: signed(derived.modifier),
    },
    {
      label: 'landing.chainAttack',
      formula: 'landing.chainAttackFormula',
      value: signed(derived.attack),
    },
    {
      label: 'landing.chainCarry',
      formula: 'landing.chainCarryFormula',
      value: messageWith('landing.chainCarryValue', { count: derived.carry }),
    },
  ];

  return (
    <div className="lg-chain" data-testid="landing-chain">
      <div className="lg-chain__source">
        <span className="lg-chain__source-label" id="lg-chain-might">
          {message('landing.chainMight')}
        </span>
        <div className="lg-chain__stepper">
          <button
            type="button"
            className="lg-chain__step"
            aria-label={message('landing.chainLower')}
            disabled={might <= MIGHT_MIN}
            onClick={() => setMight((value) => Math.max(MIGHT_MIN, value - 1))}
          >
            {message('landing.chainLowerGlyph')}
          </button>
          <output
            className="lg-chain__score"
            aria-labelledby="lg-chain-might"
            data-testid="landing-chain-might"
          >
            {might}
          </output>
          <button
            type="button"
            className="lg-chain__step"
            aria-label={message('landing.chainRaise')}
            disabled={might >= MIGHT_MAX}
            onClick={() => setMight((value) => Math.min(MIGHT_MAX, value + 1))}
          >
            {message('landing.chainRaiseGlyph')}
          </button>
        </div>
        <span className="lg-chain__hint">{message('landing.chainHint')}</span>
      </div>
      <dl className="lg-chain__derived" aria-live="polite">
        {rows.map((row) => (
          <div className="lg-chain__row" key={row.label}>
            <dt className="lg-chain__label">
              {message(row.label)}
              <span className="lg-chain__formula">{message(row.formula)}</span>
            </dt>
            <dd className="lg-chain__value" key={row.value} data-testid={`${row.label}-value`}>
              {row.value}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
