import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/theme/app_theme.dart';
import 'package:legnaris/theme/tokens.dart';

/// Widget tests for the creation dialog layout (LGN-110: DIA-01 … DIA-04).
///
/// The dialog content used to overflow short windows, so the preview card and
/// the Create/Close action row painted over each other. These tests mount the
/// dialog at the default, the 720px-tall common window and the minimum
/// supported size, and assert the action row never intersects the preview.
/// Every asserted value comes from the spec rule, never the implementation.
void main() {
  const listBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';

  /// A template large enough to push the preview against its height cap.
  String bigDetailBody(int count) {
    final leaves = [
      for (var i = 0; i < count; i++)
        '"field$i": {"schema": {"type": "number", "label": "Field $i", "is_editable": true, "default": $i}}',
    ].join(',');
    return '{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"stats": {"children": {$leaves}}}}}';
  }

  Future<void> pumpDialogAt(WidgetTester tester, Size viewSize) async {
    final supportDir = Directory.systemTemp.createTempSync('lgn110-layout');
    addTearDown(() => supportDir.deleteSync(recursive: true));
    final detail = bigDetailBody(30);
    tester.view.physicalSize = viewSize;
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(supportDir),
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient(
                (request) async => http.Response(
                  request.url.path == '/templates' ? listBody : detail,
                  200,
                ),
              ),
            ),
          ),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const CatalogScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fantasy Template'));
    await tester.pumpAndSettle();
  }

  Future<void> expectNoOverlap(WidgetTester tester) async {
    expect(
      tester.takeException(),
      isNull,
      reason: 'the dialog content must not overflow the viewport',
    );
    final submit = find.byKey(const Key('create-sheet-submit'));
    final preview = find.byKey(const Key('create-sheet-preview'));
    expect(submit, findsOneWidget);
    expect(preview, findsOneWidget);
    final submitRect = tester.getRect(submit);
    final previewRect = tester.getRect(preview);
    expect(
      submitRect.overlaps(previewRect),
      isFalse,
      reason:
          'the Create action ($submitRect) must not overlap the preview '
          '($previewRect)',
    );
  }

  group('creation dialog action vs preview (DIA-01 … DIA-04)', () {
    testWidgets('Happy path: default size keeps the action clear of the preview (DIA-01)',
        (tester) async {
      await pumpDialogAt(tester, const Size(1280, 800));
      await expectNoOverlap(tester);
    });

    testWidgets('Edge: the common 720px window keeps the action clear (DIA-02, DIA-03)',
        (tester) async {
      await pumpDialogAt(tester, const Size(1280, 720));
      await expectNoOverlap(tester);
    });

    testWidgets('Edge: the minimum supported size stays reachable and scrolls (DIA-02, DIA-04)',
        (tester) async {
      await pumpDialogAt(tester, const Size(640, 480));
      await expectNoOverlap(tester);

      await tester.enterText(
        find.byKey(const Key('create-sheet-name-input')),
        'Reachable',
      );
      await tester.pump();
      final submit = find.byKey(const Key('create-sheet-submit'));
      expect(
        tester.widget<TextButton>(submit).onPressed,
        isNotNull,
        reason: 'the Create action must be reachable at the minimum size',
      );
      final submitRect = tester.getRect(submit);
      expect(submitRect.top, greaterThanOrEqualTo(0));
      expect(submitRect.bottom, lessThanOrEqualTo(480));

      final scroll = find.descendant(
        of: find.byKey(const Key('create-sheet-preview')),
        matching: find.byType(SingleChildScrollView),
      );
      expect(scroll, findsOneWidget);
      expect(
        tester.getSize(scroll).height,
        lessThanOrEqualTo(AppSizes.previewCardMaxHeight),
      );
      final controller =
          tester.widget<SingleChildScrollView>(scroll).controller!;
      expect(controller.position.maxScrollExtent, greaterThan(0));
    });
  });
}
