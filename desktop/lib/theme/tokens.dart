/// Single source of every size, radius, spacing and text size used by the
/// shell and the screens (CAT-02), transcribed from the `desktop/AGENTS.md`
/// token table. Widgets read these tokens instead of embedding magic numbers.
library;

/// Spacing scale — 4 · 8 · 12 · 16 · 24 only, multiples of 4.
abstract final class AppSpacing {
  /// Tightest gap (icon-to-label, compact control padding).
  static const double xs = 4;

  /// Default gap between related elements.
  static const double sm = 8;

  /// Inner padding of nav entries and rows.
  static const double md = 12;

  /// Outer padding of the content area and card gaps.
  static const double lg = 16;

  /// Largest spacing: section breathing room.
  static const double xl = 24;
}

/// Corner radius — 2px for controls, 4px for surfaces; never above 4.
abstract final class AppRadius {
  /// Buttons, inputs and nav items.
  static const double control = 2;

  /// Cards, dialogs and panels.
  static const double panel = 4;
}

/// Type scale and layout dimensions. Sizes are doubles so they feed
/// `TextStyle.fontSize` directly.
abstract final class AppSizes {
  /// Caption/label size (AGENTS type scale).
  static const double caption = 12;

  /// Base body and input size.
  static const double body = 13;

  /// Emphasis size (secondary headings, table emphasis).
  static const double emphasis = 15;

  /// App/section title — the hard cap of the scale, never above 17.
  static const double title = 17;

  /// Height of table and list rows.
  static const double rowHeight = 32;

  /// Height of inputs and buttons.
  static const double controlHeight = 36;

  /// Border thickness of every hairline divider and card border.
  static const double hairline = 1;

  /// Shell header height — derived from the spacing scale (2× xl).
  static const double headerHeight = AppSpacing.xl * 2;

  /// Persistent left navigation width (spec band: 200–240px).
  static const double leftNavWidth = 220;

  /// Right panel width (spec band: 280–320px).
  static const double rightPanelWidth = 300;

  /// Maximum readable measure of the content area.
  static const double contentMaxWidth = 960;

  /// Minimum content width before the right panel is considered too costly.
  static const double contentMinWidth = 480;

  /// Reserved slot for the connection indicator in the header.
  static const double connectionSlotWidth = 160;

  /// Fixed width of a catalog card.
  static const double catalogCardMinWidth = 280;

  /// Width of the form control column on the sheet screen (Tela 2), so
  /// editable inputs and read-only values share one alignment (RND-06 … RND-10).
  static const double fieldControlWidth = 160;

  /// Tight width of the creation dialog (LGN-100). It holds the two columns
  /// (dialog information and preview card); a tight width keeps `AlertDialog`'s
  /// `IntrinsicWidth` from hugging the content and collapsing the layout.
  static const double createSheetDialogWidth = 720;

  /// Width of the read-only preview card beside the creation dialog
  /// information (LGN-100).
  static const double previewCardWidth = 360;

  /// Bounded height of the preview card body before it scrolls (LGN-100), so
  /// a large template never pushes the dialog past the window.
  static const double previewCardMaxHeight = 420;

  /// Window width below which the right panel hides first (CAT-03).
  /// Derived from its parts — including the two 1px vertical dividers the
  /// shell row lays out — so the content area sits at its minimum measure
  /// exactly at the threshold, and the collapse can never drift from the
  /// tokens it is built on.
  static const double shellMinWidthWithPanel =
      leftNavWidth + contentMinWidth + rightPanelWidth + 2 * hairline;
}
