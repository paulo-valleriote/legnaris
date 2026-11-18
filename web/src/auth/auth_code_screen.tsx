import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import type { ClipboardEvent, FormEvent } from 'react';
import { failureCopy } from '../api/error_mapper';
import { useAuthSession } from '../app/providers';
import type { PendingCodeStep } from './auth_session';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Notice } from '../ui/notice';
import { AuthStage } from './auth_stage';
import './auth.css';

const CODE_LENGTH = 6;

function normaliseCode(raw: string): string {
  return raw.replace(/\D/g, '').slice(0, CODE_LENGTH);
}

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

  if (pending !== null && seed.pending !== pending) {
    setSeed({ pending, at: tick });
  }

  useEffect(() => {
    fieldRef.current?.focus();
  }, []);

  useEffect(() => {
    const timer = setInterval(() => {
      setTick((value) => value + 1);
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  if (pending === null) {
    return (
      <div className="lg-auth">
        <AuthStage linked={false} />
        <section className="lg-auth__panel" aria-labelledby="auth-code-missing-title">
          <div className="lg-auth__intro">
            <h1 className="lg-auth__title" id="auth-code-missing-title">
              {message('auth.codeMissingTitle')}
            </h1>
            <p className="lg-auth__subtitle">{message('auth.codeMissingBody')}</p>
          </div>
          <button
            type="button"
            className="lg-auth__back"
            data-testid="code-missing-back"
            onClick={() => session.cancelCodeStep()}
          >
            {message('auth.codeBackLogin')}
          </button>
        </section>
      </div>
    );
  }

  const elapsed = tick - seed.at;
  const resendRemaining = Math.max(0, resendAfter - elapsed);
  const lifetimeRemaining = Math.max(0, expiresIn - elapsed);
  const minutes = Math.max(1, Math.floor(pending.expiresIn / 60));
  const expired = lifetimeRemaining === 0;
  const describedBy = [
    'auth-code-instruction',
    expired ? 'code-expired' : undefined,
    refusal === undefined ? undefined : 'auth-code-refusal',
  ]
    .filter((value): value is string => value !== undefined)
    .join(' ');
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
            <Notice tone="danger" surface="stage" id="code-expired">
              {message('auth.codeExpired')}
            </Notice>
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
                maxLength={CODE_LENGTH}
                pattern="[0-9]*"
                value={code}
                onChange={(event) => setCode(normaliseCode(event.target.value))}
                onPaste={(event: ClipboardEvent<HTMLInputElement>) => {
                  event.preventDefault();
                  setCode(normaliseCode(event.clipboardData.getData('text')));
                }}
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
            <Button
              type="submit"
              className="lg-auth__submit"
              fullWidth
              loading={submitting}
              label={submitting ? message('auth.codeVerifying') : message('auth.codeSubmit')}
            />
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
