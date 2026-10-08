import 'package:flutter/material.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/shell/app_shell.dart';

/// The authenticated surface the gate renders (FE6-13 … FE6-16).
///
/// At this task it mounts the pre-existing catalog inside the shell; the next
/// task resolves the user's single sheet through `GET /sheets` and swaps the
/// content for Screen 2 or the create flow.
class AuthenticatedHome extends StatelessWidget {
  const AuthenticatedHome({super.key});

  @override
  Widget build(BuildContext context) =>
      const AppShell(content: CatalogScreen());
}
