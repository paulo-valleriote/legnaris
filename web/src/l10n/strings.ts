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
  'api.error.clientFault':
    'The app sent a request the server could not read. Reload and try again.',
  'api.error.invalidEdit': 'That value is not allowed, so it was put back. Try a different one.',
  'api.error.renderMismatch': 'This sheet no longer matches its template. Reload to continue.',
  'api.error.notFound': 'That sheet no longer exists. It may have been deleted.',
  'api.error.serverError': 'The server had a problem. Try again in a moment.',
  'api.error.offline': 'No connection to the server. Check your connection and try again.',
  'api.error.unknown': 'Something went wrong. Try again.',
  'api.error.parse': "The app could not read the server's answer. Try again in a moment.",
  'api.error.unauthorized': 'Your session has ended. Sign in again.',
  'api.error.forbidden': 'That action was refused. Reload the app and try again.',
  'api.error.rateLimited': 'Too many attempts. Wait a moment and try again.',
  'api.error.invalidCode':
    'That code is not valid or has expired. Check the newest email, or send a new code.',
  'api.error.emailUnavailable': 'We could not send the code right now. Try again in a moment.',
  'api.error.conflict': 'That email already has an account. Sign in instead.',
  'api.error.disposableEmail':
    'Use a permanent email address. Disposable addresses cannot receive your code.',

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
  'auth.deferredNotice':
    'Account created. Your code will be sent when email delivery is back to normal. Sign in later to confirm your email.',
  'auth.verifiedToast': 'Email confirmed',

  'app.name': 'Legnaris',
  'nav.catalog': 'Catalog',
  'nav.ownSheet': 'Your sheet',
  'nav.logout': 'Sign out',
  'nav.sectionWorkspace': 'Workspace',
  'nav.tagline': 'Character archive',

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
  'sheet.error.saveOffline': 'Offline. Changes are kept on this device. Save again later.',
  'sheet.error.saveOfflineUnkept':
    "Offline, and these changes could not be kept on this device. Keep this page open and save again when you're back.",
  'sheet.error.saveConflict':
    'This sheet is busy with another change. Wait a moment and save again.',
  'sheet.error.previewOffline': 'Offline. Changes are kept on this device. Preview again later.',
  'sheet.error.previewOfflineUnkept':
    "Offline, and these changes could not be kept on this device. Keep this page open and preview again when you're back.",
  'sheet.retry': 'Try again',

  'sheet.headerTitle': 'Sheet',
  'sheet.openLoading': 'Loading sheet',
  'sheet.templateLoading': 'Loading sheet template',
  'sheet.emptyFields': 'This template defines no fields.',
  'sheet.missing': 'This sheet no longer exists.',
  'sheet.retryOpen': 'Reload sheet',
  'sheet.pendingEdits': 'Unsaved changes',
  'sheet.statusUnsavedNone': 'Unsaved, none recalculated',
  'sheet.statusUnsavedCount': 'Unsaved, {count} recalculated',
  'sheet.save': 'Save',
  'sheet.saving': 'Saving',
  'sheet.saved': 'Saved',
  'sheet.preview': 'Preview',
  'sheet.previewing': 'Previewing',
  'sheet.recalculatedNone': 'No values recalculated',
  'sheet.recalculatedOne': '{count} value recalculated',
  'sheet.recalculatedMany': '{count} values recalculated',
  'sheet.jumpToFirst': 'Jump to first',
  'sheet.jumpToFirstName': 'Jump to first recalculated value',
  'sheet.sections': 'Sections',
  'sheet.pinnedOnly': 'Pinned only',
  'sheet.pinnedShowing': 'Showing pinned rows, {count}',
  'sheet.pinnedShowingAll': 'Showing all sections',
  'sheet.pinnedEmpty': 'Nothing pinned yet. Pin the rows you check most and they will be here.',
  'sheet.editPins': 'Edit pins',
  'sheet.help': 'Help',
  'sheet.helpTitle': 'How this sheet works',
  'sheet.helpDerived':
    'Flat numbers with no border are worked out for you from the rows you can edit.',
  'sheet.helpPreview': 'Preview shows what your edits change without saving them. Save keeps them.',
  'sheet.helpDismiss': 'Got it',
  'sheet.backToTop': '{name}, back to top',
  'sheet.sectionLink': 'Go to {label}',
  'sheet.sectionLinkChanged': 'Go to {label}, {count} changed',
  'sheet.collapseAll': 'Collapse all',
  'sheet.expandAll': 'Expand all',
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
  'engine.dependentStale': 'Not recalculated yet',
  'engine.sectionChanged': '{count} changed',
  'engine.none': 'None',
  'engine.modified': 'Modified',
  'engine.recalculated': 'Recalculated',
  'engine.pinned': 'Pinned',
  'engine.notSet': 'Not set',
  'engine.toggleSign': 'Toggle sign of {label}',
  'engine.decrease': 'Decrease {label}',
  'engine.increase': 'Increase {label}',

  'arrange.start': 'Arrange',
  'arrange.hint':
    'Drag handles to reorder sections. Star rows to pin them. Nothing saves until Done.',
  'arrange.done': 'Done',
  'arrange.saving': 'Saving arrangement',
  'arrange.cancel': 'Cancel',
  'arrange.reset': 'Reset to default',
  'arrange.pending': 'Unsaved arrangement',
  'arrange.pinLabel': 'Pin {label}',
  'arrange.handle': 'Reorder {label}',
  'arrange.position': '{label}, position {position} of {count}',
  'arrange.pinned': '{label} pinned',
  'arrange.unpinned': '{label} unpinned',
  'arrange.pickedUp': 'Picked up {label}, position {position} of {count}',
  'arrange.dropped': 'Dropped {label}, position {position} of {count}',
  'arrange.dragCancelled': 'Cancelled. {label} stays where it was',
  'arrange.instructions':
    'Press Space to lift, the arrow keys to move, Space to drop, Escape to cancel.',
  'arrange.toolbar': 'Arrange mode',
  'landing.create': 'Start your archive',
  'landing.heroTitle': 'A character sheet that does its own math.',
  'landing.heroLead':
    'Legnaris models any tabletop RPG as typed fields and formulas, so modifiers and skills recalculate the moment you change a number.',
  'landing.principlesTitle': 'Built for the table, not the spreadsheet.',
  'landing.principle1Title': 'No game content, ever',
  'landing.principle1Body':
    'Legnaris ships calculation mechanics and generic structure, never a publisher’s rules text. The structure is generic; the content is yours.',
  'landing.principle2Title': 'Derived values are computed',
  'landing.principle2Body':
    'A formula is written once. Modifiers, totals and skills follow every edit on their own, in the order they depend on each other.',
  'landing.principle3Title': 'Safe mid-session',
  'landing.principle3Body':
    'A crash or a dropped connection does not cost you the evening: pending edits are kept as a local draft until you save them.',
  'landing.principle4Title': 'Your data stays yours',
  'landing.principle4Body':
    'Export any sheet as self-contained JSON and import it back, or host Legnaris yourself. Your sheets outlive the service.',
  'landing.howTitle': 'Change one number. Watch the rest follow.',
  'landing.howStepsLabel': 'Steps',
  'landing.step1Title': 'Pick a template',
  'landing.step1Body':
    'Choose a reviewed template that describes your system as typed fields and formulas.',
  'landing.step2Title': 'Fill in the sheet',
  'landing.step2Body': 'Type the values only you know. Everything else is derived.',
  'landing.step3Title': 'Play',
  'landing.step3Body':
    'Edit mid-session, on your phone if you like. The sheet recalculates as you go.',
  'landing.demoLabel': 'Live example of a recalculating sheet',
  'landing.demoBase': 'Might',
  'landing.demoBaseDown': 'Lower Might',
  'landing.demoBaseUp': 'Raise Might',
  'landing.demoModifier': 'Modifier',
  'landing.demoClimbing': 'Climbing',
  'landing.demoCarry': 'Carry limit',
  'landing.demoFormulaModifier': '(Might − 10) ÷ 2, rounded down',
  'landing.demoFormulaClimbing': 'Modifier + 3',
  'landing.demoFormulaCarry': 'Might × 7',
  'landing.demoNote':
    'Demo values and formulas, not taken from any game. Push Might to either end.',
  'landing.demoNoteMax': 'Thirty. The ceiling, and the sheet did not blink.',
  'landing.demoNoteMin': 'One. Modifier −5, and the arithmetic still holds.',
  'landing.faqTitle': 'Questions, answered plainly.',
  'landing.faq1Question': 'Which game systems does it support?',
  'landing.faq1Answer':
    'Templates describe fields and formulas, so no system is hard-coded and no publisher content is included. Version 1 offers a reviewed catalog of templates; creating your own is not available yet.',
  'landing.faq2Question': 'Can I use it on my phone at the table?',
  'landing.faq2Answer':
    'Yes. Legnaris is an installable web app built for reading and editing a sheet quickly during a session.',
  'landing.faq3Question': 'What happens if I lose connection or the page crashes?',
  'landing.faq3Answer':
    'Your pending edits are kept as a local draft and restored the next time you open the sheet.',
  'landing.faq4Question': 'Can I take my sheet somewhere else?',
  'landing.faq4Answer':
    'Yes. Export a sheet as JSON, import it back later, or self-host the whole service.',
  'landing.faq5Question': 'Can I share a sheet with my table?',
  'landing.faq5Answer':
    'Not yet. Version 1 gives each account one sheet, with no sharing or game master tools.',
  'landing.closeTitle': 'Open your archive and let the sheet do the counting.',
  'landing.footerLabel': 'Site',
  'landing.copyright': '© {year} Legnaris',
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
