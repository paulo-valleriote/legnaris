import { describe, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import { QueryClient } from '@tanstack/react-query';
import { AdSlot } from '../ads/ad_slot';
import type { AdSurface, AdsConfig } from '../ads/ads_config';
import { AppProviders } from '../app/providers';
import type { AuthApi } from '../api/auth_api';
import type { BillingStatus, PlanUsage } from '../api/models/plan';
import { createAuthSession } from '../auth/auth_session';
import { PLAN_QUERY_KEY } from '../catalog/queries';

const CLIENT = 'ca-pub-1234567890123456';

const CONFIG: AdsConfig = {
  client: CLIENT,
  slots: { catalog: '1111111111', campaigns: '2222222222', landing: '3333333333' },
};

const BILLING: BillingStatus = {
  subscriptionStatus: null,
  currentPeriodEnd: null,
  cancelAtPeriodEnd: false,
  paymentIssue: false,
  refundEligible: false,
  refundState: null,
};

function plan(overrides: Partial<PlanUsage> = {}, billing: Partial<BillingStatus> = {}): PlanUsage {
  return {
    plan: 'free',
    sheets: {},
    consentAt: null,
    consentVersion: null,
    llmConsentRequired: false,
    activeGenerationId: null,
    billing: { ...BILLING, ...billing },
    ...overrides,
  };
}

const FREE_PLAN = plan();
const PAID_PLAN = plan({ plan: 'paid' }, { subscriptionStatus: 'active' });

function unusedAuthApi(): AuthApi {
  const unused = () => Promise.reject(new Error('auth operation is not scripted for this suite'));
  return {
    register: unused,
    login: unused,
    refresh: unused,
    logout: () => Promise.resolve(),
    verifyChallenge: unused,
    resendChallenge: unused,
  };
}

function sessionFor(signedIn: boolean) {
  return createAuthSession({
    authApi: unusedAuthApi(),
    initialState: signedIn ? 'authenticated' : 'unauthenticated',
    initialAccessToken: signedIn ? 'test-access-token' : null,
  });
}

function detachedDocument(): Document {
  return document.implementation.createHTMLDocument('ads');
}

function consentWindow(consent: 'accepted' | 'declined'): Window {
  return {
    __tcfapi: (command: string, _version: number, callback: (data: unknown) => void) => {
      if (command !== 'addEventListener') {
        return;
      }
      callback(
        consent === 'accepted'
          ? { gdprApplies: false, eventStatus: 'tcloaded', listenerId: 1 }
          : {
              gdprApplies: true,
              eventStatus: 'tcloaded',
              purpose: { consents: {} },
              listenerId: 1,
            },
      );
    },
  } as unknown as Window;
}

interface RenderSlotOptions {
  readonly signedIn?: boolean;
  readonly usage?: PlanUsage | undefined;
  readonly config?: AdsConfig | null;
  readonly surface?: AdSurface;
  readonly adsDocument?: Document;
  readonly adsWindow?: Window;
}

function renderSlot(options: RenderSlotOptions = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  if (options.usage !== undefined) {
    queryClient.setQueryData(PLAN_QUERY_KEY, options.usage);
  }

  const result = render(
    <AppProviders
      session={sessionFor(options.signedIn ?? true)}
      queryClient={queryClient}
      adsConfig={options.config === undefined ? CONFIG : options.config}
      adsDocument={options.adsDocument ?? detachedDocument()}
      adsWindow={options.adsWindow ?? ({} as unknown as Window)}
    >
      <AdSlot surface={options.surface ?? 'catalog'} />
    </AppProviders>,
  );

  return { ...result, queryClient };
}

describe('AdSlot', () => {
  it('Happy path: a Free signed-in account renders the labelled unit with its client and slot', () => {
    const { container } = renderSlot({ usage: FREE_PLAN });

    const slot = screen.getByRole('complementary', { name: 'Advertisement' });
    expect(slot).toBeInTheDocument();
    expect(screen.getByText('Advertisement')).toBeInTheDocument();

    const unit = container.querySelector('.lg-ad-slot__unit');
    expect(unit?.tagName).toBe('INS');
    expect(unit).toHaveAttribute('data-ad-client', CLIENT);
    expect(unit).toHaveAttribute('data-ad-slot', '1111111111');
    expect(unit).toHaveAttribute('data-ad-format', 'horizontal');
    expect(unit).toHaveAttribute('data-full-width-responsive', 'false');
  });

  it('Edge: a paid account renders nothing and loads no ad script', () => {
    const doc = detachedDocument();
    const { container } = renderSlot({ usage: PAID_PLAN, adsDocument: doc });

    expect(container).toBeEmptyDOMElement();
    expect(doc.querySelectorAll('script')).toHaveLength(0);
  });

  it('Edge: a signed-in account whose plan is still loading renders nothing', () => {
    const { container } = renderSlot({ usage: undefined });

    expect(container).toBeEmptyDOMElement();
  });

  it('Happy path: a logged-out visitor sees the landing slot', () => {
    renderSlot({ signedIn: false, usage: undefined, surface: 'landing' });

    expect(screen.getByRole('complementary', { name: 'Advertisement' })).toBeInTheDocument();
  });

  it('Edge: a logged-out visitor sees no slot on the catalog', () => {
    const { container } = renderSlot({ signedIn: false, usage: undefined, surface: 'catalog' });

    expect(container).toBeEmptyDOMElement();
  });

  it('Edge: a missing slot id renders nothing and loads nothing', () => {
    const doc = detachedDocument();
    const { container } = renderSlot({
      usage: FREE_PLAN,
      config: { client: CLIENT, slots: { campaigns: '2222222222' } },
      adsDocument: doc,
    });

    expect(container).toBeEmptyDOMElement();
    expect(doc.querySelectorAll('script')).toHaveLength(0);
  });

  it('Happy path: an upgrade to a paid plan unmounts the slot from the query cache without a reload', async () => {
    const { container, queryClient } = renderSlot({ usage: FREE_PLAN });

    expect(screen.getByRole('complementary', { name: 'Advertisement' })).toBeInTheDocument();

    act(() => {
      queryClient.setQueryData(PLAN_QUERY_KEY, PAID_PLAN);
    });

    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it('Happy path: accepted consent loads the script and queues the unit once', async () => {
    const doc = detachedDocument();
    const win = consentWindow('accepted');
    renderSlot({ usage: FREE_PLAN, adsDocument: doc, adsWindow: win });

    await waitFor(() => expect(win.adsbygoogle).toHaveLength(1));
    expect(doc.querySelectorAll('script[data-lg-ads-loader]')).toHaveLength(1);
    expect(doc.querySelectorAll('script[data-lg-ads-cmp]')).toHaveLength(1);
  });

  it('Edge: declined consent asks for non-personalized ads before the queue is used', async () => {
    const doc = detachedDocument();
    const win = consentWindow('declined');
    renderSlot({ usage: FREE_PLAN, adsDocument: doc, adsWindow: win });

    await waitFor(() => expect(win.adsbygoogle?.requestNonPersonalizedAds).toBe(1));
    expect(doc.querySelectorAll('script[data-lg-ads-loader]')).toHaveLength(1);
  });

  it('Edge: a downgrade back to Free queues the rerendered unit again', async () => {
    const doc = detachedDocument();
    const win = consentWindow('accepted');
    const { container, queryClient } = renderSlot({
      usage: PAID_PLAN,
      adsDocument: doc,
      adsWindow: win,
    });

    act(() => {
      queryClient.setQueryData(PLAN_QUERY_KEY, FREE_PLAN);
    });
    await waitFor(() => expect(container.querySelector('.lg-ad-slot__unit')).not.toBeNull());
    await waitFor(() => expect(win.adsbygoogle).toHaveLength(1));

    act(() => {
      queryClient.setQueryData(PLAN_QUERY_KEY, PAID_PLAN);
    });
    await waitFor(() => expect(container).toBeEmptyDOMElement());

    act(() => {
      queryClient.setQueryData(PLAN_QUERY_KEY, FREE_PLAN);
    });
    await waitFor(() => expect(win.adsbygoogle).toHaveLength(2));
    expect(doc.querySelectorAll('script[data-lg-ads-loader]')).toHaveLength(1);
  });

  it('Edge: a paid account never asks the CMP or injects anything, even with a consent API', () => {
    const doc = detachedDocument();
    const tcf = () => {
      throw new Error('the CMP must not be consulted for a paid account');
    };
    const win = { __tcfapi: vi.fn(tcf) } as unknown as Window;
    const { container } = renderSlot({ usage: PAID_PLAN, adsDocument: doc, adsWindow: win });

    expect(container).toBeEmptyDOMElement();
    expect(doc.querySelectorAll('script')).toHaveLength(0);
    expect(win.__tcfapi).not.toHaveBeenCalled();
  });
});
