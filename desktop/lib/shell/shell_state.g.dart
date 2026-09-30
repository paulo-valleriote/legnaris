// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shell_state.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The shell's active surface. keepAlive because the app has exactly one
/// shell for its whole lifetime.

@ProviderFor(ShellState)
final shellStateProvider = ShellStateProvider._();

/// The shell's active surface. keepAlive because the app has exactly one
/// shell for its whole lifetime.
final class ShellStateProvider
    extends $NotifierProvider<ShellState, ShellSurface> {
  /// The shell's active surface. keepAlive because the app has exactly one
  /// shell for its whole lifetime.
  ShellStateProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shellStateProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$shellStateHash();

  @$internal
  @override
  ShellState create() => ShellState();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ShellSurface value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ShellSurface>(value),
    );
  }
}

String _$shellStateHash() => r'15288cd4689c531d05e650efaa8cc7b0da282bd6';

/// The shell's active surface. keepAlive because the app has exactly one
/// shell for its whole lifetime.

abstract class _$ShellState extends $Notifier<ShellSurface> {
  ShellSurface build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<ShellSurface, ShellSurface>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ShellSurface, ShellSurface>,
              ShellSurface,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}
