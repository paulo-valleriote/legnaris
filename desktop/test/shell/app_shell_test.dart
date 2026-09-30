import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/shell/left_nav.dart';
import 'package:legnaris/shell/right_panel.dart';
import 'package:legnaris/shell/shell_state.dart';
import 'package:legnaris/theme/app_theme.dart';
import 'package:legnaris/theme/tokens.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart' show Override;

void main() {
  const twoTemplatesBody = '''
{"data": [
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}},
  {"id": "sci_fi_v1", "name": "Sci Fi Template", "default": false, "fields": {}}
]}
''';

  Future<List<http.Request>> pumpShell(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? handler,
    Size viewport = const Size(1280, 800),
    List<Override> extraOverrides = const [],
  }) async {
    final recorded = <http.Request>[];
    tester.view.physicalSize = viewport;
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                return handler == null
                    ? http.Response(twoTemplatesBody, 200)
                    : handler(request);
              }),
            ),
          ),
          ...extraOverrides,
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: AppShell(
            content: SizedBox(
              key: const Key('shell-content'),
              width: double.infinity,
              height: AppSizes.controlHeight,
              child: const Text('content-body'),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return recorded;
  }

  group('layout (CAT-01 … CAT-03)', () {
    testWidgets('Edge: below the threshold the panel hides first and the nav keeps its width (CAT-03)', (tester) async {
      await pumpShell(tester, viewport: const Size(999, 800));

      expect(find.byType(RightPanel), findsNothing);
      expect(find.byType(LeftNav), findsOneWidget);
      expect(tester.getSize(find.byType(LeftNav)).width, AppSizes.leftNavWidth);
      expect(find.byKey(const Key('shell-content')), findsOneWidget);
    });

    testWidgets('Edge: shrinking and restoring the window issues no new catalog request (CAT-03 edge)', (tester) async {
      final recorded = await pumpShell(tester);
      expect(find.byType(RightPanel), findsOneWidget);
      expect(recorded, hasLength(1));

      tester.view.physicalSize = const Size(999, 800);
      await tester.pump();
      expect(find.byType(RightPanel), findsNothing);

      tester.view.physicalSize = const Size(1280, 800);
      await tester.pump();
      expect(find.byType(RightPanel), findsOneWidget);
      expect(recorded, hasLength(1));
    });

    testWidgets('Happy path: shell renders nav, header, content and right panel (CAT-01)', (tester) async {
      await pumpShell(tester);

      expect(find.byType(LeftNav), findsOneWidget);
      expect(find.byType(Header), findsOneWidget);
      expect(find.byType(RightPanel), findsOneWidget);
      expect(find.byKey(const Key('shell-content')), findsOneWidget);
    });

    testWidgets('Happy path: nav and panel widths are the theme tokens inside the spec bands (CAT-02)', (tester) async {
      await pumpShell(tester);

      final navWidth = tester.getSize(find.byType(LeftNav)).width;
      final panelWidth = tester.getSize(find.byType(RightPanel)).width;
      expect(navWidth, AppSizes.leftNavWidth);
      expect(navWidth, inInclusiveRange(200, 240));
      expect(panelWidth, AppSizes.rightPanelWidth);
      expect(panelWidth, inInclusiveRange(280, 320));
    });

    testWidgets('Happy path: content area stays width-constrained to the theme measure', (tester) async {
      await pumpShell(tester, viewport: const Size(1600, 800));

      expect(
        tester.getSize(find.byKey(const Key('shell-content'))).width,
        AppSizes.contentMaxWidth - 2 * AppSpacing.lg,
      );
    });

    testWidgets('Happy path: rendering the shell loads the catalog exactly once', (tester) async {
      final recorded = await pumpShell(tester);

      expect(recorded, hasLength(1));
      expect(recorded.single.method, 'GET');
      expect(recorded.single.url.path, '/templates');
    });
  });

  group('navigation (CAT-04)', () {
    testWidgets('Happy path: exactly one nav entry renders active for the catalog surface (CAT-04)', (tester) async {
      await pumpShell(tester);

      final entryFinder = find.byType(NavEntry);
      expect(entryFinder, findsOneWidget);
      final entry = tester.widget<NavEntry>(entryFinder);
      expect(entry.active, isTrue);
      expect(entry.label, 'Catalog');

      final scheme = Theme.of(tester.element(entryFinder)).colorScheme;
      final material = tester.widget<Material>(
        find.descendant(of: entryFinder, matching: find.byType(Material)),
      );
      expect(material.color, scheme.secondaryContainer);
      final label = tester.widget<Text>(
        find.descendant(of: entryFinder, matching: find.text('Catalog')),
      );
      expect(label.style?.color, scheme.onSecondaryContainer);
    });

    testWidgets('Edge: the catalog entry stays active while the creation dialog surface is active (CAT-04 derivation)', (tester) async {
      await pumpShell(
        tester,
        extraOverrides: [
          shellStateProvider.overrideWithValue(ShellSurface.creationDialog),
        ],
      );

      final entry = tester.widget<NavEntry>(find.byType(NavEntry));
      expect(entry.active, isTrue);
    });
  });

  group('header (CAT-05)', () {
    testWidgets('Happy path: header shows the l10n title and reserves the connection slot (CAT-05)', (tester) async {
      await pumpShell(tester);

      final header = find.byType(Header);
      expect(find.descendant(of: header, matching: find.text('Legnaris')), findsOneWidget);
      final slot = find.descendant(
        of: header,
        matching: find.byKey(const Key('connection-indicator-slot')),
      );
      expect(slot, findsOneWidget);
      expect(tester.getSize(slot).width, AppSizes.connectionSlotWidth);
    });
  });

  group('right panel (CAT-06)', () {
    testWidgets('Happy path: panel renders the Connection block, hairline and Catalog stats (CAT-06)', (tester) async {
      await pumpShell(tester);

      final panel = find.byType(RightPanel);
      expect(find.descendant(of: panel, matching: find.text('Connection')), findsOneWidget);
      expect(
        find.descendant(of: panel, matching: find.byKey(const Key('connection-panel-slot'))),
        findsOneWidget,
      );
      expect(find.descendant(of: panel, matching: find.byType(Divider)), findsOneWidget);
      expect(find.descendant(of: panel, matching: find.text('Catalog')), findsOneWidget);
      expect(find.descendant(of: panel, matching: find.text('2 templates')), findsOneWidget);
      expect(
        find.descendant(
          of: panel,
          matching: find.textContaining(RegExp(r'Last updated \d{1,2}:\d{2}')),
        ),
        findsOneWidget,
      );
    });

    testWidgets('Edge: the last update time appears only after the first load lands (CAT-06)', (tester) async {
      final gate = Completer<http.Response>();
      await pumpShell(tester, handler: (request) => gate.future);

      final panel = find.byType(RightPanel);
      expect(find.descendant(of: panel, matching: find.text('0 templates')), findsOneWidget);
      expect(find.descendant(of: panel, matching: find.textContaining('Last updated')), findsNothing);

      gate.complete(http.Response(twoTemplatesBody, 200));
      await tester.pumpAndSettle();

      expect(find.descendant(of: panel, matching: find.text('2 templates')), findsOneWidget);
      expect(
        find.descendant(of: panel, matching: find.textContaining(RegExp(r'Last updated \d'))),
        findsOneWidget,
      );
    });
  });
}
