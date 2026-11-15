import type { ReactElement } from 'react';
import { Route, Routes } from 'react-router-dom';
import { AccountScreen } from '../account/account_screen';
import { AuthGate } from '../auth/auth_gate';
import { CatalogScreen } from '../catalog/catalog_screen';
import { GenerateScreen } from '../generate/generate_screen';
import { AppShell } from '../shell/app_shell';
import { NotFound } from '../shell/not_found';
import { SheetScreen } from '../sheet/sheet_screen';
import { YourSheetScreen } from '../sheet/your_sheet_screen';
import { PrivacyScreen } from '../legal/privacy_screen';
import { DECLARED_ROUTES, NOT_FOUND_PATH, PRIVACY_PATH } from './routes';

/** Screen for each declared route, keyed by the path in `routes.ts` (FND-01, FND-08). */
const SCREENS: Record<string, ReactElement> = {
  '/': <CatalogScreen />,
  '/sheets/:id': <SheetScreen />,
  '/your-sheet': <YourSheetScreen />,
  '/account': <AccountScreen />,
  '/generate/:templateId': <GenerateScreen />,
  [PRIVACY_PATH]: <PrivacyScreen />,
  [NOT_FOUND_PATH]: <NotFound />,
};

/**
 * The router's route table. Public routes mount outside the auth gate so they
 * resolve without a session. Every session screen renders inside `AppShell`, which
 * only mounts once the auth gate holds a session; the single fallback route renders
 * the designed not-found state (FND-08, WAS-08).
 */
export function AppRoutes() {
  const publicRoutes = DECLARED_ROUTES.filter((route) => route.audience === 'public');
  const sessionRoutes = DECLARED_ROUTES.filter((route) => route.audience === 'session');

  return (
    <Routes>
      {publicRoutes.map((route) => (
        <Route key={route.path} path={route.path} element={SCREENS[route.path]} />
      ))}
      <Route element={<AuthGate />}>
        <Route element={<AppShell />}>
          {sessionRoutes.map((route) => (
            <Route key={route.path} path={route.path} element={SCREENS[route.path]} />
          ))}
        </Route>
      </Route>
    </Routes>
  );
}
