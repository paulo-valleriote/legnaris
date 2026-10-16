/**
 * Preview marking shared by the sheet controls and the form rows (ENG-10,
 * ENG-22, AD-009): the person changed an editable leaf (`modified`) or the last
 * dry-run recalculated a derived one (`recalculated`). A surface that never runs
 * a preview carries no mark.
 */

/** One row's preview marking; `undefined` means the row carries no mark. */
export type SheetMark = 'modified' | 'recalculated';

/** The class a marked control carries; its tint is defined in `engine.css`. */
export const FIELD_MARKED_CLASS = 'lg-field--marked';
