import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/main.dart';
import 'package:legnaris/theme/app_theme.dart';
import 'package:legnaris/theme/tokens.dart';

double _radiusOf(ShapeBorder? shape) =>
    ((shape as RoundedRectangleBorder).borderRadius as BorderRadius).topLeft.x;

void main() {
  final theme = buildAppTheme();

  group('type scale mapping (CAT-02)', () {
    test('Edge: M3 styles above the 17px cap land on the title size', () {
      // The M3 defaults for these are 57/45/22 — larger than the AGENTS cap.
      expect(theme.textTheme.displayLarge?.fontSize, AppSizes.title);
      expect(theme.textTheme.displayMedium?.fontSize, AppSizes.title);
      expect(theme.textTheme.titleLarge?.fontSize, AppSizes.title);
    });

    test(
      'Happy path: base body and caption styles land on their token sizes',
      () {
        expect(theme.textTheme.bodyMedium?.fontSize, AppSizes.body);
        expect(theme.textTheme.bodySmall?.fontSize, AppSizes.caption);
      },
    );

    test(
      'Happy path: every text style lands on the {12, 13, 15, 17} scale',
      () {
        final styles = <String, TextStyle?>{
          'displayLarge': theme.textTheme.displayLarge,
          'displayMedium': theme.textTheme.displayMedium,
          'displaySmall': theme.textTheme.displaySmall,
          'headlineLarge': theme.textTheme.headlineLarge,
          'headlineMedium': theme.textTheme.headlineMedium,
          'headlineSmall': theme.textTheme.headlineSmall,
          'titleLarge': theme.textTheme.titleLarge,
          'titleMedium': theme.textTheme.titleMedium,
          'titleSmall': theme.textTheme.titleSmall,
          'bodyLarge': theme.textTheme.bodyLarge,
          'bodyMedium': theme.textTheme.bodyMedium,
          'bodySmall': theme.textTheme.bodySmall,
          'labelLarge': theme.textTheme.labelLarge,
          'labelMedium': theme.textTheme.labelMedium,
          'labelSmall': theme.textTheme.labelSmall,
        };

        for (final entry in styles.entries) {
          expect(
            entry.value?.fontSize,
            isIn({12, 13, 15, 17}),
            reason: '${entry.key} must land on the type scale',
          );
        }
        expect(styles.values.map((style) => style?.fontSize).toSet(), {
          12,
          13,
          15,
          17,
        }, reason: 'the theme must use the full type scale');
      },
    );
  });

  group('theme components read the tokens', () {
    test('Happy path: theme enables Material 3 on the seeded app scheme', () {
      expect(theme.useMaterial3, isTrue);
    });

    test('Happy path: divider theme reads the hairline token and outlineVariant color', () {
      expect(theme.dividerTheme.thickness, AppSizes.hairline);
      expect(theme.dividerTheme.space, AppSizes.hairline);
      expect(theme.dividerTheme.color, theme.colorScheme.outlineVariant);
    });

    test('Happy path: card theme reads panel radius, zero elevation and hairline border', () {
      expect(theme.cardTheme.elevation, 0);
      expect(_radiusOf(theme.cardTheme.shape), AppRadius.panel);
      final side = (theme.cardTheme.shape as RoundedRectangleBorder).side;
      expect(side.width, AppSizes.hairline);
      expect(side.color, theme.colorScheme.outlineVariant);
    });

    test('Happy path: dialog theme reads the panel radius', () {
      expect(_radiusOf(theme.dialogTheme.shape), AppRadius.panel);
    });

    test(
      'Happy path: outlined and text button themes read the control radius',
      () {
        final outlined = theme.outlinedButtonTheme.style?.shape?.resolve(
          const <WidgetState>{},
        );
        final text = theme.textButtonTheme.style?.shape?.resolve(
          const <WidgetState>{},
        );
        expect(_radiusOf(outlined), AppRadius.control);
        expect(_radiusOf(text), AppRadius.control);
      },
    );
  });

  group('palette and control sizing (UAT T8)', () {
    test(
      'Edge: the palette stays out of Material\'s default purple baseline',
      () {
        final hue = HSLColor.fromColor(theme.colorScheme.primary).hue;
        expect(
          hue >= 240 && hue <= 300,
          isFalse,
          reason:
              'primary landed in the violet band (T8: purple rejected in UAT)',
        );
        expect(theme.colorScheme.primary, isNot(const Color(0xFF6750A4)));
      },
    );

    test('Happy path: the scaffold background follows the scheme surface', () {
      expect(theme.scaffoldBackgroundColor, theme.colorScheme.surface);
    });

    test('Happy path: button themes pin the control height and shrink the tap target', () {
      for (final style in <ButtonStyle?>[
        theme.textButtonTheme.style,
        theme.outlinedButtonTheme.style,
      ]) {
        final minSize = style?.minimumSize?.resolve(const <WidgetState>{});
        expect(minSize?.height, AppSizes.controlHeight);
        expect(style?.tapTargetSize, MaterialTapTargetSize.shrinkWrap);
      }
    });
  });

  group('MaterialApp wiring', () {
    testWidgets(
      'Happy path: MaterialApp wires the app theme and the placeholder still renders',
      (tester) async {
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              apiClientProvider.overrideWithValue(
                ApiClient(
                  httpClient: MockClient((request) async {
                    if (request.url.path == '/health') {
                      return http.Response(
                        '{"data": {"status": "healthy"}}',
                        200,
                      );
                    }
                    return http.Response('{"data": []}', 200);
                  }),
                ),
              ),
            ],
            child: const LegnarisApp(),
          ),
        );

        expect(find.text('Legnaris'), findsOneWidget);
        final wired = Theme.of(tester.element(find.text('Legnaris')));
        expect(wired.useMaterial3, isTrue);
        expect(wired.textTheme.bodyMedium?.fontSize, AppSizes.body);
        expect(_radiusOf(wired.cardTheme.shape), AppRadius.panel);
      },
    );
  });
}
