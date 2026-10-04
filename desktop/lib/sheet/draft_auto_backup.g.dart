// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_auto_backup.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Runs the periodic draft backup for [sheetId] while its edit state has
/// pending edits, and returns whether the backup clock is running.
///
/// One scheduler runs per dirty transition: a rebuild that leaves `isPending`
/// true does not restart the timer. The write is best-effort so a full disk
/// never interrupts editing (DRB-13).

@ProviderFor(draftAutoBackup)
final draftAutoBackupProvider = DraftAutoBackupFamily._();

/// Runs the periodic draft backup for [sheetId] while its edit state has
/// pending edits, and returns whether the backup clock is running.
///
/// One scheduler runs per dirty transition: a rebuild that leaves `isPending`
/// true does not restart the timer. The write is best-effort so a full disk
/// never interrupts editing (DRB-13).

final class DraftAutoBackupProvider
    extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// Runs the periodic draft backup for [sheetId] while its edit state has
  /// pending edits, and returns whether the backup clock is running.
  ///
  /// One scheduler runs per dirty transition: a rebuild that leaves `isPending`
  /// true does not restart the timer. The write is best-effort so a full disk
  /// never interrupts editing (DRB-13).
  DraftAutoBackupProvider._({
    required DraftAutoBackupFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'draftAutoBackupProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$draftAutoBackupHash();

  @override
  String toString() {
    return r'draftAutoBackupProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    final argument = this.argument as String;
    return draftAutoBackup(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is DraftAutoBackupProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$draftAutoBackupHash() => r'd793c6a5192e59dad73a8c82edb34208b2eee525';

/// Runs the periodic draft backup for [sheetId] while its edit state has
/// pending edits, and returns whether the backup clock is running.
///
/// One scheduler runs per dirty transition: a rebuild that leaves `isPending`
/// true does not restart the timer. The write is best-effort so a full disk
/// never interrupts editing (DRB-13).

final class DraftAutoBackupFamily extends $Family
    with $FunctionalFamilyOverride<bool, String> {
  DraftAutoBackupFamily._()
    : super(
        retry: null,
        name: r'draftAutoBackupProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// Runs the periodic draft backup for [sheetId] while its edit state has
  /// pending edits, and returns whether the backup clock is running.
  ///
  /// One scheduler runs per dirty transition: a rebuild that leaves `isPending`
  /// true does not restart the timer. The write is best-effort so a full disk
  /// never interrupts editing (DRB-13).

  DraftAutoBackupProvider call(String sheetId) =>
      DraftAutoBackupProvider._(argument: sheetId, from: this);

  @override
  String toString() => r'draftAutoBackupProvider';
}
