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
  'api.error.unauthorized': 'Your session has ended. Sign in again.',
  'api.error.forbidden': 'That action was refused. Reload the app and try again.',
  'api.error.rateLimited': 'Too many attempts. Wait a moment and try again.',

  'auth.titleLogin': 'Sign in to your archive',
  'auth.titleRegister': 'Create your archive',
  'auth.subtitleLogin': 'Continue where your last sheet left off.',
  'auth.subtitleRegister': 'One account keeps every character sheet in one place.',
  'auth.modeGroupLabel': 'Account mode',
  'auth.modeLogin': 'Existing account',
  'auth.modeRegister': 'New account',
  'auth.emailLabel': 'Email',
  'auth.passwordLabel': 'Password',
  'auth.usernameLabel': 'Username',
  'auth.submitLogin': 'Sign in',
  'auth.submitRegister': 'Create account',
  'auth.restoring': 'Restoring your session',

  'app.name': 'Legnaris',
  'nav.catalog': 'Catalog',
  'nav.ownSheet': 'Your sheet',
  'nav.logout': 'Sign out',
  'nav.sectionWorkspace': 'Workspace',
  'nav.tagline': 'Character archive',

  'theme.label': 'Theme',
  'theme.toggleLabel': 'Switch theme',
  'theme.switchToDark': 'Switch to dark',
  'theme.switchToLight': 'Switch to light',

  'catalog.title': 'Templates',
  'catalog.loading': 'Loading templates',
  'catalog.empty': 'No templates are available.',
  'catalog.retry': 'Retry',
  'catalog.openHint': 'Open this template',

  'sheet.dialogTitle': 'New sheet',
  'sheet.templateLabel': 'Template',
  'sheet.nameLabel': 'Sheet name',
  'sheet.namePlaceholder': 'Example: Aragorn, Ranger of the North',
  'sheet.nameRequired': 'A sheet name is required.',
  'sheet.create': 'Create sheet',
  'sheet.creating': 'Creating sheet',
  'sheet.close': 'Close',
  'sheet.loading': 'Loading template',
  'sheet.retry': 'Try again',

  'sheet.headerTitle': 'Sheet',
  'sheet.openLoading': 'Loading sheet',
  'sheet.templateLoading': 'Loading sheet template',
  'sheet.emptyFields': 'This template defines no fields.',
  'sheet.missing': 'This sheet no longer exists.',
  'sheet.retryOpen': 'Reload sheet',
  'sheet.pendingEdits': 'Unsaved changes',
  'sheet.save': 'Save',
  'sheet.saving': 'Saving',
  'sheet.saved': 'Saved',
  'sheet.preview': 'Preview',
  'sheet.previewing': 'Previewing',
  'sheet.refreshTitle': 'Discard unsaved changes?',
  'sheet.refreshMessage': 'Reloading the sheet template discards your unsaved changes.',
  'sheet.refreshConfirm': 'Discard and reload',
  'sheet.leaveTitle': 'Discard unsaved changes and leave?',
  'sheet.leaveMessage': 'Leaving this sheet discards your unsaved changes.',
  'sheet.leaveConfirm': 'Discard and leave',
  'sheet.cancel': 'Keep editing',

  'sheet.ownTitle': 'Your character sheet',
  'sheet.ownLoading': 'Loading your sheet',
  'sheet.ownEmpty': 'You do not have a sheet yet.',
  'sheet.ownOpen': 'Open sheet',
  'sheet.ownRetry': 'Load again',
  'sheet.ownDelete': 'Delete sheet',
  'sheet.ownDeleteTitle': 'Delete your sheet?',
  'sheet.ownDeleteMessage':
    'Deleting your sheet removes it and its unsaved draft permanently. You cannot undo this.',
  'sheet.ownDeleteConfirm': 'Delete permanently',
  'sheet.ownCancel': 'Keep sheet',

  'engine.calculated': 'Calculated',
  'engine.expand': 'Expand section',
  'engine.collapse': 'Collapse section',
  'engine.none': 'None',
  'engine.modified': 'Modified',
  'engine.recalculated': 'Recalculated',
  'ui.dismiss': 'Dismiss',
} as const;

/** A key of the English catalog. */
export type MessageKey = keyof typeof messages;

/** Resolves the copy for [key] from the English catalog (API-23). */
export function message(key: MessageKey): string {
  return messages[key];
}
