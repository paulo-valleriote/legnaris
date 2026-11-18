import { useCallback, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy } from '../api/error_mapper';
import { SHEET_KINDS, type PlanUsage, type SheetKind } from '../api/models/plan';
import { useApiClient } from '../app/providers';
import { PLAN_QUERY_KEY, usePlanUsage } from '../catalog/queries';
import { PrivacyDialog } from '../legal/privacy_dialog';
import { planName } from '../plans/plan_view';
import { message, messageWith, type MessageKey } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { PageHeader } from '../ui/page_header';
import { ErrorState, LoadingState } from '../ui/state';
import './account.css';

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

  return (
    <Dialog
      open
      tone="danger"
      title={message('account.revokeTitle')}
      description={message('account.revokeMessage')}
      onClose={onCancel}
      initialFocusRef={keepRef}
      error={failure ?? undefined}
      actions={
        <>
          <Button
            ref={keepRef}
            variant="secondary"
            label={message('account.revokeKeep')}
            disabled={pending}
            onClick={onCancel}
          />
          <Button
            variant="danger"
            label={message('account.revokeConfirm')}
            loading={pending}
            onClick={onConfirm}
          />
        </>
      }
    />
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
        <dt className="lg-account__label">{message('account.consentHeading')}</dt>
        <dd className="lg-account__content">
          <p className="lg-account__value" data-testid="account-consent-none">
            {message('account.consentNone')}
          </p>
          {revoked ? (
            <p className="lg-account__notice" role="status">
              {message('account.revoked')}
            </p>
          ) : null}
        </dd>
      </div>
    );
  }

  return (
    <div className="lg-account__row">
      <dt className="lg-account__label">{message('account.consentHeading')}</dt>
      <dd className="lg-account__content">
        <p className="lg-account__value" data-testid="account-consent">
          {messageWith('account.consentGiven', { date: formatConsentDate(usage.consentAt) })}
          {usage.consentVersion === null
            ? null
            : ` · ${messageWith('account.consentVersion', { version: usage.consentVersion })}`}
        </p>
        <Button variant="danger" label={message('account.revoke')} onClick={openDialog} />
        {revoked ? (
          <p className="lg-account__notice" role="status">
            {message('account.revoked')}
          </p>
        ) : null}
      </dd>
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
      <dt className="lg-account__label">{message('account.privacyHeading')}</dt>
      <dd className="lg-account__content">
        <Button
          variant="secondary"
          label={message('account.privacyOpen')}
          ariaHasPopup="dialog"
          onClick={() => setOpen(true)}
        />
      </dd>
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
      {entries.map(({ kind, entry }) => {
        const full = entry.used >= entry.limit;
        return (
          <div className="lg-account__usage-entry" key={kind}>
            <dt>{message(USAGE_LABELS[kind])}</dt>
            <dd
              className={
                full
                  ? 'lg-account__usage-count lg-account__usage-count--full'
                  : 'lg-account__usage-count'
              }
            >
              {messageWith('account.usageCount', { used: entry.used, limit: entry.limit })}
              {full ? (
                <>
                  {' '}
                  <span className="lg-account__quota-full">{message('account.quotaFull')}</span>
                </>
              ) : null}
            </dd>
          </div>
        );
      })}
    </dl>
  );
}

export function AccountScreen() {
  const plan = usePlanUsage();

  return (
    <section className="lg-account">
      <PageHeader title={message('account.title')} />

      {plan.isPending ? <LoadingState label={message('account.loading')} /> : null}

      {plan.isError ? (
        <ErrorState
          message={message('plan.loadFailed')}
          onRetry={() => {
            void plan.refetch();
          }}
        />
      ) : null}

      <dl className="lg-account__rows">
        {plan.data === undefined ? null : (
          <>
            <div className="lg-account__row">
              <dt className="lg-account__label">{message('account.planHeading')}</dt>
              <dd className="lg-account__content">
                <p className="lg-account__value lg-account__value--lead" data-testid="account-plan">
                  {planName(plan.data.plan)}
                </p>
                <SheetUsage usage={plan.data} />
              </dd>
            </div>
            <ConsentSection usage={plan.data} />
          </>
        )}
        <PrivacySection />
      </dl>
    </section>
  );
}
