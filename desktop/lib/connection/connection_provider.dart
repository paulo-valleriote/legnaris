import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/connection/health_scheduler.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'connection_provider.g.dart';

/// The three states the connection indicator distinguishes: checking (no
/// response yet), online (the last check decoded a 2xx) and offline (the last
/// check failed).
enum ConnectionStatus { checking, online, offline }

/// Render state of the connection: the current [status] plus the timestamp of
/// the last successful check — null until the first success ("never" in the
/// panel). Written only when a check resolves (CONN-06).
class ConnectionState {
  const ConnectionState({
    this.status = ConnectionStatus.checking,
    this.lastSuccess,
  });

  /// Current state; checking until the first response resolves (CONN-02).
  final ConnectionStatus status;

  /// Timestamp of the last successful health check; null → never succeeded.
  final DateTime? lastSuccess;
}

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
@Riverpod(keepAlive: true)
class Connection extends _$Connection {
  bool _inFlight = false;

  @override
  ConnectionState build() {
    final scheduler = ref.watch(healthSchedulerProvider);
    scheduler.start(_check);
    Future.microtask(_check);
    return const ConnectionState();
  }

  Future<void> _check() async {
    if (_inFlight) return;
    _inFlight = true;
    try {
      // The client is read, never watched: a client swap must not rebuild the
      // notifier, restart the schedule or re-fire the boot check (CONN-07).
      await ref.read(apiClientProvider).health();
      state = ConnectionState(
        status: ConnectionStatus.online,
        lastSuccess: DateTime.now(),
      );
    } catch (_) {
      state = ConnectionState(
        status: ConnectionStatus.offline,
        lastSuccess: state.lastSuccess,
      );
    } finally {
      _inFlight = false;
    }
  }
}
