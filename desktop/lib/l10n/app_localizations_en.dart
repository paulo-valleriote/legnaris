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
  String get createSheetNameLabel => 'Sheet name';

  @override
  String get createSheetNamePlaceholder => 'e.g. My Character';

  @override
  String get createSheetPrimaryAction => 'Create sheet';

  @override
  String get createSheetCreateTooltip =>
      'Sheet creation arrives in a later phase.';

  @override
  String get actionClose => 'Close';
}
