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
import type { FormEvent } from 'react';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { FailureKind } from '../api/errors';
import { useAuthSession } from '../app/providers';
import { message } from '../l10n/strings';
import type { RestoredForm } from './auth_session';
import { AuthStage } from './auth_stage';
import './auth.css';

export type AuthMode = 'login' | 'register';

export type { RestoredForm };

interface AuthScreenProps {
  readonly restore?: RestoredForm | null;
  /** The mode the surface opens in when no abandoned code step dictates one. */
  readonly initialMode?: AuthMode;
}

/** The single sign-in / registration surface (WAS-09). */
export function AuthScreen({ restore, initialMode = 'login' }: AuthScreenProps) {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const [mode, setMode] = useState<AuthMode>(restore?.origin ?? initialMode);
  const [email, setEmail] = useState(restore?.email ?? '');
  const [username, setUsername] = useState(restore?.username ?? '');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<
    { readonly copy: string; readonly kind: FailureKind } | undefined
  >(undefined);
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
          <p className="lg-auth__subtitle">
            {mode === 'login' ? message('auth.subtitleLogin') : message('auth.subtitleRegister')}
          </p>
          {snapshot.notice === 'deferred' && (
            <p className="lg-auth__notice" role="status" data-testid="auth-deferred">
              {message('auth.deferredNotice')}
            </p>
          )}
        </div>

        <div className="lg-auth__modes" role="group" aria-label={message('auth.modeGroupLabel')}>
          <button
            type="button"
            className="lg-auth__mode"
            aria-pressed={mode === 'login'}
            onClick={() => switchMode('login')}
          >
            {message('auth.modeLogin')}
          </button>
          <button
            type="button"
            className="lg-auth__mode"
            aria-pressed={mode === 'register'}
            onClick={() => switchMode('register')}
          >
            {message('auth.modeRegister')}
          </button>
        </div>

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
            <input
              id="auth-password"
              ref={passwordRef}
              className="lg-auth__input"
              type="password"
              autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
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
      </section>
    </div>
  );
}
