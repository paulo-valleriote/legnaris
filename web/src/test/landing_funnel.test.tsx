import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { resetHomeVisitReport } from '../landing/funnel_beacon';
import { renderApp } from './render';

const FUNNEL_URL = 'http://api.test/funnel/home';

function stubBeacon(accepted = true) {
  const beacon = vi.fn(() => accepted);
  Object.defineProperty(navigator, 'sendBeacon', { configurable: true, value: beacon });
  return beacon;
}

async function visit() {
  renderApp('/', { session: 'unauthenticated', apiBaseUrl: 'http://api.test' });
  await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());
  await waitFor(() => expect(screen.getByTestId('landing')).toBeInTheDocument());
}

afterEach(() => {
  Reflect.deleteProperty(navigator, 'sendBeacon');
  resetHomeVisitReport();
  vi.unstubAllGlobals();
});

describe('landing funnel counters', () => {
  it('Happy path: opening the landing reports one home visit', async () => {
    const beacon = stubBeacon();

    await visit();

    await waitFor(() => expect(beacon.mock.calls.length).toBeGreaterThan(0));
    expect(beacon).toHaveBeenCalledTimes(1);
    expect(beacon).toHaveBeenCalledWith(FUNNEL_URL, '{"event":"visit"}');
  });

  it('Happy path: the hero create action reports a CTA click', async () => {
    const beacon = stubBeacon();
    const user = userEvent.setup();
    await visit();
    await waitFor(() => expect(beacon).toHaveBeenCalledTimes(1));
    beacon.mockClear();

    const hero = screen.getByRole('banner');
    await user.click(within(hero).getByRole('link', { name: message('landing.create') }));

    expect(beacon).toHaveBeenCalledWith(FUNNEL_URL, '{"event":"cta_click"}');
  });

  it('Edge: the footer create action is not the hero call to action', async () => {
    const beacon = stubBeacon();
    const user = userEvent.setup();
    await visit();
    await waitFor(() => expect(beacon).toHaveBeenCalledTimes(1));
    beacon.mockClear();

    const footer = screen.getByRole('contentinfo');
    await user.click(within(footer).getByRole('link', { name: message('landing.create') }));

    expect(beacon).not.toHaveBeenCalledWith(FUNNEL_URL, '{"event":"cta_click"}');
  });

  it('Edge: the sign-in action is not a funnel call to action', async () => {
    const beacon = stubBeacon();
    const user = userEvent.setup();
    await visit();
    await waitFor(() => expect(beacon).toHaveBeenCalledTimes(1));
    beacon.mockClear();

    const hero = screen.getByRole('banner');
    await user.click(within(hero).getByRole('link', { name: message('auth.submitLogin') }));

    expect(beacon).not.toHaveBeenCalled();
  });

  it('Edge: without sendBeacon the visit falls back to a keepalive request', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });

    vi.stubGlobal('fetch', fetchMock);
    await visit();

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(fetchMock).toHaveBeenCalledWith(
      FUNNEL_URL,
      expect.objectContaining({
        method: 'POST',
        body: '{"event":"visit"}',
        keepalive: true,
        credentials: 'omit',
        mode: 'cors',
      }),
    );
  });

  it('Edge: a refused beacon falls back to a keepalive request', async () => {
    const beacon = stubBeacon(false);
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });

    vi.stubGlobal('fetch', fetchMock);
    await visit();

    await waitFor(() => expect(beacon).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
  });

  it('Edge: a throwing beacon falls back to a keepalive request', async () => {
    const beacon = vi.fn(() => {
      throw new Error('beacon unavailable');
    });
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });

    Object.defineProperty(navigator, 'sendBeacon', { configurable: true, value: beacon });
    vi.stubGlobal('fetch', fetchMock);
    await visit();

    await waitFor(() => expect(beacon).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
  });

  it('Edge: a throwing keepalive request never breaks the landing', async () => {
    vi.stubGlobal('fetch', () => {
      throw new Error('offline');
    });

    await visit();

    expect(screen.getByTestId('landing')).toBeInTheDocument();
  });
});
