/**
 * The template catalog screen (WCT-01, WCT-03 … WCT-10, WCT-26, WCT-28).
 *
 * It resolves exactly one render phase from the catalog query and renders it, and
 * it owns the creation surface's open state: activating an entry records the
 * identifier and the triggering element, and closing returns focus to it without
 * touching the catalog's cache or navigating.
 */

import { useCallback, useRef, useState } from 'react';
import type { UseQueryResult } from '@tanstack/react-query';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import type { TemplateSummary } from '../api/models/template';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';
import { CreateSheetDialog } from './create_sheet_dialog';
import { useCatalog } from './queries';
import './catalog.css';

/** The single situation the catalog renders; exactly one is active (SC-001). */
type CatalogRenderPhase =
  | { readonly kind: 'loading' }
  | { readonly kind: 'loaded'; readonly entries: readonly TemplateSummary[] }
  | { readonly kind: 'empty' }
  | { readonly kind: 'error'; readonly copy: string }
  | { readonly kind: 'parseError'; readonly copy: string };

function resolveCatalogPhase(query: UseQueryResult<TemplateSummary[], Error>): CatalogRenderPhase {
  if (query.isPending) {
    return { kind: 'loading' };
  }

  if (query.isError) {
    if (query.error instanceof DefensiveParseFailure) {
      return { kind: 'parseError', copy: parseFailureMessage() };
    }
    return {
      kind: 'error',
      copy: failureMessage(query.error instanceof DomainFailure ? query.error.kind : 'unknown'),
    };
  }

  const entries = query.data ?? [];
  return entries.length === 0 ? { kind: 'empty' } : { kind: 'loaded', entries };
}

interface CatalogBodyProps {
  readonly phase: CatalogRenderPhase;
  readonly onRetry: () => void;
  readonly onOpen: (templateId: string, trigger: HTMLElement) => void;
}

function CatalogBody({ phase, onRetry, onOpen }: CatalogBodyProps) {
  switch (phase.kind) {
    case 'loading':
      return <p className="lg-catalog__message">{message('catalog.loading')}</p>;
    case 'empty':
      return (
        <div className="lg-catalog__failure">
          <p className="lg-catalog__message">{message('catalog.empty')}</p>
          <Button variant="primary" size="md" label={message('catalog.retry')} onClick={onRetry} />
        </div>
      );
    case 'loaded':
      return (
        <ul className="lg-catalog__list">
          {phase.entries.map((entry, index) => (
            <li className="lg-catalog__item" key={`${entry.id}-${index}`}>
              <button
                type="button"
                className="lg-catalog__entry lg-button lg-button--secondary lg-button--md"
                title={message('catalog.openHint')}
                onClick={(event) => onOpen(entry.id, event.currentTarget)}
              >
                <span className="lg-catalog__name">{entry.name}</span>
                <span className="lg-catalog__id">{entry.id}</span>
              </button>
            </li>
          ))}
        </ul>
      );
    case 'error':
    case 'parseError':
      return (
        <div className="lg-catalog__failure">
          <p className="lg-catalog__message">{phase.copy}</p>
          <Button variant="primary" size="md" label={message('catalog.retry')} onClick={onRetry} />
        </div>
      );
  }
}

export function CatalogScreen() {
  const query = useCatalog();
  const [openTemplateId, setOpenTemplateId] = useState<string | undefined>(undefined);
  const triggerRef = useRef<HTMLElement | null>(null);
  const phase = resolveCatalogPhase(query);

  const handleRetry = useCallback(() => {
    void query.refetch();
  }, [query]);

  const handleOpen = useCallback((templateId: string, trigger: HTMLElement) => {
    triggerRef.current = trigger;
    setOpenTemplateId(templateId);
  }, []);

  const handleClose = useCallback(() => {
    setOpenTemplateId(undefined);
    triggerRef.current?.focus();
  }, []);

  return (
    <section className="lg-catalog" aria-labelledby="catalog-title" data-testid="catalog">
      <h1 className="lg-catalog__title" id="catalog-title">
        {message('catalog.title')}
      </h1>
      <CatalogBody phase={phase} onRetry={handleRetry} onOpen={handleOpen} />
      {openTemplateId !== undefined && (
        <CreateSheetDialog templateId={openTemplateId} onClose={handleClose} />
      )}
    </section>
  );
}
