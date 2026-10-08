import 'dart:io';

import 'package:legnaris/sheet/draft_backup_store.dart';

/// Shared draft-backup fixtures for the suites that seed and verify the
/// session-end sweep (DATA-01 … DATA-04), so the seed shape lives in one
/// place instead of an identical copy in each test file.
const draftSheetId = '12345678-1234-7000-8000-123456789abc';

/// Stores an empty delta for [sheetId] through the production writer under
/// `<support>/drafts/` and returns its file, creating the directory when
/// needed.
File seedDraft(Directory support, [String sheetId = draftSheetId]) {
  DraftBackupStore(directory: support).write(sheetId, const {});
  return File('${support.path}/drafts/$sheetId.json');
}

/// The draft files currently stored under `<support>/drafts/`, empty when the
/// directory is absent.
List<File> draftFiles(Directory support) {
  final drafts = Directory('${support.path}/drafts');
  if (!drafts.existsSync()) return const [];
  return drafts.listSync().whereType<File>().toList();
}
