// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'main.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// App title shown on the placeholder screen. A trivial provider that proves
/// the Riverpod codegen toolchain wired in Phase 0.

@ProviderFor(appTitle)
final appTitleProvider = AppTitleProvider._();

/// App title shown on the placeholder screen. A trivial provider that proves
/// the Riverpod codegen toolchain wired in Phase 0.

final class AppTitleProvider extends $FunctionalProvider<String, String, String>
    with $Provider<String> {
  /// App title shown on the placeholder screen. A trivial provider that proves
  /// the Riverpod codegen toolchain wired in Phase 0.
  AppTitleProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appTitleProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appTitleHash();

  @$internal
  @override
  $ProviderElement<String> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  String create(Ref ref) {
    return appTitle(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String>(value),
    );
  }
}

String _$appTitleHash() => r'51ac533e62bb0557119ecc361d65495038db67ad';
