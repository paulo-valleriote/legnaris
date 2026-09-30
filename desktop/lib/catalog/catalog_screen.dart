import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/catalog/catalog_provider.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/shell/shell_state.dart';
import 'package:legnaris/theme/tokens.dart';

/// Screen 1 — the template catalog (CAT-08 … CAT-12, CAT-19).
///
/// Pure render of [CatalogState]: the first load is scheduled by the
/// provider, and both Retry entry points only call `refresh()`, whose
/// single-flight guard absorbs double activation (CAT-12/13).
class CatalogScreen extends ConsumerWidget {
  const CatalogScreen({super.key});

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
                        SizedBox(
                          height: AppSizes.controlHeight,
                          child: TextButton(
                            onPressed: () =>
                                _openCreationDialog(context, ref, template.id),
                            child: Text(localizations.catalogCreateAction),
                          ),
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
  /// (CAT-16).
  Future<void> _openCreationDialog(
    BuildContext context,
    WidgetRef ref,
    String templateId,
  ) async {
    final shellState = ref.read(shellStateProvider.notifier);
    shellState.show(ShellSurface.creationDialog);
    try {
      await showDialog<void>(
        context: context,
        // Load-bearing: the framework gates its dismiss action (Esc) on
        // this flag (routes.dart:981) — CAT-16 breaks without it.
        barrierDismissible: true,
        builder: (dialogContext) => CreateSheetDialog(templateId: templateId),
      );
    } finally {
      shellState.show(ShellSurface.catalog);
    }
  }
}
