import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/shell/shell_state.dart';
import 'package:legnaris/theme/tokens.dart';

/// Persistent left navigation (CAT-04): exactly one entry in Phase 2, sized
/// by the nav-width token so it can never collapse into an overlay.
///
/// The entry is top-aligned with loosened height constraints: the shell row
/// stretches its children, and a tight height would force past the entry's
/// own [AppSizes.rowHeight] box — inflating the active tint to a full-height
/// block (UAT T8).
class LeftNav extends ConsumerWidget {
  const LeftNav({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final surface = ref.watch(shellStateProvider);
    final localizations = AppLocalizations.of(context)!;
    return SizedBox(
      width: AppSizes.leftNavWidth,
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Align(
          alignment: Alignment.topLeft,
          child: NavEntry(
            label: localizations.navCatalog,
            active: sectionIsActive(surface),
          ),
        ),
      ),
    );
  }
}

/// One navigation entry: fixed row height, control radius, hover feedback
/// from the theme (no layout shift), and the active entry tinted with the
/// scheme's `secondaryContainer` pair.
class NavEntry extends StatelessWidget {
  const NavEntry({
    super.key,
    required this.label,
    required this.active,
    this.onTap,
  });

  /// Localized entry label.
  final String label;

  /// Whether this entry is the surface on screen.
  final bool active;

  /// Activates the entry; null while there is nothing to navigate to.
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return Material(
      color: active ? scheme.secondaryContainer : null,
      borderRadius: BorderRadius.circular(AppRadius.control),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: SizedBox(
          height: AppSizes.rowHeight,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Text(
                label,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: active
                      ? scheme.onSecondaryContainer
                      : scheme.onSurface,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
