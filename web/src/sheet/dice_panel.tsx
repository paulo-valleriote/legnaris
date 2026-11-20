import { useEffect, useLayoutEffect, useRef } from 'react';
import type { DiceResult } from '../dice/errors';
import type { RollResult } from '../dice/roll';
import { message, messageWith } from '../l10n/strings';
import { CloseIcon } from '../ui/icons';
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
            <span className="lg-dice-panel__rolls">
              {messageWith('dice.rolls', { rolls: part.rolls.join(', ') })}
            </span>
          )}
          <span className="lg-dice-panel__value">{part.value}</span>
        </li>
      ))}
    </ul>
  );
}

export const FLOATING_FLOOR_ATTRIBUTE = 'data-floating-floor';

const FLOOR_SELECTOR = `[${FLOATING_FLOOR_ATTRIBUTE}]`;

export function floorOffset(viewportHeight: number, floorTop: number | undefined): number {
  if (floorTop === undefined || !Number.isFinite(floorTop)) return 0;
  return Math.max(0, Math.min(viewportHeight, viewportHeight - floorTop));
}

function isFloorNode(node: Node): boolean {
  if (node.nodeType !== Node.ELEMENT_NODE) {
    return false;
  }
  const element = node as Element;
  return element.matches(FLOOR_SELECTOR) || element.querySelector(FLOOR_SELECTOR) !== null;
}

function touchesFloor(records: readonly MutationRecord[]): boolean {
  return records.some(
    (record) =>
      record.type === 'attributes' ||
      [...record.addedNodes, ...record.removedNodes].some(isFloorNode),
  );
}

function useFloatAboveFloor(panel: React.RefObject<HTMLDivElement | null>) {
  useLayoutEffect(() => {
    const place = () => {
      const floor = document.querySelector<HTMLElement>(FLOOR_SELECTOR);
      const offset = floorOffset(window.innerHeight, floor?.getBoundingClientRect().top);
      const element = panel.current;
      if (element !== null && element.style.getPropertyValue('--dice-floor') !== `${offset}px`) {
        element.style.setProperty('--dice-floor', `${offset}px`);
      }
    };
    const resizer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(place);
    let observed: HTMLElement | null = null;
    const track = () => {
      const floor = document.querySelector<HTMLElement>(FLOOR_SELECTOR);
      if (floor === observed) {
        return;
      }
      resizer?.disconnect();
      observed = floor;
      if (floor !== null) {
        resizer?.observe(floor);
      }
    };
    const refresh = () => {
      track();
      place();
    };
    refresh();
    window.addEventListener('scroll', place, { passive: true });
    window.addEventListener('resize', refresh);
    const mutations =
      typeof MutationObserver === 'undefined'
        ? undefined
        : new MutationObserver((records) => {
            if (touchesFloor(records)) {
              refresh();
            }
          });
    mutations?.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: [FLOATING_FLOOR_ATTRIBUTE],
    });
    return () => {
      window.removeEventListener('scroll', place);
      window.removeEventListener('resize', refresh);
      mutations?.disconnect();
      resizer?.disconnect();
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

  const announcement = outcome.ok
    ? messageWith('dice.rolled', { label, total: outcome.value.total })
    : message(diceErrorMessageKey(outcome.error.class));

  return (
    <div ref={panel} className="lg-dice-panel">
      <p className="lg-dice-panel__sr" role="status" aria-live="polite" aria-atomic="true">
        {announcement}
      </p>
      {outcome.ok ? (
        <>
          <p className="lg-dice-panel__total">
            <span className="lg-dice-panel__total-label">{label}</span>
            <span className="lg-dice-panel__total-value">{outcome.value.total}</span>
          </p>
          <Breakdown result={outcome.value} />
        </>
      ) : (
        <p className="lg-dice-panel__error">{announcement}</p>
      )}
      <button
        type="button"
        className="lg-dice-panel__dismiss"
        aria-label={message('common.close')}
        title={message('common.close')}
        onClick={onDismiss}
      >
        <CloseIcon className="lg-dice-panel__close" />
      </button>
    </div>
  );
}
