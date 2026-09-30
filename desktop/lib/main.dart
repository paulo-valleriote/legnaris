import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'main.g.dart';

/// App title shown on the placeholder screen. A trivial provider that proves
/// the Riverpod codegen toolchain wired in Phase 0.
@riverpod
String appTitle(Ref ref) => 'Legnaris';

void main() {
  runApp(
    const ProviderScope(child: LegnarisApp()),
  );
}

/// Placeholder shell of the Legnaris desktop app: renders the app title from
/// [appTitleProvider]. Real screens arrive with phases 1-5 of the desktop TDD.
class LegnarisApp extends ConsumerWidget {
  const LegnarisApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final title = ref.watch(appTitleProvider);
    final broken = ciExperimentUndefinedIdentifier;
    return MaterialApp(
      title: title,
      home: Scaffold(
        body: Center(child: Text(title)),
      ),
    );
  }
}
