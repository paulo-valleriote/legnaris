import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/auth/secure_token_store.dart';

import '../auth/fakes.dart';

void main() {
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const sheetBody =
      '{"data": {"id": "12345678-1234-7000-8000-123456789abc", "template_id": "fantasy_v1", "name": "Aragorn", "data": {}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';

  ProviderContainer containerWith(List<http.Request> recorded) {
    final container = ProviderContainer(
      overrides: [
        secureTokenStoreProvider.overrideWithValue(InMemorySecureTokenStore()),
        authApiClientProvider.overrideWithValue(
          AuthApiClient(
            httpClient: MockClient(
              (request) async => http.Response(pairBody, 200),
            ),
          ),
        ),
        apiTransportProvider.overrideWithValue(
          MockClient((request) async {
            recorded.add(request);
            return http.Response(sheetBody, 200);
          }),
        ),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  group('domain client wiring (FE6-15)', () {
    test('Happy path: apiClientProvider authenticates GET /sheets with the session token', () async {
      final recorded = <http.Request>[];
      final container = containerWith(recorded);
      await container
          .read(authSessionProvider.notifier)
          .login(email: 'player@example.com', password: 'sup3r-secret');

      final sheet = await container.read(apiClientProvider).getMySheet();

      expect(sheet!.id, '12345678-1234-7000-8000-123456789abc');
      expect(recorded.single.url.path, '/sheets');
      expect(recorded.single.headers['Authorization'], 'Bearer access-1');
    });

    test('Edge: an unauthenticated domain request carries no bearer header', () async {
      final recorded = <http.Request>[];
      final container = containerWith(recorded);

      await container.read(apiClientProvider).getMySheet();

      expect(recorded.single.url.path, '/sheets');
      expect(recorded.single.headers.containsKey('Authorization'), isFalse);
    });
  });
}
