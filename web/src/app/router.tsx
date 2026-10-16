import type { ReactElement } from 'react';
import { Route, Routes } from 'react-router-dom';
import { AppShell } from '../shell/app_shell';
import { EmptyWorkspace } from '../shell/empty_workspace';
import { NotFound } from '../shell/not_found';
import { DECLARED_ROUTES, NOT_FOUND_PATH } from './routes';

/** Screen for each declared route, keyed by the path in `routes.ts` (FND-01, FND-08). */
const SCREENS: Record<string, ReactElement> = {
  '/': <EmptyWorkspace />,
  [NOT_FOUND_PATH]: <NotFound />,
};

/**
 * The router's route table. Every screen renders inside `AppShell`, and the
 * single fallback route renders the designed not-found state (FND-08).
 */
export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        {DECLARED_ROUTES.map((route) => (
          <Route key={route.path} path={route.path} element={SCREENS[route.path]} />
        ))}
      </Route>
    </Routes>
  );
}
