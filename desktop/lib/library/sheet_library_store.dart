/// Versioned JSON persistence for the local sheet library index.
library;

import 'dart:convert';
import 'dart:io';

import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';

/// Migrates a persisted envelope from one schema version to the next; keyed by
/// the version being migrated from.
typedef JsonMigrator = Map<String, Object?> Function(
  Map<String, Object?> envelope,
);

/// How the store loaded, reported to the caller instead of throwing (LIB-19).
enum LibraryLoadOutcome { loaded, quarantined, empty }

/// The result of [SheetLibraryStore.load]: the outcome and, when loaded, the
/// stored entries. Never carries sheet data.
class LibraryLoadResult {
  const LibraryLoadResult({required this.outcome, required this.entries});

  final LibraryLoadOutcome outcome;

  final List<SheetLibraryEntry> entries;
}

/// The persistence boundary for the sheet index: a single versioned JSON file
/// in an injected [Directory], written through a temp-file + rename replace.
///
/// The store is stateless over entries: the caller passes the complete list on
/// every save. It never persists the sheet `data` payload (LIB-08).
class SheetLibraryStore {
  SheetLibraryStore({
    required this.directory,
    this.fileName = 'sheet_library.json',
    this.migrations = const {},
    /// Clock seam: a test that forces a quarantine rename failure must know
    /// the stamped destination name in advance.
    DateTime Function()? now,
  }) : _now = now ?? DateTime.now;

  /// The schema version this build writes and understands.
  static const int currentSchemaVersion = 1;

  /// The directory holding the store file; injected so tests never touch the
  /// `path_provider` platform channel.
  final Directory directory;

  /// The store file name inside [directory].
  final String fileName;

  /// Migration steps keyed by the version being migrated from.
  ///
  /// SPEC_DEVIATION: injected per store rather than a static const map
  /// (design) so a test can register a migration for LIB-15 without mutating
  /// global state.
  final Map<int, JsonMigrator> migrations;

  final DateTime Function() _now;

  File get _file => File('${directory.path}/$fileName');

  /// The temp file for this process: the pid suffix keeps two app instances
  /// from writing (and renaming) the same in-flight file.
  File get _tempFile => File('${_file.path}.$pid.tmp');

  /// Loads the index, applying version recovery (LIB-01, LIB-14 … LIB-19).
  ///
  /// Never throws for bad input: a corrupt, malformed, or unsupported store is
  /// quarantined with its original bytes preserved under a distinct name, and
  /// reported as [LibraryLoadOutcome.quarantined] with an empty index.
  LibraryLoadResult load() {
    _deleteLeftoverTemp();

    if (!_file.existsSync()) {
      return const LibraryLoadResult(
        outcome: LibraryLoadOutcome.empty,
        entries: [],
      );
    }

    try {
      return LibraryLoadResult(
        outcome: LibraryLoadOutcome.loaded,
        entries: List.unmodifiable(_readEntries()),
      );
    } on FormatException {
      return _quarantine();
    } on FileSystemException {
      return _quarantine();
    }
  }

  /// Decodes the store file and parses every entry, applying the migration
  /// chain; throws [FormatException] on any shape this build cannot read and
  /// [FileSystemException] when the file itself cannot be read, so [load]
  /// quarantines every bad-input case in a single place.
  List<SheetLibraryEntry> _readEntries() {
    final decoded = jsonDecode(_file.readAsStringSync());
    if (decoded is! Map<String, Object?>) {
      throw const FormatException('store root is not a JSON object');
    }
    final envelope = decoded;

    final version = envelope['schema_version'];
    if (version is! int) {
      throw const FormatException('schema_version is not an integer');
    }
    if (version > currentSchemaVersion) {
      throw FormatException(
        'schema_version $version is newer than $currentSchemaVersion',
      );
    }

    var migrated = envelope;
    for (var from = version; from < currentSchemaVersion; from++) {
      final migrator = migrations[from];
      if (migrator == null) {
        throw FormatException('no migration registered from version $from');
      }
      migrated = migrator(migrated);
    }

    final rawSheets = migrated['sheets'];
    if (rawSheets is! List) {
      throw const FormatException('sheets is not a list');
    }
    final entries = <SheetLibraryEntry>[];
    for (final raw in rawSheets) {
      if (raw is! Map<String, Object?>) {
        throw const FormatException('entry is not a JSON object');
      }
      final entry = SheetLibraryEntry.fromJson(raw);
      if (!entry.isValid) {
        throw FormatException('entry "${entry.id}" is missing fields');
      }
      entries.add(entry);
    }
    return entries;
  }

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

  /// Deletes leftover temp files of this store — this process's own residue
  /// and crash residue from earlier runs, which carry another pid and can
  /// only be recognized by the `<file>[.<pid>].tmp` name pattern; cleanup is
  /// best effort so a stale temp never blocks a valid load. Sweeping every
  /// matching file at load is safe inside AC5's single-process scope: a
  /// second instance saving at that exact moment would see a typed
  /// `writeFailed`, never a corrupted store.
  void _deleteLeftoverTemp() {
    final pattern = RegExp(
      '(?:^|[/\\\\])${RegExp.escape(fileName)}(?:\\.\\d+)?\\.tmp\$',
    );
    try {
      for (final entity in directory.listSync()) {
        if (entity is File && pattern.hasMatch(entity.path)) {
          entity.deleteSync();
        }
      }
    } on FileSystemException {
      return;
    }
  }

  /// Moves a bad store aside under `<file>.corrupt-<UTC compact timestamp>`,
  /// preserving its bytes, and reports [LibraryLoadOutcome.quarantined]. If
  /// the rename itself fails nothing was quarantined: the file stays in place
  /// for the next successful save to overwrite and [LibraryLoadOutcome.empty]
  /// is reported instead (design).
  LibraryLoadResult _quarantine() {
    try {
      final stamp = _now()
          .toUtc()
          .toIso8601String()
          .replaceAll(RegExp(r'[^0-9]'), '');
      _file.renameSync('${_file.path}.corrupt-$stamp');
    } on FileSystemException {
      return const LibraryLoadResult(
        outcome: LibraryLoadOutcome.empty,
        entries: [],
      );
    }
    return const LibraryLoadResult(
      outcome: LibraryLoadOutcome.quarantined,
      entries: [],
    );
  }
}
