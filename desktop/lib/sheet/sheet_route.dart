import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/sheet/sheet_open_provider.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
import 'package:legnaris/shell/app_shell.dart';

/// Opens Screen 2 for the stored sheet [sheetId] as a route on the nearest
/// navigator (OPN-04). The route mounts the persistent shell around the sheet
/// surface; Screen 2's back action pops it, returning to the surface beneath —
/// the catalog or the post-login base — with no extra request. A [sheet]
/// injects the created `201` payload so the screen skips the `GET` (OPN-09);
/// the library path passes none and Screen 2 fetches by id.
///
/// This is the single route shape for Screen 2: the catalog open, the creation
/// flow and the post-login open all push through here, so every session pushes
/// the identical route.
Future<void> pushSheetScreen(
  BuildContext context, {
  required String sheetId,
  Sheet? sheet,
}) {
  return Navigator.of(context).push(
    MaterialPageRoute<void>(
      builder: (_) => ProviderScope(
        overrides: [
          if (sheet != null) ...[
            openedSheetProvider.overrideWithValue(sheet),
            // A nested scope shares non-overridden providers with its
            // parent, so the open and edit controllers must be created in
            // this scope to observe the opened-sheet seed above (design:
            // opened-sheet seam).
            sheetOpenControllerProvider.overrideWith2(
              (sheetId) => SheetOpenController(),
            ),
            sheetEditControllerProvider.overrideWith2(
              (sheetId) => SheetEditController(),
            ),
          ],
        ],
        child: AppShell(content: SheetScreen(sheetId: sheetId)),
      ),
    ),
  );
}
