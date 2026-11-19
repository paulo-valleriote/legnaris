import { useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';
import type { FieldSchema } from '../api/models/template';
import { message, messageWith } from '../l10n/strings';
import { Dialog } from '../ui/dialog';

const NARROW_QUERY = '(max-width: 40rem)';

function subscribeNarrow(listener: () => void): () => void {
  if (typeof window.matchMedia !== 'function') {
    return () => undefined;
  }
  const query = window.matchMedia(NARROW_QUERY);
  query.addEventListener('change', listener);
  return () => query.removeEventListener('change', listener);
}

function readNarrow(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia(NARROW_QUERY).matches;
}

export function useNarrowViewport(): boolean {
  return useSyncExternalStore(subscribeNarrow, readNarrow, () => false);
}

export interface InlineEditProps {
  readonly value: string;
  readonly label: string;
  readonly onCommit: (value: string) => void;
  readonly fallback?: string;
  readonly className?: string;
  readonly inputMode?: 'text' | 'numeric';
  readonly schema?: FieldSchema;
  readonly path?: string;
  readonly display?: ReactNode;
}

function isNumericInput(
  schema: FieldSchema | undefined,
  inputMode: 'text' | 'numeric' | undefined,
) {
  return schema?.type === 'number' || inputMode === 'numeric';
}

export function InlineEdit({
  value,
  label,
  onCommit,
  fallback,
  className,
  inputMode,
  schema,
  path,
  display,
}: InlineEditProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const finished = useRef(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const narrow = useNarrowViewport();
  const numeric = isNumericInput(schema, inputMode);

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

  const shown =
    value.trim() === '' ? (fallback ?? messageWith('sheet.inlineAdd', { label })) : value;
  const sheetDraft = numeric ? draft.replace(/^\+/, '') : draft;

  if (editing && narrow) {
    return (
      <>
        <button
          type="button"
          className={classes}
          aria-label={label}
          data-edit-path={path}
          data-empty={value.trim() === '' && fallback === undefined ? '' : undefined}
          onClick={begin}
        >
          {display ?? shown}
        </button>
        <Dialog
          open
          title={label}
          placement="bottom"
          closeLabel={message('common.close')}
          onClose={() => finish(false)}
          initialFocusRef={inputRef}
          className="lg-inline-edit__sheet"
        >
          <input
            ref={inputRef}
            className={`${classes} lg-inline-edit--sheet-input`}
            aria-label={label}
            data-edit-path={path}
            type={numeric ? 'number' : 'text'}
            inputMode={numeric ? 'numeric' : 'text'}
            value={sheetDraft}
            onChange={(event) => setDraft(event.target.value)}
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
        </Dialog>
      </>
    );
  }

  if (editing) {
    return (
      <input
        ref={inputRef}
        className={`${classes} lg-inline-edit--input`}
        aria-label={label}
        data-edit-path={path}
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

  return (
    <button
      type="button"
      className={classes}
      aria-label={label}
      data-edit-path={path}
      data-empty={value.trim() === '' && fallback === undefined ? '' : undefined}
      onClick={begin}
    >
      {display ?? shown}
    </button>
  );
}
