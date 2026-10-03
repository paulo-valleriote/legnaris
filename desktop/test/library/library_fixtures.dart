import 'package:legnaris/library/sheet_library_entry.dart';

/// Shared builder for the sheet-library test suites, so the entry defaults
/// (`Aria`, `fantasy_v1`, `2026-10-02T15:31:28.401Z`) live in one place
/// instead of an identical copy in each test file.
SheetLibraryEntry entry(
  String id, {
  String name = 'Aria',
  String templateId = 'fantasy_v1',
  String createdAt = '2026-10-02T15:31:28.401Z',
}) => SheetLibraryEntry(
  id: id,
  name: name,
  templateId: templateId,
  createdAt: DateTime.parse(createdAt),
);
