/**
 * Reason classes of the Dice expression grammar. The strings are part of the
 * shared test vectors (`testdata/dice/vectors.json`) and match the Go package
 * `backend/internal/dice`; change them there first.
 */
export type DiceErrorClass =
  | 'empty'
  | 'too_long'
  | 'too_many_terms'
  | 'dice_count'
  | 'dice_faces'
  | 'number_range'
  | 'missing_term'
  | 'unreadable_term'
  | 'unknown_field'
  | 'non_numeric_field'
  | 'no_rows'
  | 'no_value'
  | 'overflow'
  | 'no_dice'
  | 'turned_off';

/** Why a Dice expression was refused or a Roll could not happen. */
export interface DiceError {
  readonly class: DiceErrorClass;
  /** An English diagnostic for logs and tests; user-facing copy resolves through `l10n` by `class`. */
  readonly message: string;
}

/** The outcome of an operation that may be refused: errors are values, never exceptions. */
export type DiceResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: DiceError };

/** succeed wraps [value] as a successful {@link DiceResult}. */
export function succeed<T>(value: T): DiceResult<T> {
  return { ok: true, value };
}

/** refuse builds a failed {@link DiceResult} of the given class. */
export function refuse<T = never>(errorClass: DiceErrorClass, message: string): DiceResult<T> {
  return { ok: false, error: { class: errorClass, message } };
}
