import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'shell_state.g.dart';

/// Which surface of the shell is on screen: the catalog screen itself, or
/// the creation dialog opened over it. Guards the refresh action (F5 only
/// while the catalog is visible) and drives the nav active state.
enum ShellSurface { catalog, creationDialog }

/// Whether the Catalog nav entry renders active for [surface]: the creation
/// dialog is a modal over the catalog, so the catalog still counts as on
/// screen (design: Phase 4's "My sheets" only changes this derivation).
bool sectionIsActive(ShellSurface surface) =>
    surface == ShellSurface.catalog || surface == ShellSurface.creationDialog;

/// The shell's active surface. keepAlive because the app has exactly one
/// shell for its whole lifetime.
@Riverpod(keepAlive: true)
class ShellState extends _$ShellState {
  @override
  ShellSurface build() => ShellSurface.catalog;

  /// Marks [surface] as the one on screen.
  void show(ShellSurface surface) {
    state = surface;
  }
}
