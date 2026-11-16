import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { Sheet } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { DomainFailure } from '../api/errors';
import { failureCopy } from '../api/error_mapper';
import { useApiClient, useSheetStore } from '../app/providers';
import { sheetKey, templateDetailKey } from '../catalog/queries';
import { message } from '../l10n/strings';
import { cacheSheet, evictSheet } from '../library/sheet_library';

export const SHEET_QUERY_KEY = sheetKey;

export type SheetOpenResult =
  | { readonly kind: 'loading' }
  | { readonly kind: 'loaded'; readonly sheet: Sheet; readonly template: TemplateDetail }
  | { readonly kind: 'notFound'; readonly message: string }
  | { readonly kind: 'failure'; readonly message: string; readonly retry: () => void }
  | { readonly kind: 'templateLoading' }
  | { readonly kind: 'templateFailure'; readonly message: string; readonly retry: () => void };

function describeFailure(error: unknown): string {
  return failureCopy(error);
}

export function useSheetOpen(sheetId: string): SheetOpenResult {
  const client = useApiClient();
  const store = useSheetStore();

  const sheetQuery = useQuery({
    queryKey: sheetKey(sheetId),
    queryFn: () => client.getSheet(sheetId),
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnMount: false,
    refetchOnReconnect: false,
  });

  const sheet = sheetQuery.data;
  const templateId = sheet?.templateId;

  const templateQuery = useQuery({
    queryKey: templateDetailKey(templateId ?? ''),
    queryFn: () => client.getTemplate(templateId as string),
    enabled: templateId !== undefined,
    retry: false,
    refetchOnWindowFocus: false,
  });

  const notFound =
    sheetQuery.isError &&
    sheetQuery.error instanceof DomainFailure &&
    sheetQuery.error.kind === 'notFound';

  useEffect(() => {
    if (notFound) {
      void evictSheet(store, sheetId);
    }
  }, [notFound, store, sheetId]);

  useEffect(() => {
    if (sheet !== undefined) {
      void cacheSheet(store, sheet);
    }
  }, [sheet, store]);

  if (sheetQuery.isPending) {
    return { kind: 'loading' };
  }

  if (sheetQuery.isError) {
    if (notFound) {
      return { kind: 'notFound', message: message('sheet.missing') };
    }
    return {
      kind: 'failure',
      message: describeFailure(sheetQuery.error),
      retry: () => {
        void sheetQuery.refetch();
      },
    };
  }

  if (sheet === undefined) {
    return { kind: 'loading' };
  }

  if (templateQuery.isPending) {
    return { kind: 'templateLoading' };
  }

  if (templateQuery.isError) {
    return {
      kind: 'templateFailure',
      message: describeFailure(templateQuery.error),
      retry: () => {
        void templateQuery.refetch();
      },
    };
  }

  if (templateQuery.data === undefined) {
    return { kind: 'templateLoading' };
  }

  return { kind: 'loaded', sheet, template: templateQuery.data };
}
