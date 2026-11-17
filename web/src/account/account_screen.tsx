import { useCallback, useEffect, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy } from '../api/error_mapper';
import { SHEET_KINDS, type PlanUsage, type SheetKind } from '../api/models/plan';
import { useApiClient } from '../app/providers';
import { PLAN_QUERY_KEY, usePlanUsage } from '../catalog/queries';
import { PrivacyDialog } from '../legal/privacy_dialog';
import { planName } from '../plans/plan_view';
import { message, messageWith, type MessageKey } from '../l10n/strings';
import './account.css';
import '../catalog/catalog.css';

const USAGE_LABELS: Readonly<Record<SheetKind, MessageKey>> = {
  character: 'account.usageCharacter',
  npc: 'account.usageNpc',
  enemy: 'account.usageEnemy',
};

function formatConsentDate(consentAt: string): string {
  return new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(new Date(consentAt));
}

interface RevokeDialogProps {
  readonly pending: boolean;
  readonly failure: string | null;
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}

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
        <div className="lg-account__content">
          <p className="lg-account__value" data-testid="account-consent-none">
            {message('account.consentNone')}
          </p>
          {revoked ? (
            <p className="lg-account__notice" role="status">
              {message('account.revoked')}
            </p>
          ) : null}
        </div>
      </div>
    );
  }

  return (
    <div className="lg-account__row">
      <h2 className="lg-account__label">{message('account.consentHeading')}</h2>
      <div className="lg-account__content">
        <p className="lg-account__value" data-testid="account-consent">
          {messageWith('account.consentGiven', { date: formatConsentDate(usage.consentAt) })}
          {usage.consentVersion === null
            ? null
            : ` · ${messageWith('account.consentVersion', { version: usage.consentVersion })}`}
        </p>
        <button type="button" className="lg-button lg-button--danger" onClick={openDialog}>
          {message('account.revoke')}
        </button>
        {revoked ? (
          <p className="lg-account__notice" role="status">
            {message('account.revoked')}
          </p>
        ) : null}
      </div>
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

function PrivacySection() {
  const [open, setOpen] = useState(false);
  const close = useCallback(() => setOpen(false), []);

  return (
    <div className="lg-account__row">
      <h2 className="lg-account__label">{message('account.privacyHeading')}</h2>
      <div className="lg-account__content">
        <button
          type="button"
          className="lg-button"
          aria-haspopup="dialog"
          data-testid="account-privacy-open"
          onClick={() => setOpen(true)}
        >
          {message('account.privacyOpen')}
        </button>
      </div>
      {open ? <PrivacyDialog onClose={close} /> : null}
    </div>
  );
}

function SheetUsage({ usage }: { readonly usage: PlanUsage }) {
  const entries = SHEET_KINDS.flatMap((kind) => {
    const entry = usage.sheets[kind];
    return entry === undefined ? [] : [{ kind, entry }];
  });

  if (entries.length === 0) {
    return null;
  }

  return (
    <dl className="lg-account__usage" data-testid="account-usage">
      {entries.map(({ kind, entry }) => (
        <div className="lg-account__usage-entry" key={kind}>
          <dt>{message(USAGE_LABELS[kind])}</dt>
          <dd
            className={
              entry.used >= entry.limit
                ? 'lg-account__usage-count lg-account__usage-count--full'
                : 'lg-account__usage-count'
            }
          >
            {messageWith('account.usageCount', { used: entry.used, limit: entry.limit })}
          </dd>
        </div>
      ))}
    </dl>
  );
}

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

      <div className="lg-account__rows">
        {plan.data === undefined ? null : (
          <>
            <div className="lg-account__row">
              <h2 className="lg-account__label">{message('account.planHeading')}</h2>
              <div className="lg-account__content">
                <p className="lg-account__value lg-account__value--lead" data-testid="account-plan">
                  {planName(plan.data.plan)}
                </p>
                <SheetUsage usage={plan.data} />
              </div>
            </div>
            <ConsentSection usage={plan.data} />
          </>
        )}
        <PrivacySection />
      </div>
    </section>
  );
}
