import { useId, useState } from 'react';
import { message, messageWith } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';

export const STRENGTH_MIN = 8;
export const STRENGTH_MAX = 20;
const STRENGTH_START = 16;
const PROFICIENCY_BONUS = 2;
const CARRY_PER_POINT = 15;

export interface Derived {
  readonly modifier: number;
  readonly attack: number;
  readonly carry: number;
}

export function deriveFromStrength(strength: number): Derived {
  const modifier = Math.floor((strength - 10) / 2);
  return {
    modifier,
    attack: modifier + PROFICIENCY_BONUS,
    carry: strength * CARRY_PER_POINT,
  };
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
  const [strength, setStrength] = useState(STRENGTH_START);
  const titleId = useId();
  const derived = deriveFromStrength(strength);
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
    <div className="lg-chain" role="group" aria-labelledby={titleId} data-testid="landing-chain">
      <h2 className="lg-chain__title" id={titleId}>
        {message('landing.chainTitle')}
      </h2>
      <div className="lg-chain__source">
        <span className="lg-chain__source-label" id="lg-chain-strength">
          {message('landing.chainStrength')}
        </span>
        <div className="lg-chain__stepper">
          <button
            type="button"
            className="lg-chain__step"
            aria-label={message('landing.chainLower')}
            disabled={strength <= STRENGTH_MIN}
            onClick={() => setStrength((value) => Math.max(STRENGTH_MIN, value - 1))}
          >
            {message('landing.chainLowerGlyph')}
          </button>
          <output
            className="lg-chain__score"
            aria-labelledby="lg-chain-strength"
            data-testid="landing-chain-strength"
          >
            {strength}
          </output>
          <button
            type="button"
            className="lg-chain__step"
            aria-label={message('landing.chainRaise')}
            disabled={strength >= STRENGTH_MAX}
            onClick={() => setStrength((value) => Math.min(STRENGTH_MAX, value + 1))}
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
