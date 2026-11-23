import { useQueries, useQuery } from '@tanstack/react-query';
import type { QueryClient } from '@tanstack/react-query';
import type { ApiClient } from '../api/client';
import type { Layout } from '../api/models/layout';
import type { PlanUsage } from '../api/models/plan';
import type { Sheet } from '../api/models/sheet';
import { useApiClient } from '../app/providers';

export const CATALOG_QUERY_KEY = ['templates'] as const;

export function templateDetailKey(id: string) {
  return ['template', id] as const;
}

export const PLAN_QUERY_KEY = ['me', 'plan'] as const;

export function usePlanUsage(options: { refetchInterval?: PlanRefetchInterval } = {}) {
  const client = useApiClient();

  return useQuery({
    queryKey: PLAN_QUERY_KEY,
    queryFn: () => client.getMyPlan(),
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    ...(options.refetchInterval === undefined ? {} : { refetchInterval: options.refetchInterval }),
  });
}

export type PlanRefetchInterval = (query: {
  state: { data: PlanUsage | undefined };
}) => number | false;

export function useCachedPlanUsage() {
  const client = useApiClient();

  return useQuery({
    queryKey: PLAN_QUERY_KEY,
    queryFn: () => client.getMyPlan(),
    enabled: false,
  });
}

export function sheetKey(id: string) {
  return ['sheet', id] as const;
}

export function seedCreatedSheet(queryClient: QueryClient, sheet: Sheet): void {
  queryClient.setQueryData(sheetKey(sheet.id), sheet);
}

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
      kind: current.kind,
      name: current.name,
      data: current.data,
      createdAt: current.createdAt,
      updatedAt: current.updatedAt,
      ...(current.dice === undefined ? {} : { dice: current.dice }),
      ...(current.campaignId === undefined ? {} : { campaignId: current.campaignId }),
    };
    return layout === null ? unarranged : { ...unarranged, layout };
  });
}

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

export function useTemplateDetail(id: string) {
  const client = useApiClient();

  return useQuery(templateDetailOptions(client, id));
}

export function useTemplateDetails(ids: readonly string[]) {
  const client = useApiClient();

  return useQueries({
    queries: ids.map((id) => templateDetailOptions(client, id)),
  });
}

function templateDetailOptions(client: ApiClient, id: string) {
  return {
    queryKey: templateDetailKey(id),
    queryFn: () => client.getTemplate(id),
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
  };
}
