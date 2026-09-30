import 'package:http/http.dart' as http;
import 'package:legnaris/api/api_client.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'api_client_provider.g.dart';

/// App-wide [ApiClient] over a single [http.Client] transport, closed when
/// the provider is disposed. This is the one override point tests use to
/// swap in a fake transport — no screen or provider builds its own client.
/// The `apiClient` function generates the `apiClientProvider` identifier.
@Riverpod(keepAlive: true)
ApiClient apiClient(Ref ref) {
  final client = http.Client();
  ref.onDispose(client.close);
  return ApiClient(httpClient: client);
}
