import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/main.dart';

void main() {
  testWidgets('placeholder renders the app title inside ProviderScope', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(const ProviderScope(child: LegnarisApp()));

    expect(find.text('Legnaris'), findsOneWidget);
  });
}
