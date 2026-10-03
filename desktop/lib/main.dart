import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/shell/shortcut_registry.dart';
import 'package:legnaris/theme/app_theme.dart';
import 'package:path_provider/path_provider.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'main.g.dart';

/// App title shown in the OS window title bar (`MaterialApp.title`). A
/// trivial provider that proves the Riverpod codegen toolchain wired in
/// Phase 0; the in-app header title comes from l10n instead.
@riverpod
String appTitle(Ref ref) => 'Legnaris';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final directory = await getApplicationSupportDirectory();
  runApp(
    ProviderScope(
      overrides: [appSupportDirectoryProvider.overrideWithValue(directory)],
      child: const LegnarisApp(),
    ),
  );
}

/// The Legnaris desktop app: the shell mounted once with the catalog as its
/// content, themed from `lib/theme/`, localized from the ARB files, and the
/// central shortcut/action registry wired into [MaterialApp].
class LegnarisApp extends ConsumerWidget {
  const LegnarisApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final title = ref.watch(appTitleProvider);
    return MaterialApp(
      title: title,
      theme: buildAppTheme(),
      shortcuts: appShortcuts(),
      actions: appActions(ref),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const AppShell(content: CatalogScreen()),
    );
  }
}
