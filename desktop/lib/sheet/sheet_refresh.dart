/// Dispatch seam between the shell's refresh action (F5) and the mounted
/// sheet screen (LGN-102, GRN-17 … GRN-19).
///
/// The shell must not know sheet internals: it checks [activeSheetIdProvider]
/// and bumps [sheetRefreshTickProvider]; the screen keeps both alive by
/// listening while mounted, runs its own confirmation/refetch flow, and
/// unmounting discards the registration — so a stale id or tick can never
/// outlive its screen.
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'sheet_refresh.g.dart';

/// Sheet id of the sheet screen currently mounted, or null when none is.
/// The mounted screen holds the listener that keeps this alive; once it goes
/// away the provider disposes and a fresh read is null again.
@riverpod
class ActiveSheetId extends _$ActiveSheetId {
  @override
  String? build() => null;

  /// Registers [sheetId] as the active sheet, replacing any previous
  /// registration (only one sheet screen can be on screen at a time).
  void set(String? sheetId) {
    state = sheetId;
  }
}

/// Monotonic refresh counter: the shell bumps it once per F5 dispatch aimed
/// at the active sheet, and the mounted screen reacts by refreshing.
/// Starts at 0; disposal resets it, so a later sheet never sees a stale tick.
@riverpod
class SheetRefreshTick extends _$SheetRefreshTick {
  @override
  int build() => 0;

  /// Records one refresh request.
  void bump() {
    state = state + 1;
  }
}
