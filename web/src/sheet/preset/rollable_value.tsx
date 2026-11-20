import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { Resolution } from '../../dice/resolve';
import { rollsD20, type RollMode } from '../../dice/roll';
import { messageWith } from '../../l10n/strings';
import { LONG_PRESS_MS, useRowRoll } from '../dice_row';
import { diceSummary } from '../dice_view';
import { opensRollModeMenu, RollModeDialog, rollModeFromClick } from '../roll_mode';
import './rollable_value.css';

export function rollAccessibleName(label: string, resolution: Resolution): string {
  const summary = resolution.rollable ? diceSummary(resolution.parsed) : undefined;
  return summary === undefined
    ? messageWith('dice.roll', { label })
    : messageWith('dice.rollWith', { label, dice: summary });
}

export interface RollableValueProps {
  readonly rowKey: string;
  readonly sectionKey: string;
  readonly label: string;
  readonly text: string;
  readonly allowRoll?: boolean;
  readonly className?: string;
  readonly accessibleName?: string;
  readonly icon?: ReactNode;
}

export function RollableValue({
  rowKey,
  sectionKey,
  label,
  text,
  allowRoll = true,
  className,
  accessibleName,
  icon,
}: RollableValueProps) {
  const { resolution, roll } = useRowRoll(rowKey, sectionKey, label);
  const [choosing, setChoosing] = useState(false);
  const pressTimer = useRef<number | undefined>(undefined);
  const longPressed = useRef(false);
  const classes = ['lg-rollable', className].filter((entry): entry is string => !!entry).join(' ');
  const content = (
    <>
      {icon}
      {text}
    </>
  );

  const cancelPress = () => {
    if (pressTimer.current !== undefined) {
      window.clearTimeout(pressTimer.current);
      pressTimer.current = undefined;
    }
  };

  useEffect(() => cancelPress, []);

  if (!allowRoll || !resolution.rollable) {
    return (
      <span
        className={`${classes} lg-rollable--still`}
        data-rollable="false"
        title={accessibleName}
      >
        {content}
      </span>
    );
  }

  const d20 = rollsD20(resolution.parsed);

  const rollWith = (mode: RollMode) => {
    cancelPress();
    setChoosing(false);
    roll(mode);
  };

  const startPress = () => {
    longPressed.current = false;
    cancelPress();
    pressTimer.current = window.setTimeout(() => {
      pressTimer.current = undefined;
      longPressed.current = true;
      setChoosing(true);
    }, LONG_PRESS_MS);
  };

  const openChoice = () => {
    cancelPress();
    setChoosing(true);
  };

  return (
    <>
      <button
        type="button"
        className={`${classes} lg-rollable--button`}
        data-rollable="true"
        aria-label={accessibleName ?? rollAccessibleName(label, resolution)}
        aria-haspopup={d20 ? 'dialog' : undefined}
        title={accessibleName}
        onPointerDown={d20 ? startPress : undefined}
        onPointerUp={d20 ? cancelPress : undefined}
        onPointerLeave={d20 ? cancelPress : undefined}
        onPointerCancel={d20 ? cancelPress : undefined}
        onContextMenu={
          d20
            ? (event) => {
                event.preventDefault();
                openChoice();
              }
            : undefined
        }
        onKeyDown={
          d20
            ? (event) => {
                if (opensRollModeMenu(event)) {
                  event.preventDefault();
                  openChoice();
                }
              }
            : undefined
        }
        onClick={(event) => {
          if (longPressed.current) {
            longPressed.current = false;
            return;
          }
          roll(d20 ? rollModeFromClick(event) : 'normal');
        }}
      >
        {content}
      </button>
      {choosing && (
        <RollModeDialog label={label} onChoose={rollWith} onClose={() => setChoosing(false)} />
      )}
    </>
  );
}
