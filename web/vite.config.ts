import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { VitePWA } from 'vite-plugin-pwa';

// `appType: 'spa'` makes both the dev and preview servers fall back to
// `index.html` for unknown paths, so a cold deep link resolves before the
// service worker exists (FND-08).
export default defineConfig({
  appType: 'spa',
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
    },
  },
});
