/// Versioned JSON persistence for the local sheet library index.
library;

import 'dart:convert';
import 'dart:io';

import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';

/// The persistence boundary for the sheet index: a single versioned JSON file
/// in an injected [Directory], written through a temp-file + rename replace.
///
/// The store is stateless over entries: the caller passes the complete list on
/// every save. It never persists the sheet `data` payload (LIB-08).
class SheetLibraryStore {
  SheetLibraryStore({
    required this.directory,
    this.fileName = 'sheet_library.json',
  });

  /// The schema version this build writes and understands; migrations are
  /// keyed by the version being migrated from.
  static const int currentSchemaVersion = 1;

  /// The directory holding the store file; injected so tests never touch the
  /// `path_provider` platform channel.
  final Directory directory;

  /// The store file name inside [directory].
  final String fileName;

  File get _file => File('${directory.path}/$fileName');

  File get _tempFile => File('${_file.path}.tmp');

  /// Validates [entries], then atomically replaces the store with
  /// `{schema_version, sheets}` (LIB-09 … LIB-13, LIB-20).
  ///
  /// A rejected entry or a failed write throws [SheetLibraryException] and
  /// leaves any existing store file untouched; the write goes to a temporary
  /// file that is renamed over the target, so a partial write never truncates
  /// the store.
  void save(List<SheetLibraryEntry> entries) {
    for (final entry in entries) {
      if (!entry.isValid) {
        throw SheetLibraryException(
          SheetLibraryErrorKind.invalidEntry,
          'entry "${entry.id}" is missing id, name or template_id',
        );
      }
    }

    final envelope = <String, Object?>{
      'schema_version': currentSchemaVersion,
      'sheets': [for (final entry in entries) entry.toJson()],
    };

    try {
      if (!directory.existsSync()) {
        directory.createSync(recursive: true);
      }
      _tempFile.writeAsStringSync(jsonEncode(envelope), flush: true);
      _tempFile.renameSync(_file.path);
    } on FileSystemException catch (error) {
      throw SheetLibraryException(
        SheetLibraryErrorKind.writeFailed,
        'failed to persist the sheet library: ${error.message}',
      );
    }
  }
}
