// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'health_scheduler.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// App-wide polling clock. The provider owns the timer: [Ref.onDispose]
/// cancels it, so tearing down the container never leaves a pending timer.

@ProviderFor(healthScheduler)
final healthSchedulerProvider = HealthSchedulerProvider._();

/// App-wide polling clock. The provider owns the timer: [Ref.onDispose]
/// cancels it, so tearing down the container never leaves a pending timer.

final class HealthSchedulerProvider
    extends
        $FunctionalProvider<HealthScheduler, HealthScheduler, HealthScheduler>
    with $Provider<HealthScheduler> {
  /// App-wide polling clock. The provider owns the timer: [Ref.onDispose]
  /// cancels it, so tearing down the container never leaves a pending timer.
  HealthSchedulerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'healthSchedulerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$healthSchedulerHash();

  @$internal
  @override
  $ProviderElement<HealthScheduler> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  HealthScheduler create(Ref ref) {
    return healthScheduler(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(HealthScheduler value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<HealthScheduler>(value),
    );
  }
}

String _$healthSchedulerHash() => r'd6a08ece09390cb02186b3a0a814ba626e2976b4';
