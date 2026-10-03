// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sheet_edit_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The stored sheet an open Screen 2 was launched for, injected per route.
///
/// The creation flow overrides this with the `201` payload so Screen 2 edits
/// the server-stored values without a second request (CRT-08, OPN-09); the
/// library path leaves it null and `SheetOpenController` fetches the sheet by
/// id. Overriding at the route scope keeps the screen, this controller and
/// every field consumer on one container (design: opened-sheet seam).

@ProviderFor(openedSheet)
final openedSheetProvider = OpenedSheetProvider._();

/// The stored sheet an open Screen 2 was launched for, injected per route.
///
/// The creation flow overrides this with the `201` payload so Screen 2 edits
/// the server-stored values without a second request (CRT-08, OPN-09); the
/// library path leaves it null and `SheetOpenController` fetches the sheet by
/// id. Overriding at the route scope keeps the screen, this controller and
/// every field consumer on one container (design: opened-sheet seam).

final class OpenedSheetProvider
    extends $FunctionalProvider<Sheet?, Sheet?, Sheet?>
    with $Provider<Sheet?> {
  /// The stored sheet an open Screen 2 was launched for, injected per route.
  ///
  /// The creation flow overrides this with the `201` payload so Screen 2 edits
  /// the server-stored values without a second request (CRT-08, OPN-09); the
  /// library path leaves it null and `SheetOpenController` fetches the sheet by
  /// id. Overriding at the route scope keeps the screen, this controller and
  /// every field consumer on one container (design: opened-sheet seam).
  OpenedSheetProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'openedSheetProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$openedSheetHash();

  @$internal
  @override
  $ProviderElement<Sheet?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  Sheet? create(Ref ref) {
    return openedSheet(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Sheet? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Sheet?>(value),
    );
  }
}

String _$openedSheetHash() => r'7fe9b047ab9f66a5449b71c7fe6bcebabe7d9c67';

/// Edit state of the sheet opened for one sheet id.
///
/// Rebuilds (and reseeds) whenever the open sheet or the template detail
/// changes: the loading state drops the value, which discards the edits, and a
/// successful load seeds from the sheet's stored data over the schema defaults
/// — the discard-confirmed refresh flow rides on that (GRN-17).

@ProviderFor(SheetEditController)
final sheetEditControllerProvider = SheetEditControllerFamily._();

/// Edit state of the sheet opened for one sheet id.
///
/// Rebuilds (and reseeds) whenever the open sheet or the template detail
/// changes: the loading state drops the value, which discards the edits, and a
/// successful load seeds from the sheet's stored data over the schema defaults
/// — the discard-confirmed refresh flow rides on that (GRN-17).
final class SheetEditControllerProvider
    extends $NotifierProvider<SheetEditController, SheetEditState> {
  /// Edit state of the sheet opened for one sheet id.
  ///
  /// Rebuilds (and reseeds) whenever the open sheet or the template detail
  /// changes: the loading state drops the value, which discards the edits, and a
  /// successful load seeds from the sheet's stored data over the schema defaults
  /// — the discard-confirmed refresh flow rides on that (GRN-17).
  SheetEditControllerProvider._({
    required SheetEditControllerFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'sheetEditControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$sheetEditControllerHash();

  @override
  String toString() {
    return r'sheetEditControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  SheetEditController create() => SheetEditController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SheetEditState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SheetEditState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is SheetEditControllerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$sheetEditControllerHash() =>
    r'627953142465cf284cc26f7c6f15357a6435b795';

/// Edit state of the sheet opened for one sheet id.
///
/// Rebuilds (and reseeds) whenever the open sheet or the template detail
/// changes: the loading state drops the value, which discards the edits, and a
/// successful load seeds from the sheet's stored data over the schema defaults
/// — the discard-confirmed refresh flow rides on that (GRN-17).

final class SheetEditControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          SheetEditController,
          SheetEditState,
          SheetEditState,
          SheetEditState,
          String
        > {
  SheetEditControllerFamily._()
    : super(
        retry: null,
        name: r'sheetEditControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// Edit state of the sheet opened for one sheet id.
  ///
  /// Rebuilds (and reseeds) whenever the open sheet or the template detail
  /// changes: the loading state drops the value, which discards the edits, and a
  /// successful load seeds from the sheet's stored data over the schema defaults
  /// — the discard-confirmed refresh flow rides on that (GRN-17).

  SheetEditControllerProvider call(String sheetId) =>
      SheetEditControllerProvider._(argument: sheetId, from: this);

  @override
  String toString() => r'sheetEditControllerProvider';
}

/// Edit state of the sheet opened for one sheet id.
///
/// Rebuilds (and reseeds) whenever the open sheet or the template detail
/// changes: the loading state drops the value, which discards the edits, and a
/// successful load seeds from the sheet's stored data over the schema defaults
/// — the discard-confirmed refresh flow rides on that (GRN-17).

abstract class _$SheetEditController extends $Notifier<SheetEditState> {
  late final _$args = ref.$arg as String;
  String get sheetId => _$args;

  SheetEditState build(String sheetId);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<SheetEditState, SheetEditState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<SheetEditState, SheetEditState>,
              SheetEditState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args));
  }
}
