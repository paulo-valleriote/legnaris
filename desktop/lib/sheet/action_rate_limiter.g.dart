// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'action_rate_limiter.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The injectable clock behind the rolling windows. Production uses
/// [DateTime.now]; tests override it to advance time deterministically.

@ProviderFor(clock)
final clockProvider = ClockProvider._();

/// The injectable clock behind the rolling windows. Production uses
/// [DateTime.now]; tests override it to advance time deterministically.

final class ClockProvider
    extends
        $FunctionalProvider<
          DateTime Function(),
          DateTime Function(),
          DateTime Function()
        >
    with $Provider<DateTime Function()> {
  /// The injectable clock behind the rolling windows. Production uses
  /// [DateTime.now]; tests override it to advance time deterministically.
  ClockProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'clockProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$clockHash();

  @$internal
  @override
  $ProviderElement<DateTime Function()> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  DateTime Function() create(Ref ref) {
    return clock(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DateTime Function() value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DateTime Function()>(value),
    );
  }
}

String _$clockHash() => r'3f65ad34ac6fcd532de9004042bdf2ed2bd85b13';

/// App-wide cooldown clock. The provider owns the timer: [Ref.onDispose]
/// cancels it, so tearing down the container never leaves a pending timer.

@ProviderFor(cooldownScheduler)
final cooldownSchedulerProvider = CooldownSchedulerProvider._();

/// App-wide cooldown clock. The provider owns the timer: [Ref.onDispose]
/// cancels it, so tearing down the container never leaves a pending timer.

final class CooldownSchedulerProvider
    extends
        $FunctionalProvider<
          CooldownScheduler,
          CooldownScheduler,
          CooldownScheduler
        >
    with $Provider<CooldownScheduler> {
  /// App-wide cooldown clock. The provider owns the timer: [Ref.onDispose]
  /// cancels it, so tearing down the container never leaves a pending timer.
  CooldownSchedulerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'cooldownSchedulerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$cooldownSchedulerHash();

  @$internal
  @override
  $ProviderElement<CooldownScheduler> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  CooldownScheduler create(Ref ref) {
    return cooldownScheduler(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CooldownScheduler value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CooldownScheduler>(value),
    );
  }
}

String _$cooldownSchedulerHash() => r'5d6930a40108bc4f46a7ec36010860ffe3d260b5';
