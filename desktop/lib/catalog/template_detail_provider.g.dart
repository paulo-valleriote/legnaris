// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'template_detail_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Detail of one template, fetched on demand for the creation surface
/// (TPL-01).
///
/// The family is autoDispose: the creation dialog disposes it when it closes,
/// so every reopen fetches again with no cached reuse (TPL-15) and a new
/// template id can never inherit a previous summary (TPL-04). Nothing is ever
/// persisted.

@ProviderFor(TemplateDetailController)
final templateDetailControllerProvider = TemplateDetailControllerFamily._();

/// Detail of one template, fetched on demand for the creation surface
/// (TPL-01).
///
/// The family is autoDispose: the creation dialog disposes it when it closes,
/// so every reopen fetches again with no cached reuse (TPL-15) and a new
/// template id can never inherit a previous summary (TPL-04). Nothing is ever
/// persisted.
final class TemplateDetailControllerProvider
    extends $NotifierProvider<TemplateDetailController, TemplateDetailState> {
  /// Detail of one template, fetched on demand for the creation surface
  /// (TPL-01).
  ///
  /// The family is autoDispose: the creation dialog disposes it when it closes,
  /// so every reopen fetches again with no cached reuse (TPL-15) and a new
  /// template id can never inherit a previous summary (TPL-04). Nothing is ever
  /// persisted.
  TemplateDetailControllerProvider._({
    required TemplateDetailControllerFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'templateDetailControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$templateDetailControllerHash();

  @override
  String toString() {
    return r'templateDetailControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  TemplateDetailController create() => TemplateDetailController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(TemplateDetailState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<TemplateDetailState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is TemplateDetailControllerProvider &&
        other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$templateDetailControllerHash() =>
    r'2aaf2f038b027f8e59c6e865065e07eaad37227d';

/// Detail of one template, fetched on demand for the creation surface
/// (TPL-01).
///
/// The family is autoDispose: the creation dialog disposes it when it closes,
/// so every reopen fetches again with no cached reuse (TPL-15) and a new
/// template id can never inherit a previous summary (TPL-04). Nothing is ever
/// persisted.

final class TemplateDetailControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          TemplateDetailController,
          TemplateDetailState,
          TemplateDetailState,
          TemplateDetailState,
          String
        > {
  TemplateDetailControllerFamily._()
    : super(
        retry: null,
        name: r'templateDetailControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// Detail of one template, fetched on demand for the creation surface
  /// (TPL-01).
  ///
  /// The family is autoDispose: the creation dialog disposes it when it closes,
  /// so every reopen fetches again with no cached reuse (TPL-15) and a new
  /// template id can never inherit a previous summary (TPL-04). Nothing is ever
  /// persisted.

  TemplateDetailControllerProvider call(String templateId) =>
      TemplateDetailControllerProvider._(argument: templateId, from: this);

  @override
  String toString() => r'templateDetailControllerProvider';
}

/// Detail of one template, fetched on demand for the creation surface
/// (TPL-01).
///
/// The family is autoDispose: the creation dialog disposes it when it closes,
/// so every reopen fetches again with no cached reuse (TPL-15) and a new
/// template id can never inherit a previous summary (TPL-04). Nothing is ever
/// persisted.

abstract class _$TemplateDetailController
    extends $Notifier<TemplateDetailState> {
  late final _$args = ref.$arg as String;
  String get templateId => _$args;

  TemplateDetailState build(String templateId);
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<TemplateDetailState, TemplateDetailState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<TemplateDetailState, TemplateDetailState>,
              TemplateDetailState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, () => build(_$args));
  }
}
