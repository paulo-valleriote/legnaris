/**
 * The account destination (LGN-195): the caller's Plan and stored LLM consent
 * (ADR 0008), with the one destructive action that withdraws the consent.
 *
 * The plan is read through the shared plan query, so the state renders as one
 * of loading, the mapped failure with a single Retry, or the two rows. Revoking
 * is guarded by one confirmation dialog stating the consequence: its safe action
 * carries focus, Escape and the overlay barrier cancel, and the destructive action
 * is a real button, disabled while its request is in flight so a double submit
 * sends one `DELETE`.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy } from '../api/error_mapper';
import type { PlanUsage } from '../api/models/plan';
import { useApiClient } from '../app/providers';
import { PLAN_QUERY_KEY, usePlanUsage } from '../catalog/queries';
import { message, messageWith } from '../l10n/strings';
import './account.css';
import '../catalog/catalog.css';

/** Formats a stored consent timestamp as a calendar date for the account rows. */
function formatConsentDate(consentAt: string): string {
  return new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(new Date(consentAt));
}

/** The plan's display name; a plan the client does not name is shown as its identifier. */
function planName(plan: string): string {
  if (plan === 'free') {
    return message('account.planFree');
  }
  if (plan === 'paid') {
    return message('account.planPaid');
  }
  return messageWith('account.planOther', { plan });
}

interface RevokeDialogProps {
  readonly pending: boolean;
  readonly failure: string | null;
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}

/**
 * The one revoke confirmation (LGN-195): a labelled, modal dialog that names the
 * consequence. Cancel is focused on open; Escape and the scrim cancel unless a
 * request is in flight.
 */
function RevokeDialog({ pending, failure, onCancel, onConfirm }: RevokeDialogProps) {
  const keepRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    keepRef.current?.focus();
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onCancel();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onCancel]);

  return (
    <div
      className="lg-overlay"
      role="presentation"
      data-testid="account-revoke-overlay"
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onCancel();
        }
      }}
    >
      <div
        className="lg-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="account-revoke-title"
        aria-describedby="account-revoke-message"
      >
        <div className="lg-dialog__header">
          <h2 className="lg-dialog__title" id="account-revoke-title">
            {message('account.revokeTitle')}
          </h2>
        </div>
        <p className="lg-dialog__message" id="account-revoke-message">
          {message('account.revokeMessage')}
        </p>
        {failure === null ? null : (
          <p className="lg-account__error" role="alert">
            {failure}
          </p>
        )}
        <div className="lg-dialog__actions">
          <button
            type="button"
            className="lg-button"
            ref={keepRef}
            disabled={pending}
            onClick={onCancel}
          >
            {message('account.revokeKeep')}
          </button>
          <button
            type="button"
            className="lg-button lg-button--danger"
            disabled={pending}
            aria-busy={pending}
            onClick={onConfirm}
          >
            {message('account.revokeConfirm')}
          </button>
        </div>
      </div>
    </div>
  );
}

interface ConsentSectionProps {
  readonly usage: PlanUsage;
}

/**
 * The stored consent: its date and disclosure version, or the designed "none"
 * line with no revoke action. Revoking refreshes the plan so the row settles on
 * the server's answer, and the confirmation names the result once it lands.
 */
function ConsentSection({ usage }: ConsentSectionProps) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [revoked, setRevoked] = useState(false);

  const revoke = useMutation({
    mutationFn: () => client.revokeLlmConsent(),
    onSuccess: async () => {
      setDialogOpen(false);
      setRevoked(true);
      await queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
    },
  });

  const cancel = useCallback(() => {
    if (!revoke.isPending) {
      setDialogOpen(false);
    }
  }, [revoke.isPending]);

  const openDialog = () => {
    revoke.reset();
    setRevoked(false);
    setDialogOpen(true);
  };

  if (usage.consentAt === null) {
    return (
      <div className="lg-account__row">
        <h2 className="lg-account__label">{message('account.consentHeading')}</h2>
        <p className="lg-account__value" data-testid="account-consent-none">
          {message('account.consentNone')}
        </p>
        {revoked ? (
          <p className="lg-account__notice" role="status">
            {message('account.revoked')}
          </p>
        ) : null}
      </div>
    );
  }

  return (
    <div className="lg-account__row">
      <h2 className="lg-account__label">{message('account.consentHeading')}</h2>
      <p className="lg-account__value" data-testid="account-consent">
        {messageWith('account.consentGiven', { date: formatConsentDate(usage.consentAt) })}
        {usage.consentVersion === null
          ? null
          : ` · ${messageWith('account.consentVersion', { version: usage.consentVersion })}`}
      </p>
      <div>
        <button type="button" className="lg-button lg-button--danger" onClick={openDialog}>
          {message('account.revoke')}
        </button>
      </div>
      {revoked ? (
        <p className="lg-account__notice" role="status">
          {message('account.revoked')}
        </p>
      ) : null}
      {dialogOpen ? (
        <RevokeDialog
          pending={revoke.isPending}
          failure={revoke.isError ? failureCopy(revoke.error) : null}
          onCancel={cancel}
          onConfirm={() => revoke.mutate()}
        />
      ) : null}
    </div>
  );
}

/**
 * The account screen (LGN-195): loading, the mapped failure with a single Retry,
 * or the Plan and consent rows.
 */
export function AccountScreen() {
  const plan = usePlanUsage();

  return (
    <section className="lg-account" aria-labelledby="account-title">
      <h1 className="lg-account__title" id="account-title">
        {message('account.title')}
      </h1>

      {plan.isPending ? <p className="lg-account__message">{message('account.loading')}</p> : null}

      {plan.isError ? (
        <div className="lg-account__failure" data-testid="account-failure">
          <p className="lg-account__message">{failureCopy(plan.error)}</p>
          <button
            type="button"
            className="lg-button"
            onClick={() => {
              void plan.refetch();
            }}
          >
            {message('account.retry')}
          </button>
        </div>
      ) : null}

      {plan.data === undefined ? null : (
        <>
          <div className="lg-account__row">
            <h2 className="lg-account__label">{message('account.planHeading')}</h2>
            <p className="lg-account__value" data-testid="account-plan">
              {planName(plan.data.plan)}
            </p>
          </div>
          <ConsentSection usage={plan.data} />
        </>
      )}
    </section>
  );
}
