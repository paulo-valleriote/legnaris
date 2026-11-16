import { loadEnv } from 'vite';
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { VitePWA } from 'vite-plugin-pwa';
import { buildSecurityHeaders, resolveDevApiOrigin } from './config/security_headers.ts';

export default defineConfig(({ mode }) => ({
  appType: 'spa',
  server: {
    headers: buildSecurityHeaders(
      resolveDevApiOrigin(loadEnv(mode, process.cwd(), 'VITE_').VITE_API_BASE_URL),
      true,
    ),
  },
  preview: { headers: buildSecurityHeaders() },
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
      thresholds: {
        statements: 89.5,
        branches: 86,
        functions: 88.5,
        lines: 90.5,
      },
    },
  },
}));
