// Playwright parity reference test — POL-01 / US1
// Must FAIL before parity reference / comparison is configured.
import { test, expect } from '@playwright/test';

test.describe('F10 POL-01 Parity Gate', () => {
  test('template sheet screen matches desktop reference', async ({ page }) => {
    await page.goto('http://localhost:5173/sheet/template-01');
    // Reference comparison requires configured reference artifact + threshold.
    // This test intentionally fails until the parity gate contract is satisfied.
    expect(page.locator('[data-testid="sheet-form"]').count()).toBeGreaterThan(0);
  });
});
