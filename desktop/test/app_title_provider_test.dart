import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/main.dart';

void main() {
  test('appTitleProvider returns the app title', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    expect(container.read(appTitleProvider), 'Legnaris');
  });
}
