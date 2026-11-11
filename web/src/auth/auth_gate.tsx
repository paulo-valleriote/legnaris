/**
 * The auth gate (WAS-03, WAS-04, WAS-05, WAS-08, WAS-24, R8, WCS-02, WCS-21).
 *
 * It runs the one cold-start restore on mount, renders the minimal restoring
 * state until that restore settles, renders the landing on `/` and `AuthScreen`
 * on every other path whenever no session is held (the catalog, the sheet and
 * the "Your sheet" destination never mount), and renders the route tree when
 * authenticated. A deep link visited without a session is not resumed: as soon
 * as the session settles unauthenticated, the location moves to the sign-in
 * path, and once signed in the auth paths hand over to `/`, so the next session
 * starts on the catalog.
 *
 * A pending code step replaces the credential form in place, on the same URL
 * and with no declared route (R5). The session keeps the values the abandoned
 * step started with, so the form comes back with the address and the account
 * mode the person typed and with an empty password (WCS-21).
 */

import { useEffect, useSyncExternalStore } from 'react';
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { NEXT_PARAM, sanitizeNextPath, signInPathFor } from './next_path';
import { useAuthSession } from '../app/providers';
import { LANDING_PATH, SIGN_IN_PATH, SIGN_UP_PATH } from '../app/routes';
import { message } from '../l10n/strings';
import { LandingScreen } from '../landing/landing_screen';
import { Toast } from '../ui/toast';
import '../ui/ui.css';
import { AuthCodeScreen } from './auth_code_screen';
import { AuthScreen } from './auth_screen';
import './auth.css';

const AUTH_PATHS: ReadonlySet<string> = new Set([SIGN_IN_PATH, SIGN_UP_PATH]);
const PUBLIC_PATHS: ReadonlySet<string> = new Set([LANDING_PATH, SIGN_IN_PATH, SIGN_UP_PATH]);

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
    if (snapshot.state === 'unauthenticated' && !PUBLIC_PATHS.has(location.pathname)) {
      navigate(signInPathFor(`${location.pathname}${location.search}${location.hash}`), {
        replace: true,
      });
    }
  }, [snapshot.state, location.pathname, location.search, location.hash, navigate]);

  const next = sanitizeNextPath(new URLSearchParams(location.search).get(NEXT_PARAM));

  if (snapshot.state === 'restoring') {
    return (
      <p className="lg-auth__restoring" role="status" data-testid="auth-restoring">
        {message('auth.restoring')}
      </p>
    );
  }

  if (snapshot.state === 'pendingCode') {
    return <AuthCodeScreen />;
  }

  if (snapshot.state !== 'authenticated') {
    if (snapshot.restored === null && location.pathname === LANDING_PATH) {
      return <LandingScreen />;
    }

    return (
      <AuthScreen
        restore={snapshot.restored}
        initialMode={location.pathname === SIGN_UP_PATH ? 'register' : 'login'}
        next={next}
      />
    );
  }

  return (
    <>
      {snapshot.notice === 'verified' && (
        <Toast
          message={message('auth.verifiedToast')}
          variant="success"
          onDismiss={() => session.dismissNotice()}
        />
      )}
      {AUTH_PATHS.has(location.pathname) ? (
        <Navigate to={next ?? LANDING_PATH} replace />
      ) : (
        <Outlet />
      )}
    </>
  );
}
