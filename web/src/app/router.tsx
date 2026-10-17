import type { ReactElement } from 'react';
import { Route, Routes } from 'react-router-dom';
import { AuthGate } from '../auth/auth_gate';
import { CatalogScreen } from '../catalog/catalog_screen';
import { AppShell } from '../shell/app_shell';
import { NotFound } from '../shell/not_found';
import { SheetScreen } from '../sheet/sheet_screen';
import { YourSheetScreen } from '../sheet/your_sheet_screen';
import { DECLARED_ROUTES, NOT_FOUND_PATH } from './routes';

/** Screen for each declared route, keyed by the path in `routes.ts` (FND-01, FND-08). */
const SCREENS: Record<string, ReactElement> = {
  '/': <CatalogScreen />,
  '/sheets/:id': <SheetScreen />,
  '/your-sheet': <YourSheetScreen />,
  [NOT_FOUND_PATH]: <NotFound />,
};

/**
 * The router's route table. Every screen renders inside `AppShell`, which only
 * mounts once the auth gate holds a session; the single fallback route renders
 * the designed not-found state (FND-08, WAS-08).
 */
export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AuthGate />}>
        <Route element={<AppShell />}>
          {DECLARED_ROUTES.map((route) => (
            <Route key={route.path} path={route.path} element={SCREENS[route.path]} />
          ))}
        </Route>
      </Route>
    </Routes>
  );
}
