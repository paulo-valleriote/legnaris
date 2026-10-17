import { registerSW } from 'virtual:pwa-register';

// PWA-01 / F1-T006: external registration (no inline script) to support CSP script-src 'self' without unsafe-inline.
export function registerPWA() {
  registerSW({
    immediate: true,
    onNeedRefresh() {
      // Prompt for update; do not overwrite unsaved drafts forcibly.
      // PWA-01 contract: update applies after user confirmation / next navigation.
      console.info('[PWA] Update available');
    },
    onOfflineReady() {
      console.info('[PWA] Ready for offline');
    },
    onRegisteredSW(swUrl, registration) {
      console.info('[PWA] Service worker registered:', swUrl);
    },
  });
}
