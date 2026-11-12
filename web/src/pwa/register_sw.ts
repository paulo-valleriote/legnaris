import { registerSW } from 'virtual:pwa-register';

/**
 * Owns service-worker registration from an external module (FND-10: no inline
 * registration script). A new version installs in the background and parks until
 * every open window closes, so the next launch runs it without any notice (FND-09).
 */
export function registerServiceWorker(): void {
  registerSW({ immediate: true });
}
