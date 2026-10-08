// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'auth_session.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Owns the in-memory access token and drives the refresh-token lifecycle
/// against [SecureTokenStore] and the public [AuthApiClient] (FE6-01 … FE6-07).

@ProviderFor(AuthSession)
final authSessionProvider = AuthSessionProvider._();

/// Owns the in-memory access token and drives the refresh-token lifecycle
/// against [SecureTokenStore] and the public [AuthApiClient] (FE6-01 … FE6-07).
final class AuthSessionProvider
    extends $NotifierProvider<AuthSession, AuthState> {
  /// Owns the in-memory access token and drives the refresh-token lifecycle
  /// against [SecureTokenStore] and the public [AuthApiClient] (FE6-01 … FE6-07).
  AuthSessionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'authSessionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$authSessionHash();

  @$internal
  @override
  AuthSession create() => AuthSession();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AuthState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AuthState>(value),
    );
  }
}

String _$authSessionHash() => r'680d0f5d53171c29eb79eb6dd87245efe0a117ca';

/// Owns the in-memory access token and drives the refresh-token lifecycle
/// against [SecureTokenStore] and the public [AuthApiClient] (FE6-01 … FE6-07).

abstract class _$AuthSession extends $Notifier<AuthState> {
  AuthState build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<AuthState, AuthState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AuthState, AuthState>,
              AuthState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}
