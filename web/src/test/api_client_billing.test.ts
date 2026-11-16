import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const baseUrl = 'http://api.test';

function clientWith(fake: ReturnType<typeof createFakeTransport>) {
  return createApiClient({ baseUrl, transport: fake.transport });
}

const CHECKOUT_URL = 'https://checkout.stripe.com/c/pay/abc';
const PORTAL_URL = 'https://billing.stripe.com/p/session/xyz';

describe('billing checkout client (LGN-215, LGN-210 contract)', () => {
  it('Happy path: createCheckout posts once with no body and returns the provider URL', async () => {
    const fake = createFakeTransport(
      successOutcome({ url: CHECKOUT_URL, expires_at: '2026-11-17T10:00:00Z' }),
    );

    await expect(clientWith(fake).createCheckout()).resolves.toBe(CHECKOUT_URL);

    expect(fake.lastRequest).toMatchObject({
      method: 'POST',
      url: `${baseUrl}/billing/checkout`,
      credentials: 'omit',
    });
    expect(fake.lastRequest?.body).toBeUndefined();
    expect(fake.attempts).toBe(1);
  });

  it('Error: ALREADY_SUBSCRIBED keeps its code and takes the already-subscribed kind', async () => {
    const fake = createFakeTransport(errorOutcome('ALREADY_SUBSCRIBED', 'active', 409));

    const error = await clientWith(fake)
      .createCheckout()
      .catch((cause: unknown) => cause);

    expect(error).toBeInstanceOf(DomainFailure);
    expect(error).toMatchObject({
      kind: 'alreadySubscribed',
      status: 409,
      code: 'ALREADY_SUBSCRIBED',
    });
    expect(fake.attempts).toBe(1);
  });

  it('Error: BILLING_UNAVAILABLE takes the billing-unavailable kind and is not repeated', async () => {
    const fake = createFakeTransport(errorOutcome('BILLING_UNAVAILABLE', 'down', 503));

    await expect(clientWith(fake).createCheckout()).rejects.toMatchObject({
      kind: 'billingUnavailable',
      status: 503,
    });
    expect(fake.attempts).toBe(1);
  });

  it('Edge: a 200 answer without a provider URL is an unreadable answer', async () => {
    const fake = createFakeTransport(successOutcome({ expires_at: '2026-11-17T10:00:00Z' }));

    await expect(clientWith(fake).createCheckout()).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Edge: a 200 answer with a non-HTTPS URL is refused, not navigated to', async () => {
    const fake = createFakeTransport(successOutcome({ url: ['javas', 'cript:alert(1)'].join('') }));

    await expect(clientWith(fake).createCheckout()).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: a transport failure is offline and the POST is never repeated', async () => {
    const fake = createFakeTransport({ failure: new TypeError('network down') });

    await expect(clientWith(fake).createCheckout()).rejects.toMatchObject({ kind: 'offline' });
    expect(fake.attempts).toBe(1);
  });
});

describe('billing portal client (LGN-215, LGN-212 contract)', () => {
  it('Happy path: openPortal posts to the portal and returns its URL', async () => {
    const fake = createFakeTransport(successOutcome({ url: PORTAL_URL }));

    await expect(clientWith(fake).openPortal()).resolves.toBe(PORTAL_URL);

    expect(fake.lastRequest).toMatchObject({ method: 'POST', url: `${baseUrl}/billing/portal` });
    expect(fake.attempts).toBe(1);
  });

  it('Error: NO_SUBSCRIPTION takes the no-subscription kind', async () => {
    const fake = createFakeTransport(errorOutcome('NO_SUBSCRIPTION', 'none', 404));

    await expect(clientWith(fake).openPortal()).rejects.toMatchObject({
      kind: 'noSubscription',
      status: 404,
      code: 'NO_SUBSCRIPTION',
    });
  });

  it('Edge: an answer without a URL is an unreadable answer', async () => {
    const fake = createFakeTransport(successOutcome({}));

    await expect(clientWith(fake).openPortal()).rejects.toBeInstanceOf(DefensiveParseFailure);
  });
});

describe('billing refund client (LGN-215, LGN-213 contract)', () => {
  it('Happy path: requestRefund posts once and resolves on the 202 answer', async () => {
    const fake = createFakeTransport({ status: 202, body: '' });

    await expect(clientWith(fake).requestRefund()).resolves.toBeUndefined();

    expect(fake.lastRequest).toMatchObject({ method: 'POST', url: `${baseUrl}/billing/refund` });
    expect(fake.attempts).toBe(1);
  });

  it('Error: REFUND_NOT_ELIGIBLE takes the not-eligible kind and is not repeated', async () => {
    const fake = createFakeTransport(errorOutcome('REFUND_NOT_ELIGIBLE', 'window expired', 409));

    await expect(clientWith(fake).requestRefund()).rejects.toMatchObject({
      kind: 'refundNotEligible',
      status: 409,
      code: 'REFUND_NOT_ELIGIBLE',
    });
    expect(fake.attempts).toBe(1);
  });

  it('Error: a transport failure is offline and the refund is never sent twice', async () => {
    const fake = createFakeTransport({ failure: new TypeError('network down') });

    await expect(clientWith(fake).requestRefund()).rejects.toMatchObject({ kind: 'offline' });
    expect(fake.attempts).toBe(1);
  });
});
