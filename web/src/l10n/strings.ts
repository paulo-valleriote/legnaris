/**
 * Typed English copy catalog — the only place a user-facing string lives in the
 * client (API-23).
 *
 * Every key is a plain string in the single V1 locale. Components and the API
 * client resolve copy through `message()` rather than embedding literals, so a
 * later localization feature replaces this module without touching call sites.
 * The copy that carries a value — the address, the remaining time, the account
 * mode — resolves through `messageWith()`, so the sentence and the value stay
 * in the catalog together (WCS-13).
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
  'api.error.invalidCode':
    'That code is not valid or has expired. Check the newest email, or send a new code.',
  'api.error.emailUnavailable': 'We could not send the code right now. Try again in a moment.',
  'api.error.conflict': 'That email already has an account. Sign in instead.',

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
  'auth.conflictSignIn': 'Sign in with this email instead',

  'auth.accountModeNew': 'new account',
  'auth.accountModeExisting': 'existing account',
  'auth.codeOrigin': 'Confirming your {mode}',
  'auth.codeTitleRegister': 'Confirm your email',
  'auth.codeTitleLogin': 'Check your email',
  'auth.codeInstruction': 'We sent a 6-digit code to {email}. It expires in {minutes} minutes.',
  'auth.codeInstructionExpired': 'We sent a 6-digit code to {email}. It has expired.',
  'auth.codeExpired': 'Send a new code or start over.',
  'auth.codeLabel': '6-digit code',
  'auth.codeSubmit': 'Verify',
  'auth.codeVerifying': 'Verifying…',
  'auth.codeResendHint': "Didn't get it?",
  'auth.codeResend': 'Resend code',
  'auth.codeResendReady': 'The resend action is available.',
  'auth.codeResendIn': 'Resend in {seconds}s',
  'auth.codeResendSent': 'We sent another code. Either code works.',
  'auth.codeBackRegister': 'Back to registration',
  'auth.codeBackLogin': 'Back to sign-in',

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

/** The values a parameterized catalog entry resolves (WCS-13). */
export type MessageValues = Readonly<Record<string, string | number>>;

/** Resolves the copy for [key] from the English catalog (API-23). */
export function message(key: MessageKey): string {
  return messages[key];
}

/**
 * Resolves a catalog entry that carries values (WCS-13).
 *
 * The catalog owns the sentence; the caller hands over the values it holds —
 * the address, the remaining time, the account mode — so no surface
 * concatenates copy with a value the catalog did not resolve. A placeholder
 * without a value is a programming error and raises rather than rendering a
 * half-resolved sentence.
 */
export function messageWith(key: MessageKey, values: MessageValues): string {
  return messages[key].replace(/\{(\w+)\}/g, (_match, name: string) => {
    const value = values[name];

    if (value === undefined) {
      throw new Error(`the copy "${key}" needs a value for "${name}"`);
    }

    return String(value);
  });
}
