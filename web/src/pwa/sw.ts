/// <reference lib="webworker" />
// Service worker (injectManifest): precache shell; runtime cache safe GET only (no auth).
// PWA-01 contract.

import { clientsClaim } from 'workbox-core';
import { precacheAndRoute } from 'workbox-precaching';
import { registerRoute, setCatchHandler } from 'workbox-routing';
import { StaleWhileRevalidate, CacheFirst, NetworkFirst } from 'workbox-strategies';
import { ExpirationPlugin } from 'workbox-expiration';

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

// Runtime caching: GET only, non-auth, safe endpoints
registerRoute(
  ({ request, url }) => {
    return (
      request.method === 'GET' &&
      url.pathname.startsWith('/') &&
      !url.pathname.startsWith('/auth/') &&
      !url.pathname.includes('/csp-report')
    );
  },
  new StaleWhileRevalidate({
    cacheName: 'pwa-runtime-safe',
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
