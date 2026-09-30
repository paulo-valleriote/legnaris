import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/main.dart';

void main() {
  testWidgets('placeholder renders the default app title', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(const ProviderScope(child: LegnarisApp()));

    expect(find.text('Legnaris'), findsOneWidget);
  });

  testWidgets('placeholder renders the provider value, not a hardcoded string', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appTitleProvider.overrideWithValue('Legnaris (overridden)'),
        ],
        child: const LegnarisApp(),
      ),
    );

    expect(find.text('Legnaris (overridden)'), findsOneWidget);
    expect(find.text('Legnaris'), findsNothing);
  });
}
