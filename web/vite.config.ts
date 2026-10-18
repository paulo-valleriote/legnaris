import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { VitePWA } from 'vite-plugin-pwa';

// The document security headers the origin delivers to the web client. In
// production they come from Traefik (`deploy/traefik/dynamic.yml`
// `web-security-headers`); the dev and preview servers mirror them so the
// header gate asserts the real policy locally too (POL-02, B6). Keep the two
// lists in step — the policy is defined once per origin, never in the HTML.
const contentSecurityPolicy = [
  "default-src 'self'",
  "script-src 'self'",
  "style-src 'self'",
  "img-src 'self' data:",
  "font-src 'self'",
  "connect-src 'self' https://api.legnaris.com.br",
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'none'",
  "require-trusted-types-for 'script'",
  'report-to csp-endpoint',
  'report-uri https://api.legnaris.com.br/csp-report',
].join('; ');

const securityHeaders = {
  'Content-Security-Policy': contentSecurityPolicy,
  'Reporting-Endpoints': 'csp-endpoint="https://api.legnaris.com.br/csp-report"',
  'Strict-Transport-Security': 'max-age=63072000; includeSubDomains; preload',
  'X-Frame-Options': 'DENY',
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'strict-origin-when-cross-origin',
  'Permissions-Policy':
    'camera=(), microphone=(), geolocation=(), payment=(), usb=(), fullscreen=(self)',
  'Cross-Origin-Opener-Policy': 'same-origin',
  'Cross-Origin-Resource-Policy': 'same-origin',
};

// `appType: 'spa'` makes both the dev and preview servers fall back to
// `index.html` for unknown paths, so a cold deep link resolves before the
// service worker exists (FND-08).
export default defineConfig({
  appType: 'spa',
  server: { headers: securityHeaders },
  preview: { headers: securityHeaders },
  plugins: [
    react(),
    VitePWA({
      strategies: 'injectManifest',
      srcDir: 'src/pwa',
      filename: 'sw.ts',
      injectRegister: false,
      registerType: 'prompt',
      manifest: false,
      injectManifest: {
        globPatterns: ['**/*.{js,css,html,svg,png,ico,webmanifest}'],
      },
      devOptions: { enabled: false },
    }),
  ],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}', 'src/**/*.spec.{ts,tsx}'],
    css: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary'],
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/**/*.test.{ts,tsx}', 'src/test/**', 'src/vite-env.d.ts', 'src/pwa/sw.ts'],
      // Floors just under the current suite so a real regression breaks the run
      // while ordinary churn does not (F10-T003 / SC-06). Raise them as coverage
      // grows; never lower them to make a change pass.
      thresholds: {
        statements: 89.5,
        branches: 86,
        functions: 88.5,
        lines: 90.5,
      },
    },
  },
});
