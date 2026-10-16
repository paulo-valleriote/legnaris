/**
 * Opening one sheet by URL identifier (WSC-05 … WSC-11).
 *
 * The hook resolves the load situation of the sheet named by the route and, once
 * the sheet is known, its template detail. It issues the one `GET /sheets/:id`
 * (and one `GET /templates/:id`) through the F2 client, reuses a fresh seeded
 * entry so a created sheet is never fetched twice (WSC-02), caches a loaded sheet
 * locally and evicts a missing one best-effort. It owns no edit state.
 */

import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { Sheet } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import { useApiClient, useSheetStore } from '../app/providers';
import { sheetKey, templateDetailKey } from '../catalog/queries';
import { message } from '../l10n/strings';
import { cacheSheet, evictSheet } from '../library/sheet_library';

/** The sheet cache key the creation flow seeds (WSC-02). */
export const SHEET_QUERY_KEY = sheetKey;

/** The load situation of one sheet, exactly one active (WSC-05 … WSC-10). */
export type SheetOpenResult =
  | { readonly kind: 'loading' }
  | { readonly kind: 'loaded'; readonly sheet: Sheet; readonly template: TemplateDetail }
  | { readonly kind: 'notFound'; readonly message: string }
  | { readonly kind: 'failure'; readonly message: string; readonly retry: () => void }
  | { readonly kind: 'templateLoading' }
  | { readonly kind: 'templateFailure'; readonly message: string; readonly retry: () => void };

function describeFailure(error: unknown): string {
  if (error instanceof DefensiveParseFailure) {
    return parseFailureMessage();
  }
  return failureMessage(error instanceof DomainFailure ? error.kind : 'unknown');
}

/**
 * Resolves the sheet's situation and, on success, its template's detail
 * (WSC-05 … WSC-10).
 *
 * A mount issues exactly one sheet request; a `NOT_FOUND` yields the missing
 * situation with no Retry and evicts the sheet and its draft, while any other
 * failure (an undecodable payload included) yields a mapped message with a Retry
 * that re-requests exactly once.
 */
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
