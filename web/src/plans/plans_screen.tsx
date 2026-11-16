import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { PlanUsage } from '../api/models/plan';
import { redirectTo } from '../billing/redirect';
import { useApiClient } from '../app/providers';
import { PLAN_QUERY_KEY, usePlanUsage } from '../catalog/queries';
import { message, messageWith } from '../l10n/strings';
import { Dialog } from '../ui/dialog';
import { planName, refundPollingInterval } from './plan_view';
import './plans.css';

function RefundSection({ usage, onRequest }: { usage: PlanUsage; onRequest: () => void }) {
  const { refundEligible, refundState } = usage.billing;

  if (!refundEligible && refundState === null) {
    return null;
  }

  return (
    <section className="lg-plans__refund" aria-labelledby="plans-refund-heading">
      <h2 className="lg-plans__refund-title" id="plans-refund-heading">
        {message('plans.refundHeading')}
      </h2>
      {refundState === 'requested' ? (
        <p className="lg-plans__notice" role="status">
          {message('plans.refundRequested')}
        </p>
      ) : null}
      {refundState === 'refunded' || refundState === 'canceled' ? (
        <p className="lg-plans__notice" role="status">
          {message('plans.refundCompleted')}
        </p>
      ) : null}
      {refundState === 'failed' ? (
        <p className="lg-plans__error" role="alert">
          {message('plans.refundFailed')}
        </p>
      ) : null}
      {refundEligible ? (
        <>
          <p className="lg-plans__text">{message('plans.refundNote')}</p>
          <div>
            <button type="button" className="lg-button lg-button--danger" onClick={onRequest}>
              {message('plans.refundAction')}
            </button>
          </div>
        </>
      ) : null}
    </section>
  );
}

function PlansBody({ usage }: { usage: PlanUsage }) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const [refundOpen, setRefundOpen] = useState(false);

  const isFree = usage.plan === 'free';
  const isPaid = usage.plan === 'paid';
  const hasSubscription = usage.billing.subscriptionStatus !== null;

  const reloadPlan = () => queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });

  const checkout = useMutation({
    mutationFn: () => client.createCheckout(),
    onSuccess: (url) => {
      redirectTo(url);
    },
    onError: (error) => {
      if (failureKind(error) === 'alreadySubscribed') {
        void reloadPlan();
      }
    },
  });

  const portal = useMutation({
    mutationFn: () => client.openPortal(),
    onSuccess: (url) => {
      redirectTo(url);
    },
    onError: (error) => {
      if (failureKind(error) === 'noSubscription') {
        void reloadPlan();
      }
    },
  });

  const refund = useMutation({
    mutationFn: () => client.requestRefund(),
    onSuccess: async () => {
      setRefundOpen(false);
      await reloadPlan();
    },
  });

  const closeRefund = () => {
    if (!refund.isPending) {
      refund.reset();
      setRefundOpen(false);
    }
  };

  const opening = checkout.isPending || checkout.isSuccess;
  const showActions = !isPaid || hasSubscription;

  const currentBadge = (
    <p className="lg-plans__badge" data-testid="plans-current">
      {messageWith('plans.current', { plan: planName(usage.plan) })}
    </p>
  );

  return (
    <>
      {isFree || isPaid ? null : (
        <p className="lg-plans__badge lg-plans__badge--standalone" data-testid="plans-current">
          {messageWith('plans.current', { plan: planName(usage.plan) })}
        </p>
      )}

      <div className="lg-plans__grid">
        <article
          className={`lg-plans__plan${isFree ? ' lg-plans__plan--current' : ''}`}
          aria-labelledby="plans-free-heading"
        >
          <header className="lg-plans__plan-head">
            <h2 className="lg-plans__label" id="plans-free-heading">
              {planName('free')}
            </h2>
            {isFree ? currentBadge : null}
          </header>
          <p className="lg-plans__text">{message('plans.freeDescription')}</p>
        </article>

        <article
          className={`lg-plans__plan${isPaid ? ' lg-plans__plan--current' : ''}`}
          aria-labelledby="plans-paid-heading"
        >
          <header className="lg-plans__plan-head">
            <h2 className="lg-plans__label" id="plans-paid-heading">
              {planName('paid')}
            </h2>
            {isPaid ? currentBadge : null}
          </header>

          <div className="lg-plans__pricing">
            <p className="lg-plans__price">
              <span className="lg-plans__price-amount">{message('plans.priceAmount')}</span>
              <span className="lg-plans__price-period">{message('plans.pricePeriod')}</span>
            </p>
            <p className="lg-plans__price-note">{message('plans.priceNote')}</p>
          </div>

          <p className="lg-plans__text">{message('plans.paidDescription')}</p>

          {usage.billing.cancelAtPeriodEnd ? (
            <p className="lg-plans__notice">{message('plans.cancelAtPeriodEnd')}</p>
          ) : null}
          {usage.billing.currentPeriodEnd === null ? null : (
            <p className="lg-plans__notice">
              {messageWith('plans.periodEnd', {
                date: new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(
                  new Date(usage.billing.currentPeriodEnd),
                ),
              })}
            </p>
          )}
          {isPaid || hasSubscription ? null : (
            <p className="lg-plans__notice">{message('plans.neverSubscribed')}</p>
          )}

          {showActions ? (
            <div className="lg-plans__plan-actions">
              {isPaid ? null : (
                <button
                  type="button"
                  className="lg-button lg-button--primary"
                  disabled={opening}
                  aria-busy={opening}
                  onClick={() => checkout.mutate()}
                >
                  {opening ? message('plans.subscribing') : message('plans.subscribe')}
                </button>
              )}
              {hasSubscription ? (
                <button
                  type="button"
                  className="lg-button"
                  disabled={portal.isPending}
                  aria-busy={portal.isPending}
                  onClick={() => portal.mutate()}
                >
                  {portal.isPending ? message('plans.managing') : message('plans.manage')}
                </button>
              ) : null}
            </div>
          ) : null}

          {checkout.isError ? (
            <p className="lg-plans__error" role="alert">
              {failureCopy(checkout.error)}
            </p>
          ) : null}
          {portal.isError ? (
            <p className="lg-plans__error" role="alert">
              {failureCopy(portal.error)}
            </p>
          ) : null}
        </article>
      </div>

      <RefundSection usage={usage} onRequest={() => setRefundOpen(true)} />

      <Dialog
        open={refundOpen}
        title={message('plans.refundTitle')}
        description={message('plans.refundMessage')}
        onClose={closeRefund}
      >
        {refund.isError ? (
          <p className="lg-plans__error" role="alert">
            {failureCopy(refund.error)}
          </p>
        ) : null}
        <div className="lg-plans__actions">
          <button
            type="button"
            className="lg-button"
            disabled={refund.isPending}
            onClick={closeRefund}
          >
            {message('plans.refundKeep')}
          </button>
          <button
            type="button"
            className="lg-button lg-button--danger"
            disabled={refund.isPending}
            aria-busy={refund.isPending}
            onClick={() => refund.mutate()}
          >
            {message('plans.refundConfirm')}
          </button>
        </div>
      </Dialog>
    </>
  );
}

export function PlansScreen() {
  const plan = usePlanUsage({
    refetchInterval: (query) => refundPollingInterval(query.state.data),
  });

  return (
    <section className="lg-plans" aria-labelledby="plans-title">
      <h1 className="lg-plans__title" id="plans-title">
        {message('plans.title')}
      </h1>

      {plan.isPending ? <p className="lg-plans__message">{message('plans.loading')}</p> : null}

      {plan.isError ? (
        <div className="lg-plans__failure">
          <p className="lg-plans__error" role="alert">
            {failureCopy(plan.error)}
          </p>
          <button
            type="button"
            className="lg-button"
            onClick={() => {
              void plan.refetch();
            }}
          >
            {message('plans.retry')}
          </button>
        </div>
      ) : null}

      {plan.data === undefined ? null : <PlansBody usage={plan.data} />}
    </section>
  );
}
