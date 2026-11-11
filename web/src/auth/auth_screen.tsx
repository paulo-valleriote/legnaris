/**
 * The auth surface (WAS-07, WAS-09, WAS-10, R9, WCS-21, WCS-22).
 *
 * One place offers sign-in and registration through a mode switch, opening with
 * a mode-specific title beside the landing's photograph, so the surface carries
 * the same identity as the page that led to it. The single primary action submits through the session,
 * is disabled and shows progress while a submit is in flight, and the password
 * is a hidden-content control. A failed submit keeps the typed values and
 * renders the mapped failure copy resolved from the catalog — never the server
 * message, so the copy never reveals whether an email exists (WAS-10). Switching
 * mode clears a stale error but keeps the values.
 *
 * A form that comes back from an abandoned code step returns with the address
 * and the account mode the step started with and with an empty password
 * (WCS-21); the password never was part of the pending step.
 */

import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import type { FormEvent, KeyboardEvent } from 'react';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { FailureKind } from '../api/errors';
import { useAuthSession } from '../app/providers';
import { message } from '../l10n/strings';
import type { RestoredForm } from './auth_session';
import { isSheetPath } from './next_path';
import { AuthStage } from './auth_stage';
import './auth.css';

export type AuthMode = 'login' | 'register';

export type { RestoredForm };

interface AuthScreenProps {
  readonly restore?: RestoredForm | null;
  /** The mode the surface opens in when no abandoned code step dictates one. */
  readonly initialMode?: AuthMode;
  /** The sanitised destination to return to after sign-in, when one was carried. */
  readonly next?: string | null;
}

const MODES: readonly { readonly mode: AuthMode }[] = [{ mode: 'login' }, { mode: 'register' }];

function subtitle(mode: AuthMode, next: string | null): string {
  if (isSheetPath(next)) {
    return mode === 'login'
      ? message('auth.subtitleLoginSheet')
      : message('auth.subtitleRegisterSheet');
  }
  return mode === 'login' ? message('auth.subtitleLogin') : message('auth.subtitleRegister');
}

/** The single sign-in / registration surface (WAS-09). */
export function AuthScreen({ restore, initialMode = 'login', next = null }: AuthScreenProps) {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const [mode, setMode] = useState<AuthMode>(restore?.origin ?? initialMode);
  const [email, setEmail] = useState(restore?.email ?? '');
  const [username, setUsername] = useState(restore?.username ?? '');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<
    { readonly copy: string; readonly kind: FailureKind } | undefined
  >(undefined);
  const [showPassword, setShowPassword] = useState(false);
  const [passwordFocus, setPasswordFocus] = useState(0);
  const busy = snapshot.state === 'authenticating';
  const emailRef = useRef<HTMLInputElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (restore !== undefined && restore !== null) {
      emailRef.current?.focus();
    }
  }, [restore]);

  useEffect(() => {
    if (passwordFocus > 0) {
      passwordRef.current?.focus();
    }
  }, [passwordFocus]);

  const switchMode = (next: AuthMode) => {
    setMode(next);
    setError(undefined);
  };

  const switchToSignIn = () => {
    setMode('login');
    setError(undefined);
    setPasswordFocus((value) => value + 1);
  };

  const onTabKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
      return;
    }
    event.preventDefault();
    const current = MODES.findIndex((tab) => tab.mode === mode);
    let target = (current + (event.key === 'ArrowRight' ? 1 : -1) + MODES.length) % MODES.length;
    if (event.key === 'Home') {
      target = 0;
    } else if (event.key === 'End') {
      target = MODES.length - 1;
    }
    const nextMode = MODES[target].mode;
    switchMode(nextMode);
    document.getElementById(`auth-tab-${nextMode}`)?.focus();
  };

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (busy) {
      return;
    }

    setError(undefined);
    const pending =
      mode === 'login'
        ? session.login({ email, password })
        : session.register({ email, username, password });

    void pending.catch((cause: unknown) => {
      setError({ copy: failureCopy(cause), kind: failureKind(cause) });
    });
  };

  return (
    <div className="lg-auth">
      <AuthStage />
      <section className="lg-auth__panel" aria-labelledby="auth-title">
        <div className="lg-auth__intro">
          <h1 className="lg-auth__title" id="auth-title">
            {mode === 'login' ? message('auth.titleLogin') : message('auth.titleRegister')}
          </h1>
          <p className="lg-auth__subtitle">{subtitle(mode, next)}</p>
          {snapshot.notice === 'deferred' && (
            <p className="lg-auth__notice" role="status" data-testid="auth-deferred">
              {message('auth.deferredNotice')}
            </p>
          )}
        </div>

        <div className="lg-auth__modes" role="tablist" aria-label={message('auth.modeGroupLabel')}>
          {MODES.map((tab) => (
            <button
              key={tab.mode}
              type="button"
              role="tab"
              id={`auth-tab-${tab.mode}`}
              className="lg-auth__mode"
              aria-selected={mode === tab.mode}
              aria-controls="auth-tabpanel"
              onKeyDown={onTabKeyDown}
              tabIndex={mode === tab.mode ? 0 : -1}
              onClick={() => switchMode(tab.mode)}
            >
              {tab.mode === 'login' ? message('auth.modeLogin') : message('auth.modeRegister')}
            </button>
          ))}
        </div>

        <div role="tabpanel" id="auth-tabpanel" aria-labelledby={`auth-tab-${mode}`}>
          <form className="lg-auth__form" onSubmit={submit}>
            <div className="lg-auth__field">
              <label className="lg-auth__label" htmlFor="auth-email">
                {message('auth.emailLabel')}
              </label>
              <input
                id="auth-email"
                ref={emailRef}
                className="lg-auth__input"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
              />
            </div>

            {mode === 'register' && (
              <div className="lg-auth__field">
                <label className="lg-auth__label" htmlFor="auth-username">
                  {message('auth.usernameLabel')}
                </label>
                <input
                  id="auth-username"
                  className="lg-auth__input"
                  type="text"
                  autoComplete="username"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                />
              </div>
            )}

            <div className="lg-auth__field">
              <label className="lg-auth__label" htmlFor="auth-password">
                {message('auth.passwordLabel')}
              </label>
              <div className="lg-auth__secret">
                <input
                  id="auth-password"
                  ref={passwordRef}
                  className="lg-auth__input"
                  type={showPassword ? 'text' : 'password'}
                  autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                />
                <button
                  type="button"
                  className="lg-auth__reveal"
                  aria-pressed={showPassword}
                  aria-controls="auth-password"
                  onClick={() => setShowPassword((value) => !value)}
                >
                  {showPassword ? message('auth.hidePassword') : message('auth.showPassword')}
                </button>
              </div>
            </div>

            {error !== undefined && (
              <div className="lg-auth__refusal">
                <p
                  className="lg-auth__error"
                  role="alert"
                  aria-live="polite"
                  data-testid="auth-error"
                >
                  {error.copy}
                </p>
                {error.kind === 'conflict' && (
                  <button
                    type="button"
                    className="lg-auth__inline-action"
                    data-testid="auth-conflict-action"
                    onClick={switchToSignIn}
                  >
                    {message('auth.conflictSignIn')}
                  </button>
                )}
              </div>
            )}

            <footer className="lg-auth__footer">
              <button type="submit" className="lg-auth__submit" disabled={busy} aria-busy={busy}>
                {mode === 'login' ? message('auth.submitLogin') : message('auth.submitRegister')}
              </button>
            </footer>
          </form>
        </div>
      </section>
    </div>
  );
}
