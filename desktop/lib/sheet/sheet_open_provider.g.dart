// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sheet_open_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Load state of one stored sheet, fetched on demand for Screen 2 (OPN-05).

@ProviderFor(SheetOpenController)
final sheetOpenControllerProvider = SheetOpenControllerFamily._();

/// Load state of one stored sheet, fetched on demand for Screen 2 (OPN-05).
final class SheetOpenControllerProvider
    extends $NotifierProvider<SheetOpenController, SheetOpenState> {
  /// Load state of one stored sheet, fetched on demand for Screen 2 (OPN-05).
  SheetOpenControllerProvider._({
    required SheetOpenControllerFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'sheetOpenControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$sheetOpenControllerHash();

  @override
  String toString() {
    return r'sheetOpenControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  SheetOpenController create() => SheetOpenController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SheetOpenState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SheetOpenState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is SheetOpenControllerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$sheetOpenControllerHash() =>
    r'd6bace857faedfe6ea3cd2d630c631299b0fa027';

/// Load state of one stored sheet, fetched on demand for Screen 2 (OPN-05).

final class SheetOpenControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          SheetOpenController,
          SheetOpenState,
          SheetOpenState,
          SheetOpenState,
          String
        > {
  SheetOpenControllerFamily._()
    : super(
        retry: null,
        name: r'sheetOpenControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// Load state of one stored sheet, fetched on demand for Screen 2 (OPN-05).

  SheetOpenControllerProvider call(String sheetId) =>
      SheetOpenControllerProvider._(argument: sheetId, from: this);

  @override
  String toString() => r'sheetOpenControllerProvider';
}

/// Load state of one stored sheet, fetched on demand for Screen 2 (OPN-05).

abstract class _$SheetOpenController extends $Notifier<SheetOpenState> {
  late final _$args = ref.$arg as String;
  String get sheetId => _$args;

  SheetOpenState build(String sheetId);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<SheetOpenState, SheetOpenState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<SheetOpenState, SheetOpenState>,
              SheetOpenState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args));
  }
}
