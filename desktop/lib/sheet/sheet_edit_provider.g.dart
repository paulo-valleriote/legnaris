// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sheet_edit_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Edit state of the sheet for one template id.
///
/// Rebuilds (and reseeds) whenever the template detail state changes: the
/// loading state drops the template, which discards the edits, and a
/// successful reload seeds the fresh schema — the discard-confirmed refresh
/// flow rides on that (GRN-17).

@ProviderFor(SheetEditController)
final sheetEditControllerProvider = SheetEditControllerFamily._();

/// Edit state of the sheet for one template id.
///
/// Rebuilds (and reseeds) whenever the template detail state changes: the
/// loading state drops the template, which discards the edits, and a
/// successful reload seeds the fresh schema — the discard-confirmed refresh
/// flow rides on that (GRN-17).
final class SheetEditControllerProvider
    extends $NotifierProvider<SheetEditController, SheetEditState> {
  /// Edit state of the sheet for one template id.
  ///
  /// Rebuilds (and reseeds) whenever the template detail state changes: the
  /// loading state drops the template, which discards the edits, and a
  /// successful reload seeds the fresh schema — the discard-confirmed refresh
  /// flow rides on that (GRN-17).
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
    r'afbb5cfd08e9823888f95abe86664bed8088cbff';

/// Edit state of the sheet for one template id.
///
/// Rebuilds (and reseeds) whenever the template detail state changes: the
/// loading state drops the template, which discards the edits, and a
/// successful reload seeds the fresh schema — the discard-confirmed refresh
/// flow rides on that (GRN-17).

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

  /// Edit state of the sheet for one template id.
  ///
  /// Rebuilds (and reseeds) whenever the template detail state changes: the
  /// loading state drops the template, which discards the edits, and a
  /// successful reload seeds the fresh schema — the discard-confirmed refresh
  /// flow rides on that (GRN-17).

  SheetEditControllerProvider call(String templateId) =>
      SheetEditControllerProvider._(argument: templateId, from: this);

  @override
  String toString() => r'sheetEditControllerProvider';
}

/// Edit state of the sheet for one template id.
///
/// Rebuilds (and reseeds) whenever the template detail state changes: the
/// loading state drops the template, which discards the edits, and a
/// successful reload seeds the fresh schema — the discard-confirmed refresh
/// flow rides on that (GRN-17).

abstract class _$SheetEditController extends $Notifier<SheetEditState> {
  late final _$args = ref.$arg as String;
  String get templateId => _$args;

  SheetEditState build(String templateId);
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
