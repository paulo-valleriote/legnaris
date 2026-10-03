import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/catalog/catalog_provider.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/sheet/sheet_open_provider.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/shell/shell_state.dart';
import 'package:legnaris/theme/tokens.dart';

/// Screen 1 — the template catalog (CAT-08 … CAT-12, CAT-19).
///
/// Pure render of [CatalogState]: the first load is scheduled by the
/// provider, and both Retry entry points only call `refresh()`, whose
/// single-flight guard absorbs double activation (CAT-12/13).
class CatalogScreen extends ConsumerWidget {
  const CatalogScreen({super.key});

  /// Compact horizontal padding so both card actions fit inside the card
  /// width on one line (RND-14); height and radius stay from the theme.
  static final ButtonStyle _cardActionStyle = TextButton.styleFrom(
    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm),
  );

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(catalogProvider);
    final localizations = AppLocalizations.of(context)!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (state.isLoading) const LinearProgressIndicator(),
        Expanded(child: _body(context, ref, state, localizations)),
      ],
    );
  }

  /// Picks the render state: never-loaded without a failure reads as loading
  /// (defensive — a failed first load is the error branch, CAT-11), a loaded
  /// catalog keeps its entries even when a refresh failed (CAT-19).
  Widget _body(
    BuildContext context,
    WidgetRef ref,
    CatalogState state,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    final mutedStyle = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );
    if (!state.hasLoaded) {
      if (state.errorMessage != null && !state.isLoading) {
        return _fullError(context, ref, state.errorMessage!, localizations);
      }
      return Center(
        child: Text(localizations.catalogLoading, style: mutedStyle),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (state.errorMessage != null)
          _inlineError(context, ref, state.errorMessage!, localizations),
        Expanded(
          child: state.isEmpty
              ? Center(
                  child: Text(localizations.catalogEmpty, style: mutedStyle),
                )
              : _cardList(context, ref, state, localizations),
        ),
      ],
    );
  }

  /// Error without a cache: the mapped message plus one Retry action (CAT-11).
  Widget _fullError(
    BuildContext context,
    WidgetRef ref,
    String message,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            message,
            textAlign: TextAlign.center,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.error,
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          OutlinedButton(
            onPressed: () => ref.read(catalogProvider.notifier).refresh(),
            child: Text(localizations.actionRetry),
          ),
        ],
      ),
    );
  }

  /// Error over a cache: the mapped message stays inline above the list so
  /// the entries remain visible (CAT-19).
  Widget _inlineError(
    BuildContext context,
    WidgetRef ref,
    String message,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.md),
      child: Row(
        children: [
          Expanded(
            child: Text(
              message,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.error,
              ),
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          OutlinedButton(
            onPressed: () => ref.read(catalogProvider.notifier).refresh(),
            child: Text(localizations.actionRetry),
          ),
        ],
      ),
    );
  }

  /// The card grid: one bounded card per template in API order (CAT-09).
  Widget _cardList(
    BuildContext context,
    WidgetRef ref,
    CatalogState state,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    return SingleChildScrollView(
      child: Wrap(
        spacing: AppSpacing.lg,
        runSpacing: AppSpacing.lg,
        children: [
          for (final template in state.templates!)
            SizedBox(
              width: AppSizes.catalogCardMinWidth,
              child: Card(
                child: InkWell(
                  onTap: () => _openCreationDialog(context, ref, template.id),
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.md),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(template.name, style: theme.textTheme.titleMedium),
                        const SizedBox(height: AppSpacing.xs),
                        Text(
                          template.id,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        TextButton(
                          key: Key('catalog-create-${template.id}'),
                          style: _cardActionStyle,
                          onPressed: () =>
                              _openCreationDialog(context, ref, template.id),
                          child: Text(localizations.catalogCreateAction),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  /// Opens the creation dialog for [templateId] (CAT-15) and marks the
  /// creation surface while it is up, restoring the catalog in `finally` so
  /// every dismiss path — Esc, Close, barrier — lands back on the catalog
  /// (CAT-16). A successful create pops the dialog with the created sheet,
  /// which is then opened on Screen 2 (CRT-04).
  Future<void> _openCreationDialog(
    BuildContext context,
    WidgetRef ref,
    String templateId,
  ) async {
    final shellState = ref.read(shellStateProvider.notifier);
    shellState.show(ShellSurface.creationDialog);
    Sheet? created;
    try {
      created = await showDialog<Sheet>(
        context: context,
        // Load-bearing: the framework gates its dismiss action (Esc) on
        // this flag (routes.dart:981) — CAT-16 breaks without it.
        barrierDismissible: true,
        builder: (dialogContext) => CreateSheetDialog(templateId: templateId),
      );
    } finally {
      shellState.show(ShellSurface.catalog);
    }
    if (created == null || !context.mounted) return;
    await _openSheetScreen(context, sheetId: created.id, sheet: created);
  }

  /// Opens Screen 2 for the stored sheet [sheetId] (OPN-04). The route mounts
  /// the persistent shell around the sheet surface; Screen 2's back action pops
  /// it, returning to the cached catalog with no extra request. A [sheet]
  /// injects the created `201` payload so the screen skips the `GET`
  /// (OPN-09); the library path passes none and Screen 2 fetches by id.
  Future<void> _openSheetScreen(
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
}
