export type RouteAudience = 'public' | 'session';

export interface DeclaredRoute {
  readonly path: string;
  readonly label: string;
  readonly audience: RouteAudience;
  readonly deepLink: true;
  readonly notFound: boolean;
}

export const LANDING_PATH = '/';

export const SIGN_IN_PATH = '/sign-in';
export const SIGN_UP_PATH = '/sign-up';

export const PRIVACY_PATH = '/privacy';

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
  { path: '/account', label: 'Account', audience: 'session', deepLink: true, notFound: false },
  {
    path: '/generate/:templateId',
    label: 'Generate',
    audience: 'session',
    deepLink: true,
    notFound: false,
  },
  {
    path: PRIVACY_PATH,
    label: 'Privacy policy',
    audience: 'public',
    deepLink: true,
    notFound: false,
  },
  { path: NOT_FOUND_PATH, label: 'Not found', audience: 'session', deepLink: true, notFound: true },
];
