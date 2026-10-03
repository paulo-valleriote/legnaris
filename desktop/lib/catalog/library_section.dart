/// "My sheets" section on Screen 1: the local library index (LGN-107) with an
/// Open and a local-only Remove per entry (OPN-01 … OPN-03, OPN-12 … OPN-15).
///
/// Pure over [sheetLibraryProvider]; opening is delegated to the caller through
/// [onOpen] so the section owns no navigation. Removing shows a destructive
/// confirmation and never issues an API request.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/theme/tokens.dart';

/// The Screen 1 section listing every stored library entry.
class LibrarySection extends ConsumerStatefulWidget {
  const LibrarySection({super.key, required this.onOpen});

  /// Opens the stored sheet for one entry.
  final ValueChanged<SheetLibraryEntry> onOpen;

  @override
  ConsumerState<LibrarySection> createState() => _LibrarySectionState();
}

class _LibrarySectionState extends ConsumerState<LibrarySection> {
  /// Inline copy of the last failed removal; cleared on the next success.
  String? _errorMessage;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final localizations = AppLocalizations.of(context)!;
    final entries = ref.watch(sheetLibraryProvider).entries;
    final muted = theme.textTheme.bodyMedium?.copyWith(
      color: theme.colorScheme.onSurfaceVariant,
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          localizations.librarySectionTitle,
          key: const Key('library-section-title'),
          style: theme.textTheme.titleMedium,
        ),
        const SizedBox(height: AppSpacing.sm),
        if (entries.isEmpty)
          Text(localizations.libraryEmpty, style: muted)
        else
          for (final entry in entries) _row(context, entry, localizations),
        if (_errorMessage != null) ...[
          const SizedBox(height: AppSpacing.sm),
          Text(
            _errorMessage!,
            key: const Key('library-remove-error'),
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.error,
            ),
          ),
        ],
      ],
    );
  }

  Widget _row(
    BuildContext context,
    SheetLibraryEntry entry,
    AppLocalizations localizations,
  ) {
    final theme = Theme.of(context);
    return SizedBox(
      height: AppSizes.rowHeight,
      child: Row(
        children: [
          Expanded(
            child: Text(
              entry.name,
              key: Key('library-name-${entry.id}'),
              overflow: TextOverflow.ellipsis,
            ),
          ),
          const SizedBox(width: AppSpacing.sm),
          Text(
            entry.templateId,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          TextButton(
            key: Key('library-open-${entry.id}'),
            onPressed: () => widget.onOpen(entry),
            child: Text(localizations.libraryOpenAction),
          ),
          TextButton(
            key: Key('library-remove-${entry.id}'),
            style: TextButton.styleFrom(
              foregroundColor: theme.colorScheme.error,
            ),
            onPressed: () => _confirmRemove(entry, localizations),
            child: Text(localizations.libraryRemoveAction),
          ),
        ],
      ),
    );
  }

  /// Destructive confirmation for the local-only removal: only an explicit
  /// confirm proceeds, and a confirm issues no API request (OPN-12 … OPN-14).
  Future<void> _confirmRemove(
    SheetLibraryEntry entry,
    AppLocalizations localizations,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      // Load-bearing: barrier and Esc land on the safe side — dismiss equals
      // cancel (desktop/AGENTS.md: never confirm by Enter alone).
      barrierDismissible: true,
      builder: (dialogContext) {
        final theme = Theme.of(dialogContext);
        return AlertDialog(
          title: Text(localizations.libraryRemoveDialogTitle),
          content: Text(localizations.libraryRemoveDialogMessage),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: Text(localizations.libraryRemoveCancelAction),
            ),
            TextButton(
              style: TextButton.styleFrom(
                foregroundColor: theme.colorScheme.error,
              ),
              onPressed: () => Navigator.of(dialogContext).pop(true),
              child: Text(localizations.libraryRemoveConfirmAction),
            ),
          ],
        );
      },
    );
    if (!mounted || confirmed != true) return;
    try {
      ref.read(sheetLibraryProvider.notifier).remove(entry.id);
      setState(() => _errorMessage = null);
    } on SheetLibraryException {
      setState(() => _errorMessage = localizations.libraryRemoveFailed);
    }
  }
}
