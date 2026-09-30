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
}
