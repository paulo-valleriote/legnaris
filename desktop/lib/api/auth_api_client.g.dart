// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'auth_api_client.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The app-wide public auth client over its own plain transport; the decorated
/// domain transport never calls these routes.

@ProviderFor(authApiClient)
final authApiClientProvider = AuthApiClientProvider._();

/// The app-wide public auth client over its own plain transport; the decorated
/// domain transport never calls these routes.

final class AuthApiClientProvider
    extends $FunctionalProvider<AuthApiClient, AuthApiClient, AuthApiClient>
    with $Provider<AuthApiClient> {
  /// The app-wide public auth client over its own plain transport; the decorated
  /// domain transport never calls these routes.
  AuthApiClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'authApiClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$authApiClientHash();

  @$internal
  @override
  $ProviderElement<AuthApiClient> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AuthApiClient create(Ref ref) {
    return authApiClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AuthApiClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AuthApiClient>(value),
    );
  }
}

String _$authApiClientHash() => r'c4b2b26f9841715f11296bd267dbe03605670bf0';
