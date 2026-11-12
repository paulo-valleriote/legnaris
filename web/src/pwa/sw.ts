/// <reference lib="webworker" />
// Service worker (injectManifest): precache shell; runtime cache safe GET only (no auth).
// PWA-01 contract.

import { clientsClaim } from 'workbox-core';
import { precacheAndRoute } from 'workbox-precaching';
import { registerRoute, setCatchHandler } from 'workbox-routing';
import { StaleWhileRevalidate, CacheFirst, NetworkFirst } from 'workbox-strategies';
import { ExpirationPlugin } from 'workbox-expiration';
import { isRuntimeCacheable, LEGACY_RUNTIME_CACHE_NAMES, RUNTIME_CACHE_NAME } from './cache_rules';

// Self is workbox-injected precache manifest
const MANIFEST = self.__WB_MANIFEST as Array<{ url: string; revision?: string | null }>;

// PWA-01: precache shell and icons; do NOT precache authenticated endpoints (/auth/*, /sheets auth-required).
precacheAndRoute(
  MANIFEST.filter((entry) => {
    const url = entry.url;
    return !url.includes('/auth/') && !url.includes('/csp-report');
  }),
);

clientsClaim();

// A new worker stays parked until every client of the old one is closed, so an open
// session never swaps code mid-edit; the next launch runs the new version silently.

self.addEventListener('activate', (event: ExtendableEvent) => {
  event.waitUntil(
    Promise.all(LEGACY_RUNTIME_CACHE_NAMES.map((name) => caches.delete(name))).then(() => undefined),
  );
});

// Runtime caching covers same-origin static assets only: API responses are scoped to
// the signed-in caller by cookie, so serving one from cache shows stale or deleted data.
registerRoute(
  ({ request, url }) =>
    isRuntimeCacheable(
      {
        url: url.href,
        method: request.method,
        hasAuthorization: request.headers.has('Authorization'),
        isNavigation: request.mode === 'navigate',
      },
      self.location.origin,
    ),
  new StaleWhileRevalidate({
    cacheName: RUNTIME_CACHE_NAME,
    plugins: [new ExpirationPlugin({ maxEntries: 50, maxAgeSeconds: 86400 })],
  }),
);

// Deep-link fallback (PWA-03): return cached index.html for navigation requests
setCatchHandler(async ({ request }) => {
  if (request.mode === 'navigate') {
    return caches.match('/');
  }
  return Response.error();
});
