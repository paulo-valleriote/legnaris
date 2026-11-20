import './segmented_choice.css';

export interface SegmentedChoiceOption {
  readonly value: string;
  readonly label: string;
  readonly short?: string;
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
        <button
          key={option.value}
          type="button"
          className="lg-segmented__option"
          disabled={disabled}
          aria-pressed={option.value === selected}
          aria-label={option.short === undefined ? undefined : option.label}
          title={option.short === undefined ? undefined : option.label}
          onClick={() => onChange(option.value)}
        >
          {option.short ?? option.label}
        </button>
      ))}
    </div>
  );
}
