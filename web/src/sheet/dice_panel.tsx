import { useEffect, useLayoutEffect, useRef } from 'react';
import type { RollPart, RollResult } from '../dice/roll';
import { message, messageWith } from '../l10n/strings';
import { CloseIcon } from '../ui/icons';
import type { RollEntry } from './dice_history';
import { diceErrorMessageKey } from './dice_view';
import './dice.css';

export interface DicePanelProps {
  readonly results: readonly RollEntry[];
  readonly onDismiss: () => void;
}

function Rolls({ part }: { readonly part: RollPart }) {
  if (part.rolls === undefined) {
    return null;
  }
  if (part.discarded === undefined) {
    return (
      <span className="lg-dice-panel__rolls">
        {messageWith('dice.rolls', { rolls: part.rolls.join(', ') })}
      </span>
    );
  }
  const kept = part.rolls.filter((_, index) => !part.discarded?.includes(index));
  return (
    <span className="lg-dice-panel__rolls">
      <span aria-hidden="true">
        {part.rolls.map((roll, index) => (
          <span key={`${index}-${roll}`}>
            {index === 0 ? null : ', '}
            <span
              className={
                part.discarded?.includes(index)
                  ? 'lg-dice-panel__die lg-dice-panel__die--dropped'
                  : 'lg-dice-panel__die lg-dice-panel__die--kept'
              }
            >
              {roll}
            </span>
          </span>
        ))}
      </span>
      <span className="lg-dice-panel__sr">
        {messageWith('dice.keptRolls', {
          rolls: part.rolls.join(', '),
          kept: kept.join(', '),
        })}
      </span>
    </span>
  );
}

function Breakdown({ result }: { readonly result: RollResult }) {
  return (
    <ul className="lg-dice-panel__parts">
      {result.parts.map((part, index) => (
        <li className="lg-dice-panel__part" key={`${index}-${part.label}`}>
          <span className="lg-dice-panel__sign">{part.sign === 1 ? '+' : '-'}</span>
          <span className="lg-dice-panel__label">{part.label}</span>
          <Rolls part={part} />
          <span className="lg-dice-panel__value">{part.value}</span>
        </li>
      ))}
    </ul>
  );
}

const MODE_KEYS = {
  advantage: 'dice.mode.advantage',
  disadvantage: 'dice.mode.disadvantage',
} as const;

function modeKey(mode: RollEntry['mode']) {
  return mode === 'advantage' || mode === 'disadvantage' ? MODE_KEYS[mode] : undefined;
}

function Result({ entry }: { readonly entry: RollEntry }) {
  const key = modeKey(entry.mode);
  if (entry.outcome.ok) {
    return (
      <>
        <p className="lg-dice-panel__total">
          <span className="lg-dice-panel__total-label">
            {entry.label}
            {key !== undefined && <span className="lg-dice-panel__mode">{message(key)}</span>}
          </span>
          <span className="lg-dice-panel__total-value">{entry.outcome.value.total}</span>
        </p>
        {entry.outcome.value.minimum !== undefined && (
          <p className="lg-dice-panel__minimum">
            {messageWith('dice.minimum', { minimum: entry.outcome.value.minimum })}
          </p>
        )}
        <Breakdown result={entry.outcome.value} />
      </>
    );
  }

  return (
    <>
      <p className="lg-dice-panel__total">
        <span className="lg-dice-panel__total-label">
          {entry.label}
          {key !== undefined && <span className="lg-dice-panel__mode">{message(key)}</span>}
        </span>
      </p>
      <p className="lg-dice-panel__error">
        {message(diceErrorMessageKey(entry.outcome.error.class))}
      </p>
    </>
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

export function DicePanel({ results, onDismiss }: DicePanelProps) {
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

  const newest = results[0];
  if (newest === undefined) {
    return null;
  }
  const announcement =
    newest.outcome.ok === false
      ? message(diceErrorMessageKey(newest.outcome.error.class))
      : newest.mode === 'advantage'
        ? messageWith('dice.rolledAdvantage', {
            label: newest.label,
            total: newest.outcome.value.total,
          })
        : newest.mode === 'disadvantage'
          ? messageWith('dice.rolledDisadvantage', {
              label: newest.label,
              total: newest.outcome.value.total,
            })
          : messageWith('dice.rolled', {
              label: newest.label,
              total: newest.outcome.value.total,
            });

  return (
    <div ref={panel} className="lg-dice-panel">
      <p className="lg-dice-panel__sr" role="status" aria-live="polite" aria-atomic="true">
        {announcement}
      </p>
      <ol className="lg-dice-panel__history" aria-label={message('dice.history')}>
        {results.map((entry) => (
          <li className="lg-dice-panel__entry" key={entry.id}>
            <Result entry={entry} />
          </li>
        ))}
      </ol>
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
