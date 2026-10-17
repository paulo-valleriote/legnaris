/**
 * The declared route table (FND-08). The router builds its `<Routes>` from this
 * list, so a route added here is deep-linkable and enumerable by tests.
 */

export type RouteAudience = 'public' | 'session';

export interface DeclaredRoute {
  readonly path: string;
  readonly label: string;
  readonly audience: RouteAudience;
  /** Every declared route resolves identically on a direct navigation (FND-08). */
  readonly deepLink: true;
  readonly notFound: boolean;
}

/** The single fallback route; renders the designed not-found state (FND-08). */
export const NOT_FOUND_PATH = '*';

export const DECLARED_ROUTES: readonly DeclaredRoute[] = [
  { path: '/', label: 'Catalog', audience: 'session', deepLink: true, notFound: false },
  { path: '/sheets/:id', label: 'Sheet', audience: 'session', deepLink: true, notFound: false },
  {
    path: '/your-sheet',
    label: 'Your sheet',
    audience: 'session',
    deepLink: true,
    notFound: false,
  },
  { path: NOT_FOUND_PATH, label: 'Not found', audience: 'session', deepLink: true, notFound: true },
];
