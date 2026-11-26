import { useCallback, useEffect, useState } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { PlanUsage } from '../api/models/plan';
import { redirectTo } from '../billing/redirect';
import { useApiClient } from '../app/providers';
import { PLAN_QUERY_KEY, usePlanUsage } from '../catalog/queries';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { Notice } from '../ui/notice';
import { PageHeader } from '../ui/page_header';
import { ErrorState, LoadingState } from '../ui/state';
import {
  CHECKOUT_CONFIRM_LIMIT_MS,
  checkoutPhase,
  checkoutPollingInterval,
  checkoutReturn,
  planName,
} from './plan_view';
import './plans.css';

function formatPeriodEnd(periodEnd: string): string {
  return new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(new Date(periodEnd));
}

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
        <Notice tone="info" live="polite">
          {message('plans.refundRequested')}
        </Notice>
      ) : null}
      {refundState === 'refunded' || refundState === 'canceled' ? (
        <Notice tone="success" live="polite">
          {message('plans.refundCompleted')}
        </Notice>
      ) : null}
      {refundState === 'failed' ? (
        <Notice tone="danger" live="polite">
          {message('plans.refundFailed')}
        </Notice>
      ) : null}
      {refundEligible ? (
        <>
          <p className="lg-plans__text">{message('plans.refundNote')}</p>
          <Button variant="danger" label={message('plans.refundAction')} onClick={onRequest} />
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
  const isComp = usage.billing.subscriptionProvider === 'comp';
  const hasManageableSubscription = hasSubscription && !isComp;

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

  const { isPending: refundPending, reset: resetRefund } = refund;

  const closeRefund = useCallback(() => {
    if (!refundPending) {
      resetRefund();
      setRefundOpen(false);
    }
  }, [refundPending, resetRefund]);

  const opening = checkout.isPending || checkout.isSuccess;
  const showActions = !isPaid || hasManageableSubscription;

  const currentBadge = (
    <p
      className={`lg-plans__badge${isFree || isPaid ? '' : ' lg-plans__badge--standalone'}`}
      data-testid="plans-current"
    >
      {messageWith('plans.current', { plan: planName(usage.plan) })}
    </p>
  );

  return (
    <>
      {isFree || isPaid ? null : currentBadge}

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

          <ul className="lg-plans__advantages" aria-label={message('plans.paidAdvantages')}>
            <li className="lg-plans__advantage">{message('plans.paidNoAds')}</li>
          </ul>

          {usage.billing.cancelAtPeriodEnd ? (
            <Notice tone="warning" live="polite">
              {message('plans.cancelAtPeriodEnd')}
            </Notice>
          ) : null}
          {usage.billing.currentPeriodEnd === null ? null : (
            <Notice tone="info" live="polite">
              {messageWith('plans.periodEnd', {
                date: formatPeriodEnd(usage.billing.currentPeriodEnd),
              })}
            </Notice>
          )}
          {isPaid || hasSubscription ? null : (
            <p className="lg-plans__text">{message('plans.neverSubscribed')}</p>
          )}

          {showActions ? (
            <div className="lg-plans__plan-actions">
              {isPaid ? null : (
                <Button
                  variant="primary"
                  label={message('plans.subscribe')}
                  loading={opening}
                  onClick={() => checkout.mutate()}
                />
              )}
              {hasManageableSubscription ? (
                <Button
                  variant="secondary"
                  label={message('plans.manage')}
                  loading={portal.isPending}
                  onClick={() => portal.mutate()}
                />
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
        tone="danger"
        title={message('plans.refundTitle')}
        description={message('plans.refundMessage')}
        error={refund.isError ? failureCopy(refund.error) : undefined}
        onClose={closeRefund}
        actions={
          <>
            <Button
              variant="secondary"
              label={message('plans.refundKeep')}
              disabled={refund.isPending}
              onClick={closeRefund}
            />
            <Button
              variant="danger"
              label={message('plans.refundConfirm')}
              loading={refund.isPending}
              onClick={() => refund.mutate()}
            />
          </>
        }
      ></Dialog>
    </>
  );
}

export function PlansScreen() {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [returned] = useState(() => checkoutReturn(searchParams.get('checkout')));
  const [timedOut, setTimedOut] = useState(false);

  const plan = usePlanUsage({
    refetchInterval: (query) =>
      checkoutPollingInterval(
        checkoutPhase(returned, query.state.data, timedOut),
        query.state.data,
      ),
  });

  const phase = checkoutPhase(returned, plan.data, timedOut);
  const settled = returned === 'canceled' || phase === 'activated' || phase === 'processing';

  useEffect(() => {
    if (!settled || !searchParams.has('checkout')) {
      return;
    }
    const next = new URLSearchParams(searchParams);
    next.delete('checkout');
    const search = next.toString();
    navigate(
      { pathname: location.pathname, search: search === '' ? '' : `?${search}` },
      { replace: true },
    );
  }, [settled, searchParams, navigate, location.pathname]);

  useEffect(() => {
    if (returned !== 'success') {
      return;
    }
    const handle = setTimeout(() => setTimedOut(true), CHECKOUT_CONFIRM_LIMIT_MS);
    return () => clearTimeout(handle);
  }, [returned]);

  return (
    <section className="lg-plans">
      <PageHeader title={message('plans.title')} />

      {phase === 'activated' && plan.data !== undefined ? (
        <Notice tone="success" live="polite">
          {messageWith('plans.checkoutActivated', { plan: planName(plan.data.plan) })}
        </Notice>
      ) : null}

      {phase === 'confirming' ? (
        <Notice tone="info" live="polite">
          {message('plans.checkoutConfirming')}
        </Notice>
      ) : null}

      {phase === 'processing' ? (
        <Notice
          tone="warning"
          live="polite"
          action={
            <Button
              variant="secondary"
              label={message('plans.checkoutCheckAgain')}
              onClick={() => {
                void plan.refetch();
              }}
            />
          }
        >
          {message('plans.checkoutProcessing')}
        </Notice>
      ) : null}

      {phase === 'idle' || phase === 'activated' ? (
        <>
          {plan.isPending ? <LoadingState label={message('plans.loading')} /> : null}

          {plan.isError ? (
            <ErrorState
              message={message('plan.loadFailed')}
              onRetry={() => {
                void plan.refetch();
              }}
            />
          ) : null}

          {plan.data === undefined ? null : <PlansBody usage={plan.data} />}
        </>
      ) : null}
    </section>
  );
}
