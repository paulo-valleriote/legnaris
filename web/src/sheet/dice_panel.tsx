import { useEffect, useLayoutEffect, useRef } from 'react';
import type { DiceResult } from '../dice/errors';
import type { RollResult } from '../dice/roll';
import { message, messageWith } from '../l10n/strings';
import { diceErrorMessageKey } from './dice_view';
import './dice.css';

export interface DicePanelProps {
  readonly label: string;
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

export const FLOATING_FLOOR_ATTRIBUTE = 'data-floating-floor';

export function floorOffset(viewportHeight: number, floorTop: number | undefined): number {
  if (floorTop === undefined || !Number.isFinite(floorTop)) return 0;
  return Math.max(0, Math.min(viewportHeight, viewportHeight - floorTop));
}

function useFloatAboveFloor(panel: React.RefObject<HTMLDivElement | null>) {
  useLayoutEffect(() => {
    const place = () => {
      const floor = document.querySelector<HTMLElement>(`[${FLOATING_FLOOR_ATTRIBUTE}]`);
      const offset = floorOffset(window.innerHeight, floor?.getBoundingClientRect().top);
      panel.current?.style.setProperty('--dice-floor', `${offset}px`);
    };
    place();
    window.addEventListener('scroll', place, { passive: true });
    window.addEventListener('resize', place);
    return () => {
      window.removeEventListener('scroll', place);
      window.removeEventListener('resize', place);
    };
  }, [panel]);
}

export function DicePanel({ label, outcome, onDismiss }: DicePanelProps) {
  const panel = useRef<HTMLDivElement>(null);
  useFloatAboveFloor(panel);
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
    <div ref={panel} className="lg-dice-panel" role="status" aria-live="polite">
      {outcome.ok ? (
        <>
          <p className="lg-dice-panel__total">
            <span className="lg-dice-panel__total-label">{label}</span>
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
