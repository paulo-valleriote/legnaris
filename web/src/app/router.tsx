import { lazy, Suspense } from 'react';
import type { ReactElement } from 'react';
import { Route, Routes } from 'react-router-dom';
import { AccountScreen } from '../account/account_screen';
import { AuthGate } from '../auth/auth_gate';
import { CampaignScreen } from '../campaign/campaign_screen';
import { CampaignsScreen } from '../campaign/campaigns_screen';
import { GmNpcSheetScreen, GmSheetScreen } from '../campaign/gm_sheet_screen';
import { CatalogScreen } from '../catalog/catalog_screen';
import { GenerateScreen } from '../generate/generate_screen';
import { AppShell } from '../shell/app_shell';
import { NotFound } from '../shell/not_found';
import { SheetScreen } from '../sheet/sheet_screen';
import { YourSheetScreen } from '../sheet/your_sheet_screen';
import { PrivacyScreen } from '../legal/privacy_screen';
import { PlansScreen } from '../plans/plans_screen';
import { JoinScreen } from '../join/join_screen';
import {
  CAMPAIGN_DETAIL_PATH,
  CAMPAIGN_NPC_PATH,
  CAMPAIGN_SHEET_PATH,
  CAMPAIGNS_PATH,
  DECLARED_ROUTES,
  DEMO_PATH,
  NOT_FOUND_PATH,
  PLANS_PATH,
  PRIVACY_PATH,
} from './routes';

const DemoScreen = lazy(() => import('../landing/demo_screen'));

const SCREENS: Record<string, ReactElement> = {
  '/': <CatalogScreen />,
  '/sheets/:id': <SheetScreen />,
  '/your-sheet': <YourSheetScreen />,
  '/account': <AccountScreen />,
  '/join/:code': <JoinScreen />,
  [PLANS_PATH]: <PlansScreen />,
  [CAMPAIGNS_PATH]: <CampaignsScreen />,
  [CAMPAIGN_DETAIL_PATH]: <CampaignScreen />,
  [CAMPAIGN_SHEET_PATH]: <GmSheetScreen />,
  [CAMPAIGN_NPC_PATH]: <GmNpcSheetScreen />,
  '/generate/:templateId': <GenerateScreen />,
  [DEMO_PATH]: (
    <Suspense fallback={null}>
      <DemoScreen />
    </Suspense>
  ),
  [PRIVACY_PATH]: <PrivacyScreen />,
  [NOT_FOUND_PATH]: <NotFound />,
};

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
