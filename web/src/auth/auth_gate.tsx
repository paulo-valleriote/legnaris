/**
 * The auth gate (WAS-03, WAS-04, WAS-05, WAS-08, WAS-24, R8).
 *
 * It runs the one cold-start restore on mount, renders the minimal restoring
 * state until that restore settles, renders `AuthScreen` whenever no session is
 * held (the catalog, the sheet and the "Your sheet" destination never mount), and
 * renders the route tree when authenticated. A deep link visited without a
 * session is not resumed: as soon as the session settles unauthenticated, the
 * location resets to `/`, so the next session starts on the catalog.
 */

import { useEffect, useSyncExternalStore } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useAuthSession } from '../app/providers';
import { message } from '../l10n/strings';
import { AuthScreen } from './auth_screen';
import './auth.css';

/** The gate that decides what the whole client renders (R8). */
export function AuthGate() {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    void session.restore();
  }, [session]);

  useEffect(() => {
    if (snapshot.state === 'unauthenticated' && location.pathname !== '/') {
      navigate('/', { replace: true });
    }
  }, [snapshot.state, location.pathname, navigate]);

  if (snapshot.state === 'restoring') {
    return (
      <p className="lg-auth__restoring" role="status" data-testid="auth-restoring">
        {message('auth.restoring')}
      </p>
    );
  }

  if (snapshot.state !== 'authenticated') {
    return <AuthScreen />;
  }

  return <Outlet />;
}
