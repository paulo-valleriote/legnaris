/**
 * The catalog and template-detail server state (WCT-02, WCT-07, WCT-08, WCT-12,
 * WCT-16, WCT-21).
 *
 * Each hook is a thin wrapper over the F2 `ApiClient`: the client builds the
 * URL, unwraps the envelope and decodes the payload, so a query function never
 * repeats any of that. The query client mounted by `AppProviders` disables the
 * library's own retry, leaving retry to the F2 client's single read retry.
 */

import { useQuery } from '@tanstack/react-query';
import type { QueryClient } from '@tanstack/react-query';
import type { Layout } from '../api/models/layout';
import type { Sheet } from '../api/models/sheet';
import { useApiClient } from '../app/providers';

/** The catalog's query key; it carries no parameter, so every mount shares one entry. */
export const CATALOG_QUERY_KEY = ['templates'] as const;

/** The detail's query key for one template identifier (WCT-21). */
export function templateDetailKey(id: string) {
  return ['template', id] as const;
}

/** The sheet's query key for one identifier; the creation flow seeds it (WSC-02). */
export function sheetKey(id: string) {
  return ['sheet', id] as const;
}

/**
 * Seeds a created sheet's entry so the sheet screen renders it without a second
 * fetch (WSC-02, WSC-34).
 *
 * `setQueryData` stamps the entry as just-updated, so the sheet query — which
 * reads with an infinite stale time — resolves it as fresh and issues no `GET`.
 */
export function seedCreatedSheet(queryClient: QueryClient, sheet: Sheet): void {
  queryClient.setQueryData(sheetKey(sheet.id), sheet);
}

/**
 * Records the Layout the server stored on the cached sheet, so reopening the sheet
 * shows the committed arrangement without a fetch. `null` (a reset) drops the key.
 */
export function updateSheetLayout(
  queryClient: QueryClient,
  sheetId: string,
  layout: Layout | null,
): void {
  queryClient.setQueryData(sheetKey(sheetId), (current: Sheet | undefined) => {
    if (current === undefined) {
      return current;
    }
    const unarranged: Sheet = {
      id: current.id,
      templateId: current.templateId,
      name: current.name,
      data: current.data,
      createdAt: current.createdAt,
      updatedAt: current.updatedAt,
      ...(current.dice === undefined ? {} : { dice: current.dice }),
    };
    return layout === null ? unarranged : { ...unarranged, layout };
  });
}

/**
 * Reads the template catalog once per app load (WCT-02).
 *
 * `staleTime: Infinity` with focus, mount and reconnect refetching disabled
 * keeps the catalog cached in memory and never re-requests it without a Retry.
 */
export function useCatalog() {
  const client = useApiClient();

  return useQuery({
    queryKey: CATALOG_QUERY_KEY,
    queryFn: () => client.listTemplates(),
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnMount: false,
    refetchOnReconnect: false,
  });
}

/**
 * Reads one template's detail, fresh on every opening (WCT-12, WCT-21).
 *
 * `gcTime: 0` drops the entry when the surface unmounts, so reopening the same
 * template starts at `loading` and never shows a previously opened template's
 * detail (WCT-13).
 */
export function useTemplateDetail(id: string) {
  const client = useApiClient();

  return useQuery({
    queryKey: templateDetailKey(id),
    queryFn: () => client.getTemplate(id),
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
  });
}
