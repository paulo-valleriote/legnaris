/**
 * The code surface (WCS-03, WCS-05, WCS-13, WCS-16, WCS-19 … WCS-25).
 *
 * The state of the single auth surface while a code step is pending: it names
 * the address the code was sent to and how long the code remains valid, takes
 * the six digits over one editable field presented as six single-digit
 * positions, shows the mapped refusal against that field and offers one way
 * back to the credential form.
 *
 * It is a state of the auth surface and not a route: the handle lives in the
 * session, so a deep link would arrive with nothing to verify (R5). The field
 * owns the focus while the surface is up, the refusal keeps it there, and the
 * countdown the resend row carries is static text outside any live region
 * (WCS-22, WCS-23).
 */

import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import type { FormEvent } from 'react';
import { failureCopy } from '../api/error_mapper';
import { useAuthSession } from '../app/providers';
import { message, messageWith } from '../l10n/strings';
import { BrandMark } from '../shell/brand_mark';
import './auth.css';

/** The digits the code carries (WCS-24). */
const CODE_LENGTH = 6;

/** The code surface, rendered in place of the credential form (WCS-02). */
export function AuthCodeScreen() {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const pending = snapshot.pending;
  const [code, setCode] = useState('');
  const [refusal, setRefusal] = useState<string | undefined>(undefined);
  const [submitting, setSubmitting] = useState(false);
  const fieldRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fieldRef.current?.focus();
  }, []);

  if (pending === null) {
    return null;
  }

  const minutes = Math.max(1, Math.floor(pending.expiresIn / 60));
  const describedBy =
    refusal === undefined ? 'auth-code-instruction' : 'auth-code-instruction auth-code-refusal';

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (submitting) {
      return;
    }

    setRefusal(undefined);
    setSubmitting(true);

    void session
      .submitCode(code)
      .catch((cause: unknown) => {
        setRefusal(failureCopy(cause));
        fieldRef.current?.focus();
      })
      .finally(() => {
        setSubmitting(false);
      });
  };

  const back = () => {
    session.cancelCodeStep();
  };

  return (
    <div className="lg-auth">
      <section className="lg-auth__panel" aria-labelledby="auth-code-title">
        <header className="lg-auth__brand">
          <span className="lg-auth__mark" aria-hidden="true">
            <BrandMark />
          </span>
          <span className="lg-auth__wordmark">{message('app.name')}</span>
          <span className="lg-auth__tagline">{message('nav.tagline')}</span>
        </header>

        <p className="lg-auth__origin" data-testid="code-origin">
          {messageWith('auth.codeOrigin', {
            mode:
              pending.origin === 'register'
                ? message('auth.accountModeNew')
                : message('auth.accountModeExisting'),
          })}
        </p>

        <div className="lg-auth__intro">
          <h1 className="lg-auth__title" id="auth-code-title">
            {pending.origin === 'register'
              ? message('auth.codeTitleRegister')
              : message('auth.codeTitleLogin')}
          </h1>
          <p
            className="lg-auth__subtitle"
            id="auth-code-instruction"
            data-testid="code-instruction"
          >
            {messageWith('auth.codeInstruction', { email: pending.email, minutes })}
          </p>
        </div>

        <form className="lg-auth__form" onSubmit={submit}>
          <div className="lg-auth__field">
            <label className="lg-auth__label" htmlFor="auth-code">
              {message('auth.codeLabel')}
            </label>
            <div className="lg-auth__code">
              <input
                id="auth-code"
                ref={fieldRef}
                className="lg-auth__input lg-auth__code-input"
                type="text"
                inputMode="numeric"
                autoComplete="one-time-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                aria-describedby={describedBy}
                aria-invalid={refusal !== undefined}
              />
              <div className="lg-auth__code-slots" aria-hidden="true">
                {Array.from({ length: CODE_LENGTH }, (_unused, index) => (
                  <span
                    key={index}
                    className="lg-auth__code-slot"
                    data-testid={`code-slot-${index}`}
                  >
                    {code.charAt(index)}
                  </span>
                ))}
              </div>
            </div>
          </div>

          {refusal !== undefined && (
            <p
              className="lg-auth__error"
              role="alert"
              id="auth-code-refusal"
              data-testid="code-refusal"
            >
              {refusal}
            </p>
          )}

          <footer className="lg-auth__footer">
            <button
              type="submit"
              className="lg-button lg-button--primary lg-button--md lg-auth__submit"
              disabled={submitting}
              aria-busy={submitting}
            >
              {submitting ? message('auth.codeVerifying') : message('auth.codeSubmit')}
            </button>
          </footer>
        </form>

        <button type="button" className="lg-auth__back" data-testid="code-back" onClick={back}>
          {pending.origin === 'register'
            ? message('auth.codeBackRegister')
            : message('auth.codeBackLogin')}
        </button>
      </section>
    </div>
  );
}
