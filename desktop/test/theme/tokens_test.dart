import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/theme/tokens.dart';

void main() {
  group('type scale (CAT-02, AGENTS tokens)', () {
    test('Happy path: type scale is exactly {12, 13, 15, 17}', () {
      expect(
        {AppSizes.caption, AppSizes.body, AppSizes.emphasis, AppSizes.title},
        {12, 13, 15, 17},
      );
    });
  });

  group('spacing, radius and control sizes (AGENTS tokens)', () {
    test('Happy path: spacing tokens are the fixed {4, 8, 12, 16, 24} scale', () {
      expect(
        {AppSpacing.xs, AppSpacing.sm, AppSpacing.md, AppSpacing.lg, AppSpacing.xl},
        {4, 8, 12, 16, 24},
      );
    });

    test('Happy path: radius tokens are exactly the 2px/4px pair and never exceed 4', () {
      expect({AppRadius.control, AppRadius.panel}, {2, 4});
      expect(
        [AppRadius.control, AppRadius.panel],
        everyElement(lessThanOrEqualTo(4)),
      );
    });

    test('Happy path: row, control and hairline sizes follow the AGENTS table', () {
      expect(AppSizes.rowHeight, 32);
      expect(AppSizes.controlHeight, inInclusiveRange(32, 36));
      expect(AppSizes.hairline, 1);
    });
  });

  group('shell dimensions (CAT-02, design widths)', () {
    test('Happy path: left nav width token sits inside the 200–240px band', () {
      expect(AppSizes.leftNavWidth, inInclusiveRange(200, 240));
      expect(AppSizes.leftNavWidth, 220);
    });

    test('Happy path: right panel width token sits inside the 280–320px band', () {
      expect(AppSizes.rightPanelWidth, inInclusiveRange(280, 320));
      expect(AppSizes.rightPanelWidth, 300);
    });

    test('Happy path: content measure tokens match the readable width', () {
      expect(AppSizes.contentMaxWidth, 960);
      expect(AppSizes.contentMinWidth, 480);
    });

    test('Happy path: shell slot tokens use the design sizes', () {
      expect(AppSizes.connectionSlotWidth, 160);
      expect(AppSizes.catalogCardMinWidth, 280);
    });
  });

  group('creation dialog preview (LGN-100)', () {
    test('Happy path: preview layout tokens use the design sizes', () {
      expect(AppSizes.createSheetDialogWidth, 720);
      expect(AppSizes.previewCardWidth, 360);
      expect(AppSizes.previewCardMaxHeight, 420);
    });
  });

  group('derived values (no magic numbers)', () {
    test('Happy path: shellMinWidthWithPanel is derived from its parts', () {
      expect(
        AppSizes.shellMinWidthWithPanel,
        AppSizes.leftNavWidth +
            AppSizes.contentMinWidth +
            AppSizes.rightPanelWidth +
            2 * AppSizes.hairline,
      );
      expect(AppSizes.shellMinWidthWithPanel, 1002);
    });

    test('Happy path: header height derives from the spacing scale', () {
      expect(AppSizes.headerHeight, AppSpacing.xl * 2);
      expect(AppSizes.headerHeight, 48);
    });
  });
}
