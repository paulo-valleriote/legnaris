import { Button } from '../../ui/button';
import './segmented_choice.css';

export interface SegmentedChoiceOption {
  readonly value: string;
  readonly label: string;
}

export interface SegmentedChoiceProps {
  readonly label: string;
  readonly options: readonly SegmentedChoiceOption[];
  readonly selected?: string;
  readonly disabled?: boolean;
  readonly onChange: (value: string) => void;
}

export function SegmentedChoice({
  label,
  options,
  selected,
  disabled = false,
  onChange,
}: SegmentedChoiceProps) {
  if (options.length === 0) {
    return null;
  }

  return (
    <div className="lg-segmented" role="group" aria-label={label}>
      {options.map((option) => (
        <Button
          key={option.value}
          variant="outline"
          size="sm"
          className="lg-segmented__option"
          disabled={disabled}
          ariaPressed={option.value === selected}
          label={option.label}
          onClick={() => onChange(option.value)}
        />
      ))}
    </div>
  );
}
