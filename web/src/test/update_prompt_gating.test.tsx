import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createAuthApi } from '../api/auth_api';
import { AppProviders } from '../app/providers';
import { createAuthSession } from '../auth/auth_session';
import { UpdatePrompt } from '../pwa/update_prompt';
import { createDeferredTransport } from './fake_transport';

const SNAPSHOT = vi.hoisted(() => ({ needRefresh: false, offlineReady: true }));

vi.mock('../pwa/register_sw', () => ({
  subscribe: () => () => undefined,
  getSnapshot: () => SNAPSHOT,
  applyUpdate: vi.fn(),
  dismissUpdate: vi.fn(),
}));

describe('offline-ready notice gating', () => {
  it('Edge: stays hidden while no session is held', () => {
    const session = createAuthSession({
      authApi: createAuthApi({
        baseUrl: 'http://api.test',
        transport: createDeferredTransport().transport,
        readCsrfToken: () => undefined,
      }),
    });
    render(
      <AppProviders session={session}>
        <UpdatePrompt />
      </AppProviders>,
    );

    expect(session.getSnapshot().state).not.toBe('authenticated');
    expect(screen.queryByTestId('update-prompt')).not.toBeInTheDocument();
  });
});
