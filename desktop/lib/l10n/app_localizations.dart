import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_en.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'l10n/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations? of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations);
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[Locale('en')];

  /// User-facing copy for ApiErrorKind.clientFault (apperr INVALID_JSON).
  ///
  /// In en, this message translates to:
  /// **'Something went wrong with the request. Please report this issue.'**
  String get errorClientFault;

  /// User-facing copy for ApiErrorKind.invalidEdit (apperr INVALID_REQUEST).
  ///
  /// In en, this message translates to:
  /// **'This edit is not valid. The field was not changed.'**
  String get errorInvalidEdit;

  /// User-facing copy for ApiErrorKind.renderMismatch (apperr UNKNOWN_FIELD).
  ///
  /// In en, this message translates to:
  /// **'The form does not match the template schema. Your changes were reverted.'**
  String get errorRenderMismatch;

  /// User-facing copy for ApiErrorKind.notFound (apperr NOT_FOUND).
  ///
  /// In en, this message translates to:
  /// **'The requested item was not found.'**
  String get errorNotFound;

  /// User-facing copy for ApiErrorKind.serverError (apperr INTERNAL_ERROR).
  ///
  /// In en, this message translates to:
  /// **'The server had a problem. Please try again later.'**
  String get errorServerError;

  /// User-facing copy for ApiErrorKind.offline (transport failure after retries).
  ///
  /// In en, this message translates to:
  /// **'You are offline. Check your connection and try again.'**
  String get errorOffline;

  /// Generic fallback copy for ApiErrorKind.unknown (unmapped apperr codes).
  ///
  /// In en, this message translates to:
  /// **'Something went wrong. Please try again.'**
  String get errorUnknown;

  /// Defensive copy for parse failures: broken JSON, a missing data key, or a payload that does not match the expected model (CAT-14).
  ///
  /// In en, this message translates to:
  /// **'The server sent an unexpected response. Please try again.'**
  String get errorParse;

  /// App title shown in the shell header.
  ///
  /// In en, this message translates to:
  /// **'Legnaris'**
  String get appTitle;

  /// Left navigation entry for the template catalog.
  ///
  /// In en, this message translates to:
  /// **'Catalog'**
  String get navCatalog;

  /// Right panel section reserved for the connection indicator.
  ///
  /// In en, this message translates to:
  /// **'Connection'**
  String get sectionConnection;

  /// Right panel section showing catalog statistics.
  ///
  /// In en, this message translates to:
  /// **'Catalog'**
  String get sectionCatalog;

  /// Connection state label while no health response has arrived yet (CONN-02, CONN-11).
  ///
  /// In en, this message translates to:
  /// **'Checking…'**
  String get connectionChecking;

  /// Connection state label after a successful health check (CONN-03, CONN-11).
  ///
  /// In en, this message translates to:
  /// **'Online'**
  String get connectionOnline;

  /// Connection state label after a failed health check (CONN-04, CONN-11).
  ///
  /// In en, this message translates to:
  /// **'Offline'**
  String get connectionOffline;

  /// Right panel line with the timestamp of the last successful health check (CONN-09, CONN-11).
  ///
  /// In en, this message translates to:
  /// **'Last check {time}'**
  String connectionLastCheck(String time);

  /// Right panel line shown before the first successful health check (CONN-09, CONN-11).
  ///
  /// In en, this message translates to:
  /// **'Last check never'**
  String get connectionLastCheckNever;

  /// Number of loaded templates shown in the right panel.
  ///
  /// In en, this message translates to:
  /// **'{count, plural, =1{1 template} other{{count} templates}}'**
  String catalogCount(int count);

  /// Time of the last successful catalog load shown in the right panel.
  ///
  /// In en, this message translates to:
  /// **'Last updated {time}'**
  String catalogLastUpdated(String time);

  /// Loading state of the catalog screen while the first request is in flight (CAT-08).
  ///
  /// In en, this message translates to:
  /// **'Loading templates...'**
  String get catalogLoading;

  /// Single line rendered by the empty catalog state; no decorative art (CAT-10).
  ///
  /// In en, this message translates to:
  /// **'No templates available yet'**
  String get catalogEmpty;

  /// Action that re-issues the catalog request from an error state (CAT-11, CAT-12).
  ///
  /// In en, this message translates to:
  /// **'Retry'**
  String get actionRetry;

  /// Card action that opens the creation dialog for that template (wireframe 'criar ficha').
  ///
  /// In en, this message translates to:
  /// **'Create sheet'**
  String get catalogCreateAction;

  /// Label above the template identity shown in the creation dialog (CAT-15, TPL-02).
  ///
  /// In en, this message translates to:
  /// **'Template'**
  String get createSheetTemplateLabel;

  /// Loading state of the creation dialog while GET /templates/:id is in flight (TPL-01).
  ///
  /// In en, this message translates to:
  /// **'Loading template...'**
  String get templateDetailLoading;

  /// Shown in place of the prefix summary when the template fields tree is empty (TPL-02 edge).
  ///
  /// In en, this message translates to:
  /// **'This template has no fields.'**
  String get templateNoFields;

  /// Header of the read-only field preview card in the creation dialog (PRV-01, PRV-06).
  ///
  /// In en, this message translates to:
  /// **'Read-only preview'**
  String get createSheetPreviewLabel;

  /// Label of the sheet-name input in the creation dialog (TPL-11).
  ///
  /// In en, this message translates to:
  /// **'Sheet name'**
  String get createSheetNameLabel;

  /// Example placeholder of the empty sheet-name input (TPL-11).
  ///
  /// In en, this message translates to:
  /// **'e.g. My Character'**
  String get createSheetNamePlaceholder;

  /// Primary action of the creation dialog; enabled once a non-empty sheet name is entered (CRT-01, TPL-12).
  ///
  /// In en, this message translates to:
  /// **'Create sheet'**
  String get createSheetPrimaryAction;

  /// Tooltip explaining why the primary action is disabled: the sheet name is empty (CRT-05).
  ///
  /// In en, this message translates to:
  /// **'Enter a sheet name.'**
  String get createSheetNameRequiredTooltip;

  /// Action that dismisses the creation dialog back to the catalog (CAT-16).
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get actionClose;

  /// Read-only highlight shown next to a non-editable (derived) field value (RND-10).
  ///
  /// In en, this message translates to:
  /// **'Calculated'**
  String get fieldCalculated;

  /// Row mark shown left of an editable field the pending delta changed, after a successful preview (PRV-12).
  ///
  /// In en, this message translates to:
  /// **'Modified'**
  String get fieldModified;

  /// Row mark shown left of a derived field the last successful preview recalculated (PRV-13).
  ///
  /// In en, this message translates to:
  /// **'Recalculated'**
  String get fieldRecalculated;

  /// Footer caption shown while the in-memory edit state differs from the seeded defaults (GRN-11, GRN-12).
  ///
  /// In en, this message translates to:
  /// **'Local edits'**
  String get sheetLocalEdits;

  /// Footer action that commits the changed editable delta with one PATCH /sheets/:id (SAV-01, SAV-04).
  ///
  /// In en, this message translates to:
  /// **'Save'**
  String get sheetSaveAction;

  /// Tooltip explaining why the Save action is disabled: no pending edit (SAV-02).
  ///
  /// In en, this message translates to:
  /// **'There are no changes to save.'**
  String get sheetSaveNoChangesTooltip;

  /// Secondary footer action that runs one stateless dry-run and merges the cascade without saving (PRV-03).
  ///
  /// In en, this message translates to:
  /// **'Preview changes'**
  String get sheetPreviewAction;

  /// Tooltip explaining why Preview is disabled: the pending delta already matches the last successful preview (PRV-02).
  ///
  /// In en, this message translates to:
  /// **'There are no new changes to preview.'**
  String get sheetPreviewNoChangesTooltip;

  /// Tooltip explaining why both Preview and Save are disabled: a request is in flight (SVE-01).
  ///
  /// In en, this message translates to:
  /// **'Wait for the current request to finish.'**
  String get sheetRequestInProgressTooltip;

  /// Tooltip explaining why Preview is disabled: three previews were issued inside the rolling window (SVE-02).
  ///
  /// In en, this message translates to:
  /// **'Preview limit reached. Try again in a minute.'**
  String get sheetPreviewLimitTooltip;

  /// Tooltip explaining why Save is disabled: three saves were issued inside the rolling window (SVE-03).
  ///
  /// In en, this message translates to:
  /// **'Save limit reached. Try again in a minute.'**
  String get sheetSaveLimitTooltip;

  /// Loading state of Screen 2 while GET /sheets/:id is in flight (OPN-05).
  ///
  /// In en, this message translates to:
  /// **'Loading sheet...'**
  String get sheetLoading;

  /// Title of the destructive confirmation opened by the sheet back action while edits are pending (GRN-13).
  ///
  /// In en, this message translates to:
  /// **'Discard local edits?'**
  String get sheetLeaveDialogTitle;

  /// Explicit consequence stated by the sheet leave confirmation dialog (GRN-13).
  ///
  /// In en, this message translates to:
  /// **'Your local edits on this sheet were never saved and will be discarded.'**
  String get sheetLeaveDialogMessage;

  /// Destructive button of the leave confirmation: discards the edit state and navigates back (GRN-14).
  ///
  /// In en, this message translates to:
  /// **'Discard and leave'**
  String get sheetLeaveConfirmAction;

  /// Safe button of the discard confirmations (leave and refresh): closes the dialog and keeps the screen with its edit state (GRN-15, GRN-19).
  ///
  /// In en, this message translates to:
  /// **'Keep editing'**
  String get sheetLeaveCancelAction;

  /// Title of the confirmation opened by F5 on a sheet with pending local edits (GRN-18).
  ///
  /// In en, this message translates to:
  /// **'Discard local edits and reload?'**
  String get sheetRefreshDialogTitle;

  /// Explicit consequence stated by the F5 refresh confirmation dialog (GRN-18).
  ///
  /// In en, this message translates to:
  /// **'Your local edits will be discarded and the latest template will be loaded.'**
  String get sheetRefreshDialogMessage;

  /// Destructive button of the refresh confirmation: discards the edit state and refetches the template detail (GRN-17, GRN-18).
  ///
  /// In en, this message translates to:
  /// **'Discard and reload'**
  String get sheetRefreshConfirmAction;

  /// Option label of an exclusive-group dropdown entry that clears the group when no member has an active default (EXC-10, EXC-27).
  ///
  /// In en, this message translates to:
  /// **'None'**
  String get fieldNone;

  /// Shown on Screen 2 when GET /sheets/:id returns NOT_FOUND and the local entry was evicted (OPN-06).
  ///
  /// In en, this message translates to:
  /// **'This sheet no longer exists in the API. It was removed from your library.'**
  String get sheetMissingMessage;

  /// Header of the local sheet library section on Screen 1 (OPN-01).
  ///
  /// In en, this message translates to:
  /// **'My sheets'**
  String get librarySectionTitle;

  /// Single line rendered by the empty library section; no decorative art (OPN-02).
  ///
  /// In en, this message translates to:
  /// **'No sheets created yet'**
  String get libraryEmpty;

  /// Library row action that opens the stored sheet on Screen 2 (OPN-04).
  ///
  /// In en, this message translates to:
  /// **'Open'**
  String get libraryOpenAction;

  /// Library row action that removes the entry from the local index (OPN-12).
  ///
  /// In en, this message translates to:
  /// **'Remove'**
  String get libraryRemoveAction;

  /// Title of the destructive confirmation for removing a library entry (OPN-12).
  ///
  /// In en, this message translates to:
  /// **'Remove from local library?'**
  String get libraryRemoveDialogTitle;

  /// Explicit consequence stated by the library removal confirmation: local-only, sheet stays in the API (OPN-12).
  ///
  /// In en, this message translates to:
  /// **'This removes the sheet from this app only. The sheet stays in the API and can be opened again if you keep its id.'**
  String get libraryRemoveDialogMessage;

  /// Destructive button of the removal confirmation (OPN-13).
  ///
  /// In en, this message translates to:
  /// **'Remove'**
  String get libraryRemoveConfirmAction;

  /// Safe button of the removal confirmation; keeps the entry (OPN-14).
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get libraryRemoveCancelAction;

  /// Inline error shown when a library removal fails to persist; the entry stays (OPN-15).
  ///
  /// In en, this message translates to:
  /// **'Could not update the local library. Please try again.'**
  String get libraryRemoveFailed;

  /// Mode switch tab that selects the login form (FE6-05).
  ///
  /// In en, this message translates to:
  /// **'Log in'**
  String get authModeLogin;

  /// Mode switch tab that selects the registration form (FE6-04).
  ///
  /// In en, this message translates to:
  /// **'Register'**
  String get authModeRegister;

  /// Label of the email input on the auth screen (FE6-03, FE6-05).
  ///
  /// In en, this message translates to:
  /// **'Email'**
  String get authEmailLabel;

  /// Label of the password input on the auth screen (FE6-03, FE6-05).
  ///
  /// In en, this message translates to:
  /// **'Password'**
  String get authPasswordLabel;

  /// Label of the username input shown only in register mode (FE6-04, FE6-21).
  ///
  /// In en, this message translates to:
  /// **'Username'**
  String get authUsernameLabel;

  /// Primary action that submits the login form (FE6-03).
  ///
  /// In en, this message translates to:
  /// **'Log in'**
  String get authSubmitLogin;

  /// Primary action that submits the registration form (FE6-04).
  ///
  /// In en, this message translates to:
  /// **'Create account'**
  String get authSubmitRegister;

  /// Shell header action that ends the session and returns to the auth screen (FE6-07, FE6-19).
  ///
  /// In en, this message translates to:
  /// **'Log out'**
  String get authLogoutAction;
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['en'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'en':
      return AppLocalizationsEn();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
