import { useEffect, useRef, useState, type ReactNode } from 'react';
import { messageWith } from '../l10n/strings';

export interface InlineEditProps {
  readonly value: string;
  readonly label: string;
  readonly onCommit: (value: string) => void;
  readonly fallback?: string;
  readonly className?: string;
  readonly inputMode?: 'text' | 'numeric';
  readonly display?: ReactNode;
}

export function InlineEdit({
  value,
  label,
  onCommit,
  fallback,
  className,
  inputMode,
  display,
}: InlineEditProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const finished = useRef(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
    }
  }, [editing]);

  const begin = () => {
    finished.current = false;
    setDraft(value);
    setEditing(true);
  };

  const finish = (commit: boolean) => {
    if (finished.current) {
      return;
    }
    finished.current = true;
    if (commit) {
      onCommit(draft.trim());
    }
    setEditing(false);
  };

  const classes = ['lg-inline-edit', className]
    .filter((entry): entry is string => !!entry)
    .join(' ');

  if (editing) {
    return (
      <input
        ref={inputRef}
        className={`${classes} lg-inline-edit--input`}
        aria-label={label}
        inputMode={inputMode}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => finish(true)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            finish(true);
          } else if (event.key === 'Escape') {
            event.preventDefault();
            finish(false);
          }
        }}
      />
    );
  }

  const shown =
    value.trim() === '' ? (fallback ?? messageWith('sheet.inlineAdd', { label })) : value;

  return (
    <button
      type="button"
      className={classes}
      aria-label={label}
      data-empty={value.trim() === '' && fallback === undefined ? '' : undefined}
      onClick={begin}
    >
      {display ?? shown}
    </button>
  );
}
