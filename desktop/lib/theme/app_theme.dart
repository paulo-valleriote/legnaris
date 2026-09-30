import 'package:flutter/material.dart';
import 'package:legnaris/theme/tokens.dart';

/// Builds the single app theme (CAT-02): Material 3 defaults with every
/// dimension read from [AppSpacing], [AppRadius] and [AppSizes].
///
/// No palette of its own — colors come from the `ColorScheme` Material
/// derives, and hairlines always use `outlineVariant`. The theme is written
/// once here even for surfaces that arrive in later phases.
ThemeData buildAppTheme() {
  final base = ThemeData(useMaterial3: true);
  // Font sizes are not part of ThemeData.textTheme: they live in the
  // geometry theme and are merged by Theme.of at runtime per script
  // category. V1 is en-only, so the englishLike geometry is baked in here —
  // that is what gives the mapping real sizes to clamp onto the scale.
  final localized = ThemeData.localize(
    base,
    base.typography.geometryThemeFor(ScriptCategory.englishLike),
  );
  return localized.copyWith(
    textTheme: _tokenTextTheme(localized.textTheme),
    primaryTextTheme: _tokenTextTheme(localized.primaryTextTheme),
    dividerTheme: DividerThemeData(
      color: base.colorScheme.outlineVariant,
      thickness: AppSizes.hairline,
      space: AppSizes.hairline,
    ),
    cardTheme: CardThemeData(
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.panel),
        side: BorderSide(
          color: base.colorScheme.outlineVariant,
          width: AppSizes.hairline,
        ),
      ),
    ),
    dialogTheme: DialogThemeData(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.panel),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.control),
        ),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.control),
        ),
      ),
    ),
    listTileTheme: ListTileThemeData(
      contentPadding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
    ),
  );
}

/// Rebuilds [base] so every style lands on the {12, 13, 15, 17} type scale:
/// Material's headline/dialog/title defaults are above the 17px cap and are
/// clamped down through the token bands instead of passing through.
TextTheme _tokenTextTheme(TextTheme base) => TextTheme(
  displayLarge: _tokenStyle(base.displayLarge),
  displayMedium: _tokenStyle(base.displayMedium),
  displaySmall: _tokenStyle(base.displaySmall),
  headlineLarge: _tokenStyle(base.headlineLarge),
  headlineMedium: _tokenStyle(base.headlineMedium),
  headlineSmall: _tokenStyle(base.headlineSmall),
  titleLarge: _tokenStyle(base.titleLarge),
  titleMedium: _tokenStyle(base.titleMedium),
  titleSmall: _tokenStyle(base.titleSmall),
  bodyLarge: _tokenStyle(base.bodyLarge),
  bodyMedium: _tokenStyle(base.bodyMedium),
  bodySmall: _tokenStyle(base.bodySmall),
  labelLarge: _tokenStyle(base.labelLarge),
  labelMedium: _tokenStyle(base.labelMedium),
  labelSmall: _tokenStyle(base.labelSmall),
);

TextStyle? _tokenStyle(TextStyle? style) {
  if (style == null) return null;
  final size = style.fontSize ?? AppSizes.body;
  final double token;
  if (size >= AppSizes.title) {
    token = AppSizes.title;
  } else if (size >= AppSizes.emphasis) {
    token = AppSizes.emphasis;
  } else if (size >= AppSizes.body) {
    token = AppSizes.body;
  } else {
    token = AppSizes.caption;
  }
  return style.copyWith(fontSize: token);
}
