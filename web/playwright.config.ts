import { defineConfig, devices } from '@playwright/test';

const PORT = 4173;

/**
 * Browser-level checks (Chromium engine per the installability contract).
 * Firefox and Safari are verified manually at each release; the Chromium
 * installability audit also runs through Lighthouse (lighthouserc.json).
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://localhost:${PORT}`,
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: `npm run build && npm run preview -- --port ${PORT} --strictPort`,
    url: `http://localhost:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    // Keep the built client same-origin so the browser specs intercept the
    // catalog request locally instead of reaching the production API.
    env: { VITE_API_BASE_URL: `http://localhost:${PORT}` },
  },
});
