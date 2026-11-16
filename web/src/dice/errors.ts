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

export interface DiceError {
  readonly class: DiceErrorClass;
  readonly message: string;
}

export type DiceResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: DiceError };

export function succeed<T>(value: T): DiceResult<T> {
  return { ok: true, value };
}

export function refuse<T = never>(errorClass: DiceErrorClass, message: string): DiceResult<T> {
  return { ok: false, error: { class: errorClass, message } };
}
