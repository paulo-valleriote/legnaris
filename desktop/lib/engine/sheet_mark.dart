/// Preview marking shared by the sheet field controls and the form rows: the
/// user changed an editable field (`modified`) or the last successful dry-run
/// recalculated a derived one (`recalculated`). Read-only surfaces that never
/// run a preview carry no mark.
library;

import 'package:flutter/material.dart';

/// One row's preview marking; null means the row carries no preview marking.
enum SheetMark {
  /// The user's pending delta changed this editable leaf.
  modified,

  /// The last dry-run recalculated this derived leaf.
  recalculated,
}

/// Background tint marking a control the last preview touched, derived from
/// the scheme so it tracks the app palette (desktop/AGENTS.md: no hardcoded
/// hex in widgets).
Color sheetMarkTint(ColorScheme scheme) =>
    scheme.primary.withValues(alpha: 0.12);
