// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sheet_refresh.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Template id of the sheet screen currently mounted, or null when none is.
/// The mounted screen holds the listener that keeps this alive; once it goes
/// away the provider disposes and a fresh read is null again.

@ProviderFor(ActiveSheetId)
final activeSheetIdProvider = ActiveSheetIdProvider._();

/// Template id of the sheet screen currently mounted, or null when none is.
/// The mounted screen holds the listener that keeps this alive; once it goes
/// away the provider disposes and a fresh read is null again.
final class ActiveSheetIdProvider
    extends $NotifierProvider<ActiveSheetId, String?> {
  /// Template id of the sheet screen currently mounted, or null when none is.
  /// The mounted screen holds the listener that keeps this alive; once it goes
  /// away the provider disposes and a fresh read is null again.
  ActiveSheetIdProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'activeSheetIdProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$activeSheetIdHash();

  @$internal
  @override
  ActiveSheetId create() => ActiveSheetId();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$activeSheetIdHash() => r'c058d637a2091be080fe7e9beddd47c07db881ff';

/// Template id of the sheet screen currently mounted, or null when none is.
/// The mounted screen holds the listener that keeps this alive; once it goes
/// away the provider disposes and a fresh read is null again.

abstract class _$ActiveSheetId extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<String?, String?>,
              String?,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}

/// Monotonic refresh counter: the shell bumps it once per F5 dispatch aimed
/// at the active sheet, and the mounted screen reacts by refreshing.
/// Starts at 0; disposal resets it, so a later sheet never sees a stale tick.

@ProviderFor(SheetRefreshTick)
final sheetRefreshTickProvider = SheetRefreshTickProvider._();

/// Monotonic refresh counter: the shell bumps it once per F5 dispatch aimed
/// at the active sheet, and the mounted screen reacts by refreshing.
/// Starts at 0; disposal resets it, so a later sheet never sees a stale tick.
final class SheetRefreshTickProvider
    extends $NotifierProvider<SheetRefreshTick, int> {
  /// Monotonic refresh counter: the shell bumps it once per F5 dispatch aimed
  /// at the active sheet, and the mounted screen reacts by refreshing.
  /// Starts at 0; disposal resets it, so a later sheet never sees a stale tick.
  SheetRefreshTickProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sheetRefreshTickProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sheetRefreshTickHash();

  @$internal
  @override
  SheetRefreshTick create() => SheetRefreshTick();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$sheetRefreshTickHash() => r'cac397bede4746432c4bd5591e1e26d156dcc137';

/// Monotonic refresh counter: the shell bumps it once per F5 dispatch aimed
/// at the active sheet, and the mounted screen reacts by refreshing.
/// Starts at 0; disposal resets it, so a later sheet never sees a stale tick.

abstract class _$SheetRefreshTick extends $Notifier<int> {
  int build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<int, int>,
              int,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}
