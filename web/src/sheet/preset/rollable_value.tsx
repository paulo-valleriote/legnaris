import type { ReactNode } from 'react';
import type { Resolution } from '../../dice/resolve';
import { messageWith } from '../../l10n/strings';
import { useRowRoll } from '../dice_row';
import { diceSummary } from '../dice_view';
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
  const classes = ['lg-rollable', className].filter((entry): entry is string => !!entry).join(' ');
  const content = (
    <>
      {icon}
      {text}
    </>
  );

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

  return (
    <button
      type="button"
      className={`${classes} lg-rollable--button`}
      data-rollable="true"
      aria-label={accessibleName ?? rollAccessibleName(label, resolution)}
      title={accessibleName}
      onClick={roll}
    >
      {content}
    </button>
  );
}
