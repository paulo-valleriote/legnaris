import { useEffect, useSyncExternalStore } from 'react';
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { NEXT_PARAM, sanitizeNextPath, signInPathFor } from './next_path';
import { useAuthSession } from '../app/providers';
import { LANDING_PATH, SIGN_IN_PATH, SIGN_UP_PATH } from '../app/routes';
import { message } from '../l10n/strings';
import { LandingScreen } from '../landing/landing_screen';
import { readStartIntent, startTemplateId } from '../landing/start_intent';
import { StartSheet } from '../landing/start_sheet';
import { Toast } from '../ui/toast';
import '../ui/ui.css';
import { AuthBusy } from './auth_busy';
import { AuthCodeScreen } from './auth_code_screen';
import { AuthScreen } from './auth_screen';
import './auth.css';

const AUTH_PATHS: ReadonlySet<string> = new Set([SIGN_IN_PATH, SIGN_UP_PATH]);
const PUBLIC_PATHS: ReadonlySet<string> = new Set([LANDING_PATH, SIGN_IN_PATH, SIGN_UP_PATH]);

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

  const params = new URLSearchParams(location.search);
  const next = sanitizeNextPath(params.get(NEXT_PARAM));
  const startIntent = location.pathname === SIGN_UP_PATH ? readStartIntent(params) : null;

  if (snapshot.state === 'restoring') {
    return <AuthBusy testId="auth-restoring" label={message('auth.restoring')} />;
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
        startIntent !== null && next === null ? (
          <StartSheet templateId={startTemplateId(startIntent)} />
        ) : (
          <Navigate to={next ?? LANDING_PATH} replace />
        )
      ) : (
        <Outlet />
      )}
    </>
  );
}
