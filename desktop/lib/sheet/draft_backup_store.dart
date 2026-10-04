/// Atomic per-sheet persistence of an unsaved draft delta (DRB-04, DRB-05,
/// DRB-12, DRB-14).
///
/// One versioned JSON file per sheet under `<appSupport>/drafts/<sheet_id>.json`
/// holds only the pending editable delta the caller passes; derived values are
/// never written here. A write goes to a per-process temp file that is renamed
/// over the target, so a failed write leaves any prior backup intact.
library;

import 'dart:convert';
import 'dart:io';

import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'draft_backup_store.g.dart';

/// Raised when a draft backup cannot be written. Reads and deletes are
/// best-effort and never throw this, so only a write surfaces a failure.
class DraftBackupException implements Exception {
  const DraftBackupException(this.message);

  /// Description of what failed.
  final String message;

  @override
  String toString() => 'DraftBackupException: $message';
}

/// Read, atomically write and delete one sheet's draft delta under an injected
/// application-support [directory].
///
/// `read` never throws: a missing, corrupt or newer-version file resolves to
/// null so the caller seeds cleanly from the stored sheet (DRB-12). `delete`
/// is best-effort (DRB-14).
class DraftBackupStore {
  DraftBackupStore({
    required this.directory,
    DateTime Function()? now,
  }) : _now = now ?? DateTime.now;

  /// The schema version this build writes and understands.
  static const int currentSchemaVersion = 1;

  /// The application-support directory; injected so tests never touch the
  /// `path_provider` platform channel.
  final Directory directory;

  final DateTime Function() _now;

  /// Validates that [sheetId] is a valid UUID (v4 or v7) to prevent path
  /// traversal when used in file paths. Returns true if valid.
  static bool _isValidSheetId(String sheetId) {
    // UUID regex: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (lowercase only per idgen)
    return RegExp(
      r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
    ).hasMatch(sheetId);
  }

  Directory get _draftsDirectory => Directory('${directory.path}/drafts');

  File _file(String sheetId) {
    if (!_isValidSheetId(sheetId)) {
      throw ArgumentError.value(
        sheetId,
        'sheetId',
        'must be a valid UUID (lowercase) to be used in a file path',
      );
    }
    return File('${_draftsDirectory.path}/$sheetId.json');
  }

  /// Temp file for this process: the pid suffix keeps two app instances from
  /// writing (and renaming) the same in-flight file.
  File _tempFile(String sheetId) => File('${_file(sheetId).path}.$pid.tmp');

  /// Returns the stored delta for [sheetId], or null when the file is missing,
  /// cannot be read, is not a valid envelope, or carries an unsupported schema
  /// version (DRB-12).
  Map<String, Object?>? read(String sheetId) {
    final file = _file(sheetId);
    if (!file.existsSync()) {
      return null;
    }
    try {
      final decoded = jsonDecode(file.readAsStringSync());
      if (decoded is! Map<String, Object?>) {
        return null;
      }
      if (decoded['schema_version'] != currentSchemaVersion) {
        return null;
      }
      final delta = decoded['delta'];
      if (delta is! Map<String, Object?>) {
        return null;
      }
      return delta;
    } on FormatException {
      return null;
    } on FileSystemException {
      return null;
    }
  }

  /// Atomically replaces [sheetId]'s backup with the versioned envelope
  /// `{schema_version, sheet_id, delta, saved_at}`, throwing
  /// [DraftBackupException] on an unwritable path; the previous file is left
  /// intact because the write lands on a temp file first (DRB-04, DRB-05).
  void write(String sheetId, Map<String, Object?> delta) {
    final envelope = <String, Object?>{
      'schema_version': currentSchemaVersion,
      'sheet_id': sheetId,
      'delta': delta,
      'saved_at': _now().toUtc().toIso8601String(),
    };
    final temp = _tempFile(sheetId);
    try {
      if (!_draftsDirectory.existsSync()) {
        _draftsDirectory.createSync(recursive: true);
      }
      temp.writeAsStringSync(jsonEncode(envelope), flush: true);
      temp.renameSync(_file(sheetId).path);
    } on FileSystemException catch (error) {
      throw DraftBackupException(
        'failed to persist the draft for "$sheetId": ${error.message}',
      );
    }
  }

  /// Deletes [sheetId]'s backup best-effort; an absent file or a filesystem
  /// failure is a no-op (DRB-14).
  void delete(String sheetId) {
    try {
      final file = _file(sheetId);
      if (file.existsSync()) {
        file.deleteSync();
      }
    } on FileSystemException {
      // Best-effort: a failed delete never blocks the caller.
    }
  }
}

/// The draft backup boundary over the injected app-support directory.
@Riverpod(keepAlive: true)
DraftBackupStore draftBackupStore(Ref ref) =>
    DraftBackupStore(directory: ref.watch(appSupportDirectoryProvider));
