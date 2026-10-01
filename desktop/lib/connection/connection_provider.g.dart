// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'connection_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Connection state machine and request policy (CONN-01 … CONN-07, CONN-13):
/// the single writer of connection state, fed by exactly one source —
/// `ApiClient.health()`.
///
/// [build] starts the injectable schedule and fires the boot check through a
/// microtask, so polling begins at app start without waiting for the first
/// tick (CONN-01). Checks are single-flight: a tick while one is in flight is
/// discarded (CONN-05). A resolved check writes `online` plus its timestamp
/// (CONN-03, CONN-13 — any decoded 2xx payload counts); any throw writes
/// `offline` carrying the previous `lastSuccess` (CONN-04, CONN-09). While a
/// check flies, the current state holds (CONN-06).

@ProviderFor(Connection)
final connectionProvider = ConnectionProvider._();

/// Connection state machine and request policy (CONN-01 … CONN-07, CONN-13):
/// the single writer of connection state, fed by exactly one source —
/// `ApiClient.health()`.
///
/// [build] starts the injectable schedule and fires the boot check through a
/// microtask, so polling begins at app start without waiting for the first
/// tick (CONN-01). Checks are single-flight: a tick while one is in flight is
/// discarded (CONN-05). A resolved check writes `online` plus its timestamp
/// (CONN-03, CONN-13 — any decoded 2xx payload counts); any throw writes
/// `offline` carrying the previous `lastSuccess` (CONN-04, CONN-09). While a
/// check flies, the current state holds (CONN-06).
final class ConnectionProvider
    extends $NotifierProvider<Connection, ConnectionState> {
  /// Connection state machine and request policy (CONN-01 … CONN-07, CONN-13):
  /// the single writer of connection state, fed by exactly one source —
  /// `ApiClient.health()`.
  ///
  /// [build] starts the injectable schedule and fires the boot check through a
  /// microtask, so polling begins at app start without waiting for the first
  /// tick (CONN-01). Checks are single-flight: a tick while one is in flight is
  /// discarded (CONN-05). A resolved check writes `online` plus its timestamp
  /// (CONN-03, CONN-13 — any decoded 2xx payload counts); any throw writes
  /// `offline` carrying the previous `lastSuccess` (CONN-04, CONN-09). While a
  /// check flies, the current state holds (CONN-06).
  ConnectionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'connectionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$connectionHash();

  @$internal
  @override
  Connection create() => Connection();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ConnectionState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ConnectionState>(value),
    );
  }
}

String _$connectionHash() => r'51074fc74deefe25ccd4f4ebfefb374a23a7549e';

/// Connection state machine and request policy (CONN-01 … CONN-07, CONN-13):
/// the single writer of connection state, fed by exactly one source —
/// `ApiClient.health()`.
///
/// [build] starts the injectable schedule and fires the boot check through a
/// microtask, so polling begins at app start without waiting for the first
/// tick (CONN-01). Checks are single-flight: a tick while one is in flight is
/// discarded (CONN-05). A resolved check writes `online` plus its timestamp
/// (CONN-03, CONN-13 — any decoded 2xx payload counts); any throw writes
/// `offline` carrying the previous `lastSuccess` (CONN-04, CONN-09). While a
/// check flies, the current state holds (CONN-06).

abstract class _$Connection extends $Notifier<ConnectionState> {
  ConnectionState build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<ConnectionState, ConnectionState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ConnectionState, ConnectionState>,
              ConnectionState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}
