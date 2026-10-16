/// <reference lib="webworker" />
import { clientsClaim } from 'workbox-core';
import { ExpirationPlugin } from 'workbox-expiration';
import { cleanupOutdatedCaches, createHandlerBoundToURL, precacheAndRoute } from 'workbox-precaching';
import { NavigationRoute, registerRoute } from 'workbox-routing';
import { NetworkFirst } from 'workbox-strategies';
import { isRuntimeCacheable } from './cache_rules';

declare const self: ServiceWorkerGlobalScope;

// The build's precache manifest is injected here by vite-plugin-pwa.
precacheAndRoute(self.__WB_MANIFEST ?? []);
cleanupOutdatedCaches();

// Navigations resolve to the cached shell so a deep link works offline (FND-07).
registerRoute(new NavigationRoute(createHandlerBoundToURL('/index.html')));

// Public GETs may be cached at runtime; authenticated requests never are (FND-09).
registerRoute(
  ({ request, url }) =>
    isRuntimeCacheable({
      url: url.href,
      method: request.method,
      hasAuthorization: request.headers.has('authorization'),
      isNavigation: request.mode === 'navigate',
    }),
  new NetworkFirst({
    cacheName: 'lg-runtime-v1',
    plugins: [new ExpirationPlugin({ maxEntries: 64, maxAgeSeconds: 60 * 60 })],
  }),
);

// registerType: 'prompt' keeps a new worker in `waiting` until the visitor
// accepts the update, then activates it (FND-09).
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    void self.skipWaiting();
  }
});

self.addEventListener('activate', () => {
  void self.clients.claim();
});

clientsClaim();
