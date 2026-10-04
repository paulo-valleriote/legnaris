// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_backup_scheduler.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// App-wide draft backup clock. The provider owns the timer:
/// [Ref.onDispose] cancels it, so tearing down the container never leaves a
/// pending timer.

@ProviderFor(draftBackupScheduler)
final draftBackupSchedulerProvider = DraftBackupSchedulerProvider._();

/// App-wide draft backup clock. The provider owns the timer:
/// [Ref.onDispose] cancels it, so tearing down the container never leaves a
/// pending timer.

final class DraftBackupSchedulerProvider
    extends
        $FunctionalProvider<
          DraftBackupScheduler,
          DraftBackupScheduler,
          DraftBackupScheduler
        >
    with $Provider<DraftBackupScheduler> {
  /// App-wide draft backup clock. The provider owns the timer:
  /// [Ref.onDispose] cancels it, so tearing down the container never leaves a
  /// pending timer.
  DraftBackupSchedulerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'draftBackupSchedulerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$draftBackupSchedulerHash();

  @$internal
  @override
  $ProviderElement<DraftBackupScheduler> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  DraftBackupScheduler create(Ref ref) {
    return draftBackupScheduler(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DraftBackupScheduler value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DraftBackupScheduler>(value),
    );
  }
}

String _$draftBackupSchedulerHash() =>
    r'77edd3fb0bd693fb043aee7367bafe7781109fef';
