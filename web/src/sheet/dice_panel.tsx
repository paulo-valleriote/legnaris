/**
 * The Roll result panel (ADR 0005, LGN-166): the total and every die and number
 * that made it, or the explicit reason the Roll could not happen. It is a
 * non-modal status region: it never traps focus, never sets `aria-modal` and
 * never covers the sheet, and it closes on its dismiss button or on Escape.
 */

import { useEffect } from 'react';
import type { DiceResult } from '../dice/errors';
import type { RollResult } from '../dice/roll';
import { message, messageWith } from '../l10n/strings';
import { diceErrorMessageKey } from './dice_view';
import './dice.css';

export interface DicePanelProps {
  /** The name of the Row or Section that was rolled; read aloud with the total. */
  readonly label: string;
  /** The outcome of the Roll: the result, or the refusal that stopped it. */
  readonly outcome: DiceResult<RollResult>;
  readonly onDismiss: () => void;
}

function Breakdown({ result }: { readonly result: RollResult }) {
  return (
    <ul className="lg-dice-panel__parts">
      {result.parts.map((part, index) => (
        <li className="lg-dice-panel__part" key={`${index}-${part.label}`}>
          <span className="lg-dice-panel__sign">{part.sign === 1 ? '+' : '-'}</span>
          <span className="lg-dice-panel__label">{part.label}</span>
          {part.rolls !== undefined && (
            <span className="lg-dice-panel__rolls">[{part.rolls.join(', ')}]</span>
          )}
          <span className="lg-dice-panel__value">{part.value}</span>
        </li>
      ))}
    </ul>
  );
}

/** Renders the Roll outcome; Escape anywhere on the page dismisses it. */
export function DicePanel({ label, outcome, onDismiss }: DicePanelProps) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onDismiss();
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [onDismiss]);

  return (
    <div className="lg-dice-panel" role="status" aria-live="polite">
      {outcome.ok ? (
        <>
          <p className="lg-dice-panel__total">
            <span className="lg-dice-panel__total-label">{message('dice.total')}</span>
            <span className="lg-dice-panel__total-value">{outcome.value.total}</span>
          </p>
          <p className="lg-dice-panel__sr">
            {messageWith('dice.rolled', { label, total: outcome.value.total })}
          </p>
          <Breakdown result={outcome.value} />
        </>
      ) : (
        <p className="lg-dice-panel__error">{message(diceErrorMessageKey(outcome.error.class))}</p>
      )}
      <button
        type="button"
        className="lg-dice-panel__dismiss"
        aria-label={message('dice.dismiss')}
        onClick={onDismiss}
      >
        ×
      </button>
    </div>
  );
}
