/// <reference lib="webworker" />

import { clientsClaim } from 'workbox-core';
import { precacheAndRoute } from 'workbox-precaching';
import { registerRoute, setCatchHandler } from 'workbox-routing';
import { StaleWhileRevalidate, CacheFirst, NetworkFirst } from 'workbox-strategies';
import { ExpirationPlugin } from 'workbox-expiration';
import { isRuntimeCacheable, LEGACY_RUNTIME_CACHE_NAMES, RUNTIME_CACHE_NAME } from './cache_rules';

const MANIFEST = self.__WB_MANIFEST as Array<{ url: string; revision?: string | null }>;

precacheAndRoute(
  MANIFEST.filter((entry) => {
    const url = entry.url;
    return !url.includes('/auth/') && !url.includes('/csp-report');
  }),
);

clientsClaim();

self.addEventListener('activate', (event: ExtendableEvent) => {
  event.waitUntil(
    Promise.all(LEGACY_RUNTIME_CACHE_NAMES.map((name) => caches.delete(name))).then(() => undefined),
  );
});

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

setCatchHandler(async ({ request }) => {
  if (request.mode === 'navigate') {
    return caches.match('/');
  }
  return Response.error();
});
