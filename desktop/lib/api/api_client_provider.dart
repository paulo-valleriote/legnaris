import 'package:http/http.dart' as http;
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/authenticated_client.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'api_client_provider.g.dart';

/// The raw transport the authenticated domain client decorates. This is the
/// single override point tests use to swap in a fake transport.
@Riverpod(keepAlive: true)
http.Client apiTransport(Ref ref) {
  final client = http.Client();
  ref.onDispose(client.close);
  return client;
}

/// App-wide [ApiClient] over the [AuthenticatedClient] decorator, which adds
/// the bearer header and refreshes on a protected `401` through the session
/// delegate. No screen or provider builds its own client.
@Riverpod(keepAlive: true)
ApiClient apiClient(Ref ref) => ApiClient(
  httpClient: AuthenticatedClient(
    inner: ref.watch(apiTransportProvider),
    session: ref.watch(authSessionProvider.notifier),
  ),
);
