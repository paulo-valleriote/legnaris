import { useEffect } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { render, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, useQueryClient } from '@tanstack/react-query';
import { AppProviders, useApiClient } from '../app/providers';
import type { TemplateSummary } from '../api/models/template';
import { createFakeTransport, successOutcome } from './fake_transport';

const ONE_TEMPLATE: readonly TemplateSummary[] = [
  { id: 'dnd5e', name: 'Dungeons & Dragons 5e', default: true, fields: {} },
];

/** Calls the client from context and reports the template names it decoded. */
function CatalogProbe({ onResult }: { onResult: (names: readonly string[]) => void }) {
  const client = useApiClient();

  useEffect(() => {
    void client.listTemplates().then((templates) => onResult(templates.map((entry) => entry.name)));
  }, [client, onResult]);

  return null;
}

describe('AppProviders (WCT-02, WCT-12)', () => {
  it('Happy path: builds one API client over the injected transport and exposes it through context', async () => {
    const fake = createFakeTransport(successOutcome(ONE_TEMPLATE));
    const onResult = vi.fn();

    render(
      <AppProviders transport={fake.transport}>
        <CatalogProbe onResult={onResult} />
      </AppProviders>,
    );

    await waitFor(() => expect(onResult).toHaveBeenCalledWith(['Dungeons & Dragons 5e']));
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.url).toContain('/templates');
  });

  it('Edge: consuming useApiClient outside the provider raises the documented error', () => {
    expect(() => renderHook(() => useApiClient())).toThrow(/AppProviders/);
  });

  it('Happy path: the query client it creates disables query retries, leaving retry to the F2 client', () => {
    const fake = createFakeTransport(successOutcome([]));
    let captured: QueryClient | undefined;

    function QueryClientProbe() {
      captured = useQueryClient();
      return null;
    }

    render(
      <AppProviders transport={fake.transport}>
        <QueryClientProbe />
      </AppProviders>,
    );

    expect(captured?.getDefaultOptions().queries?.retry).toBe(false);
  });
});
