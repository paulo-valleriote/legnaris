/**
 * Typed English copy catalog — the only place a user-facing string lives in the
 * client (API-23).
 *
 * Every key is a plain string in the single V1 locale. Components and the API
 * client resolve copy through `message()` rather than embedding literals, so a
 * later localization feature replaces this module without touching call sites.
 */

export const messages = {
  'api.error.clientFault': 'The app sent a request the server could not read.',
  'api.error.invalidEdit': 'That change is not allowed, so the value was restored.',
  'api.error.renderMismatch': 'This sheet no longer matches its template. Reload to continue.',
  'api.error.notFound': 'That sheet no longer exists.',
  'api.error.serverError': 'The server had a problem. Try again in a moment.',
  'api.error.offline': 'No connection to the server.',
  'api.error.unknown': 'Something went wrong. Try again.',
  'api.error.parse': 'The server response could not be read.',

  'nav.catalog': 'Catalog',

  'catalog.title': 'Templates',
  'catalog.loading': 'Loading templates',
  'catalog.empty': 'No templates are available.',
  'catalog.retry': 'Retry',
  'catalog.openHint': 'Open this template',

  'sheet.dialogTitle': 'New sheet',
  'sheet.nameLabel': 'Sheet name',
  'sheet.namePlaceholder': 'Example: Aragorn, Ranger of the North',
  'sheet.nameRequired': 'A sheet name is required.',
  'sheet.create': 'Create sheet',
  'sheet.close': 'Close',
  'sheet.loading': 'Loading template',
  'sheet.retry': 'Try again',

  'engine.calculated': 'Calculated',
  'engine.none': 'None',
  'engine.modified': 'Modified',
  'engine.recalculated': 'Recalculated',
} as const;

/** A key of the English catalog. */
export type MessageKey = keyof typeof messages;

/** Resolves the copy for [key] from the English catalog (API-23). */
export function message(key: MessageKey): string {
  return messages[key];
}
