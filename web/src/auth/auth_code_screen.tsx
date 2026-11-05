/**
 * The code surface (WCS-03, WCS-05, WCS-07, WCS-09, WCS-13, WCS-16, WCS-19 … WCS-26).
 *
 * The state of the single auth surface while a code step is pending: it names
 * the address the code was sent to and how long the code remains valid, takes
 * the six digits over one editable field presented as six single-digit
 * positions, shows the mapped refusal against that field, offers the resend
 * with an honest countdown and one way back to the credential form.
 *
 * It is a state of the auth surface and not a route: the handle lives in the
 * session, so a deep link would arrive with nothing to verify (R5). The field
 * owns the focus while the surface is up, the refusal keeps it there, and the
 * countdown the resend row carries is static text outside any live region
 * (WCS-22, WCS-23). One interval drives both the resend window and the stated
 * lifetime, so at most one timer runs (WCS-25).
 */

import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import type { FormEvent } from 'react';
import { failureCopy } from '../api/error_mapper';
import { useAuthSession } from '../app/providers';
import type { PendingCodeStep } from './auth_session';
import { message, messageWith } from '../l10n/strings';
import { AuthStage } from './auth_stage';
import './auth.css';

/** The digits the code carries (WCS-24). */
const CODE_LENGTH = 6;

/** The code surface, rendered in place of the credential form (WCS-02). */
export function AuthCodeScreen() {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const pending = snapshot.pending;
  const resendAfter = pending?.resendAfter ?? 0;
  const expiresIn = pending?.expiresIn ?? 0;
  const [code, setCode] = useState('');
  const [refusal, setRefusal] = useState<string | undefined>(undefined);
  const [confirmation, setConfirmation] = useState<string | undefined>(undefined);
  const [focused, setFocused] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [resending, setResending] = useState(false);
  const [tick, setTick] = useState(0);
  const [seed, setSeed] = useState<{
    readonly pending: PendingCodeStep | null;
    readonly at: number;
  }>({
    pending: null,
    at: 0,
  });
  const fieldRef = useRef<HTMLInputElement>(null);

  // A recorded resend answers with a new window, so the pending step is a new
  // projection; the countdown is re-seeded from it (WCS-08, WCS-20).
  if (pending !== null && seed.pending !== pending) {
    setSeed({ pending, at: tick });
  }

  useEffect(() => {
    fieldRef.current?.focus();
  }, []);

  // One interval drives both countdowns, so at most one timer runs (WCS-25, R9).
  useEffect(() => {
    const timer = setInterval(() => {
      setTick((value) => value + 1);
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  if (pending === null) {
    return null;
  }

  const elapsed = tick - seed.at;
  const resendRemaining = Math.max(0, resendAfter - elapsed);
  const lifetimeRemaining = Math.max(0, expiresIn - elapsed);
  const minutes = Math.max(1, Math.floor(pending.expiresIn / 60));
  const describedBy =
    refusal === undefined ? 'auth-code-instruction' : 'auth-code-instruction auth-code-refusal';
  const expired = lifetimeRemaining === 0;
  const resendUnavailable = resendRemaining > 0 || resending || submitting;
  const status =
    confirmation ?? (resendRemaining === 0 ? message('auth.codeResendReady') : undefined);

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (submitting) {
      return;
    }

    setRefusal(undefined);
    setConfirmation(undefined);
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

  const resend = () => {
    if (resendUnavailable) {
      return;
    }

    setResending(true);
    setRefusal(undefined);

    void session
      .resendCode()
      .then(() => {
        setConfirmation(message('auth.codeResendSent'));
      })
      .catch((cause: unknown) => {
        // A refused send re-arms the action for a whole window, so the person
        // is not offered the same refused send again immediately (WCS-09).
        setSeed({ pending, at: tick });
        setRefusal(failureCopy(cause));
      })
      .finally(() => {
        setResending(false);
      });
  };

  const back = () => {
    session.cancelCodeStep();
  };

  return (
    <div className="lg-auth">
      <AuthStage linked={false} />
      <section className="lg-auth__panel" aria-labelledby="auth-code-title">
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
            {expired
              ? messageWith('auth.codeInstructionExpired', { email: pending.email })
              : messageWith('auth.codeInstruction', { email: pending.email, minutes })}
          </p>
          {expired && (
            <p className="lg-auth__notice" data-testid="code-expired">
              {message('auth.codeExpired')}
            </p>
          )}
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
                onFocus={() => setFocused(true)}
                onBlur={() => setFocused(false)}
                aria-describedby={describedBy}
                aria-invalid={refusal !== undefined}
              />
              <div className="lg-auth__code-slots" aria-hidden="true">
                {Array.from({ length: CODE_LENGTH }, (_unused, index) => (
                  <span
                    key={index}
                    className="lg-auth__code-slot"
                    data-testid={`code-slot-${index}`}
                    data-active={focused && index === Math.min(code.length, CODE_LENGTH - 1)}
                    data-invalid={refusal !== undefined}
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
              className="lg-auth__submit"
              disabled={submitting}
              aria-busy={submitting}
            >
              {submitting ? message('auth.codeVerifying') : message('auth.codeSubmit')}
            </button>
          </footer>
        </form>

        <div className="lg-auth__resend" data-testid="code-resend-row">
          <p className="lg-auth__hint">{message('auth.codeResendHint')}</p>
          <button
            type="button"
            className="lg-auth__resend-action"
            aria-disabled={resendUnavailable}
            aria-describedby={resendRemaining > 0 ? 'auth-code-countdown' : undefined}
            data-testid="code-resend"
            onClick={resend}
          >
            {message('auth.codeResend')}
          </button>
          {resendRemaining > 0 && (
            <p className="lg-auth__countdown" id="auth-code-countdown" data-testid="code-countdown">
              {messageWith('auth.codeResendIn', { seconds: resendRemaining })}
            </p>
          )}
          <p className="lg-auth__status" role="status" data-testid="code-status">
            {status}
          </p>
        </div>

        <button type="button" className="lg-auth__back" data-testid="code-back" onClick={back}>
          {pending.origin === 'register'
            ? message('auth.codeBackRegister')
            : message('auth.codeBackLogin')}
        </button>
      </section>
    </div>
  );
}
