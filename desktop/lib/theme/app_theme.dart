import 'package:flutter/material.dart';
import 'package:legnaris/theme/tokens.dart';

/// V1 palette seed: blue-gray — neutral and legible with no brand identity
/// (that stays V2). The seed keeps Material's tonal logic but replaces its
/// default purple baseline, which the UAT rejected: purple-tinted neutrals
/// across the whole shell (T8).
const Color _paletteSeed = Color(0xFF546E7A);

/// Minimum button box: Material's own 64px default width with the height
/// re-pinned to the control token — M3 ships 40px, outside the 32–36 band.
const Size _buttonMinSize = Size(64, AppSizes.controlHeight);

/// Builds the single app theme (CAT-02): Material 3 structure with every
/// dimension read from [AppSpacing], [AppRadius] and [AppSizes], and a
/// [ColorScheme] seeded from [_paletteSeed].
///
/// The scheme is passed to the constructor so every derived default
/// (scaffold surface, hairlines via `outlineVariant`, hover/focus tints)
/// follows it instead of Material's stock palette. Widgets keep reading
/// colors from `ColorScheme` only, per AGENTS. The theme is written once
/// here even for surfaces that arrive in later phases.
ThemeData buildAppTheme() {
  final base = ThemeData(
    useMaterial3: true,
    colorScheme: ColorScheme.fromSeed(seedColor: _paletteSeed),
  );
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
        minimumSize: _buttonMinSize,
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.control),
        ),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        minimumSize: _buttonMinSize,
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
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
