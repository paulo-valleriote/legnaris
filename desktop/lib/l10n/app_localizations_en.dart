// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

  @override
  String get errorClientFault =>
      'Something went wrong with the request. Please report this issue.';

  @override
  String get errorInvalidEdit =>
      'This edit is not valid. The field was not changed.';

  @override
  String get errorRenderMismatch =>
      'The form does not match the template schema. Your changes were reverted.';

  @override
  String get errorNotFound => 'The requested item was not found.';

  @override
  String get errorServerError =>
      'The server had a problem. Please try again later.';

  @override
  String get errorOffline =>
      'You are offline. Check your connection and try again.';

  @override
  String get errorUnknown => 'Something went wrong. Please try again.';

  @override
  String get errorParse =>
      'The server sent an unexpected response. Please try again.';

  @override
  String get appTitle => 'Legnaris';

  @override
  String get navCatalog => 'Catalog';

  @override
  String get sectionConnection => 'Connection';

  @override
  String get sectionCatalog => 'Catalog';

  @override
  String get connectionChecking => 'Checking…';

  @override
  String get connectionOnline => 'Online';

  @override
  String get connectionOffline => 'Offline';

  @override
  String connectionLastCheck(String time) {
    return 'Last check $time';
  }

  @override
  String get connectionLastCheckNever => 'Last check never';

  @override
  String catalogCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count templates',
      one: '1 template',
    );
    return '$_temp0';
  }

  @override
  String catalogLastUpdated(String time) {
    return 'Last updated $time';
  }

  @override
  String get catalogLoading => 'Loading templates...';

  @override
  String get catalogEmpty => 'No templates available yet';

  @override
  String get actionRetry => 'Retry';

  @override
  String get catalogCreateAction => 'Create sheet';

  @override
  String get createSheetTemplateLabel => 'Template';

  @override
  String get templateDetailLoading => 'Loading template...';

  @override
  String get templateNoFields => 'This template has no fields.';

  @override
  String get createSheetPreviewLabel => 'Read-only preview';

  @override
  String get createSheetNameLabel => 'Sheet name';

  @override
  String get createSheetNamePlaceholder => 'e.g. My Character';

  @override
  String get createSheetPrimaryAction => 'Create sheet';

  @override
  String get createSheetNameRequiredTooltip => 'Enter a sheet name.';

  @override
  String get actionClose => 'Close';

  @override
  String get fieldCalculated => 'Calculated';

  @override
  String get fieldModified => 'Modified';

  @override
  String get fieldRecalculated => 'Recalculated';

  @override
  String get sheetLocalEdits => 'Local edits';

  @override
  String get sheetSaveAction => 'Save';

  @override
  String get sheetSaveNoChangesTooltip => 'There are no changes to save.';

  @override
  String get sheetPreviewAction => 'Preview changes';

  @override
  String get sheetPreviewNoChangesTooltip =>
      'There are no new changes to preview.';

  @override
  String get sheetRequestInProgressTooltip =>
      'Wait for the current request to finish.';

  @override
  String get sheetPreviewLimitTooltip =>
      'Preview limit reached. Try again in a minute.';

  @override
  String get sheetSaveLimitTooltip =>
      'Save limit reached. Try again in a minute.';

  @override
  String get sheetLoading => 'Loading sheet...';

  @override
  String get sheetLeaveDialogTitle => 'Discard local edits?';

  @override
  String get sheetLeaveDialogMessage =>
      'Your local edits on this sheet were never saved and will be discarded.';

  @override
  String get sheetLeaveConfirmAction => 'Discard and leave';

  @override
  String get sheetLeaveCancelAction => 'Keep editing';

  @override
  String get sheetRefreshDialogTitle => 'Discard local edits and reload?';

  @override
  String get sheetRefreshDialogMessage =>
      'Your local edits will be discarded and the latest template will be loaded.';

  @override
  String get sheetRefreshConfirmAction => 'Discard and reload';

  @override
  String get fieldNone => 'None';

  @override
  String get sheetMissingMessage =>
      'This sheet no longer exists in the API. It was removed from your library.';

  @override
  String get librarySectionTitle => 'My sheets';

  @override
  String get libraryEmpty => 'No sheets created yet';

  @override
  String get libraryOpenAction => 'Open';

  @override
  String get libraryRemoveAction => 'Remove';

  @override
  String get libraryRemoveDialogTitle => 'Remove from local library?';

  @override
  String get libraryRemoveDialogMessage =>
      'This removes the sheet from this app only. The sheet stays in the API and can be opened again if you keep its id.';

  @override
  String get libraryRemoveConfirmAction => 'Remove';

  @override
  String get libraryRemoveCancelAction => 'Cancel';

  @override
  String get libraryRemoveFailed =>
      'Could not update the local library. Please try again.';
}
