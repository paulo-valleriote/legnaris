import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { QueryClient } from '@tanstack/react-query';
import { useApiClient } from '../app/providers';
import type { Campaign } from '../api/models/campaign';
import type { Sheet } from '../api/models/sheet';
import { drift } from '../api/parse';
import { GM_POLL_INTERVAL_MS } from './campaign_view';

export const CAMPAIGNS_QUERY_KEY = ['campaigns'] as const;

export function campaignKey(id: string) {
  return ['campaign', id] as const;
}

export function campaignSheetsKey(id: string) {
  return ['campaign', id, 'sheets'] as const;
}

export function gmSheetKey(campaignId: string, sheetId: string) {
  return ['campaign', campaignId, 'sheet', sheetId] as const;
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

export function useCampaignSheets(id: string, enabled: boolean) {
  const client = useApiClient();

  return useQuery({
    queryKey: campaignSheetsKey(id),
    queryFn: () => client.listCampaignSheets(id),
    enabled,
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

export function seedCampaign(queryClient: QueryClient, campaign: Campaign): void {
  queryClient.setQueryData(campaignKey(campaign.id), campaign);
}
