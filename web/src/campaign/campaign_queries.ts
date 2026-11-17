import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { QueryClient } from '@tanstack/react-query';
import { useApiClient } from '../app/providers';
import type { Campaign } from '../api/models/campaign';
import type { SheetKind } from '../api/models/plan';
import type { Sheet } from '../api/models/sheet';
import { failureKind } from '../api/error_mapper';
import { drift } from '../api/parse';
import { GM_POLL_INTERVAL_MS } from './campaign_view';

export const CAMPAIGNS_QUERY_KEY = ['campaigns'] as const;

export function campaignKey(id: string) {
  return ['campaign', id] as const;
}

export function campaignSheetsKey(id: string) {
  return ['campaign', id, 'sheets'] as const;
}

export function npcLinksKey(id: string) {
  return ['campaign', id, 'npcs'] as const;
}

export function gmSheetKey(campaignId: string, sheetId: string) {
  return ['campaign', campaignId, 'sheet', sheetId] as const;
}

export function npcSheetKey(campaignId: string, sheetId: string) {
  return ['campaign', campaignId, 'npc', sheetId] as const;
}

export interface GmSheetSnapshot {
  readonly sheet: Sheet;
  readonly etag: string | undefined;
}

export function useCampaigns() {
  const client = useApiClient();

  return useQuery({
    queryKey: CAMPAIGNS_QUERY_KEY,
    queryFn: () => client.listCampaigns(),
    retry: false,
  });
}

export function useCampaign(id: string) {
  const client = useApiClient();

  return useQuery({
    queryKey: campaignKey(id),
    queryFn: () => client.getCampaign(id),
    retry: false,
  });
}

export function ownedSheetsKey(kind: SheetKind) {
  return ['sheets', 'owned', kind] as const;
}

export function useOwnedSheets(kind: SheetKind, enabled: boolean) {
  const client = useApiClient();

  return useQuery({
    queryKey: ownedSheetsKey(kind),
    queryFn: () => client.listOwnedSheets(kind),
    enabled,
    retry: false,
  });
}

export function useOwnedCharacterSheets(enabled: boolean) {
  return useOwnedSheets('character', enabled);
}

export function useCampaignSheets(id: string, enabled: boolean) {
  const client = useApiClient();

  return useQuery({
    queryKey: campaignSheetsKey(id),
    queryFn: () => client.listCampaignSheets(id),
    enabled,
    retry: false,
  });
}

export function useNpcLinks(id: string, enabled: boolean) {
  const client = useApiClient();

  return useQuery({
    queryKey: npcLinksKey(id),
    queryFn: () => client.listNpcLinks(id),
    enabled,
    refetchInterval: GM_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
    retry: false,
  });
}

export function useGmSheet(campaignId: string, sheetId: string) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const key = gmSheetKey(campaignId, sheetId);

  return useQuery({
    queryKey: key,
    queryFn: () => loadGmSheet(client, queryClient, campaignId, sheetId),
    refetchInterval: GM_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
    staleTime: 0,
    retry: false,
  });
}

async function loadGmSheet(
  client: ReturnType<typeof useApiClient>,
  queryClient: QueryClient,
  campaignId: string,
  sheetId: string,
): Promise<GmSheetSnapshot> {
  const cached = queryClient.getQueryData<GmSheetSnapshot>(gmSheetKey(campaignId, sheetId));
  const result = await client.getCampaignSheet(campaignId, sheetId, cached?.etag);

  if (!result.notModified) {
    return { sheet: result.sheet, etag: result.etag };
  }
  if (cached === undefined) {
    throw drift('the server answered not modified to a read with no cached copy');
  }
  return cached;
}

export function useNpcSheet(campaignId: string, sheetId: string) {
  const client = useApiClient();
  const queryClient = useQueryClient();
  const key = npcSheetKey(campaignId, sheetId);

  return useQuery({
    queryKey: key,
    queryFn: () => loadNpcSheet(client, queryClient, campaignId, sheetId),
    refetchInterval: (query) =>
      failureKind(query.state.error) === 'notFound' ? false : GM_POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
    staleTime: 0,
    retry: false,
  });
}

async function loadNpcSheet(
  client: ReturnType<typeof useApiClient>,
  queryClient: QueryClient,
  campaignId: string,
  sheetId: string,
): Promise<GmSheetSnapshot> {
  const cached = queryClient.getQueryData<GmSheetSnapshot>(npcSheetKey(campaignId, sheetId));
  const result = await client.getCampaignNpc(campaignId, sheetId, cached?.etag);

  if (!result.notModified) {
    return { sheet: result.sheet, etag: result.etag };
  }
  if (cached === undefined) {
    throw drift('the server answered not modified to a read with no cached copy');
  }
  return cached;
}

export function seedCampaign(queryClient: QueryClient, campaign: Campaign): void {
  queryClient.setQueryData(campaignKey(campaign.id), campaign);
}
