/**
 * The auth surface (WAS-07, WAS-09, WAS-10, R9).
 *
 * One place offers sign-in and registration through a mode switch, opening with
 * the brand lockup and a mode-specific title so the first screen of the product
 * carries its identity. The single primary action submits through the session,
 * is disabled and shows progress while a submit is in flight, and the password
 * is a hidden-content control. A failed submit keeps the typed values and
 * renders the mapped failure copy resolved from the catalog — never the server
 * message, so the copy never reveals whether an email exists (WAS-10). Switching
 * mode clears a stale error but keeps the values.
 */

import { useState, useSyncExternalStore } from 'react';
import type { FormEvent } from 'react';
import { failureCopy } from '../api/error_mapper';
import { useAuthSession } from '../app/providers';
import { message } from '../l10n/strings';
import { BrandMark } from '../shell/brand_mark';
import './auth.css';

type AuthMode = 'login' | 'register';

/** The single sign-in / registration surface (WAS-09). */
export function AuthScreen() {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const [mode, setMode] = useState<AuthMode>('login');
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | undefined>(undefined);
  const busy = snapshot.state === 'authenticating';

  const switchMode = (next: AuthMode) => {
    setMode(next);
    setError(undefined);
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
      setError(failureCopy(cause));
    });
  };

  return (
    <div className="lg-auth">
      <section className="lg-auth__panel" aria-labelledby="auth-title">
        <header className="lg-auth__brand">
          <span className="lg-auth__mark" aria-hidden="true">
            <BrandMark />
          </span>
          <span className="lg-auth__wordmark">{message('app.name')}</span>
          <span className="lg-auth__tagline">{message('nav.tagline')}</span>
        </header>

        <div className="lg-auth__intro">
          <h1 className="lg-auth__title" id="auth-title">
            {mode === 'login' ? message('auth.titleLogin') : message('auth.titleRegister')}
          </h1>
          <p className="lg-auth__subtitle">
            {mode === 'login' ? message('auth.subtitleLogin') : message('auth.subtitleRegister')}
          </p>
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
              className="lg-auth__input"
              type="password"
              autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>

          {error !== undefined && (
            <p className="lg-auth__error" role="alert" aria-live="polite" data-testid="auth-error">
              {error}
            </p>
          )}

          <footer className="lg-auth__footer">
            <button
              type="submit"
              className="lg-button lg-button--primary lg-button--md lg-auth__submit"
              disabled={busy}
              aria-busy={busy}
            >
              {mode === 'login' ? message('auth.submitLogin') : message('auth.submitRegister')}
            </button>
          </footer>
        </form>
      </section>
    </div>
  );
}
