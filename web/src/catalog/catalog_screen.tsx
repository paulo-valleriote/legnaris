import { useCallback, useState } from 'react';
import type { UseQueryResult } from '@tanstack/react-query';
import { AdSlot } from '../ads/ad_slot';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import type { TemplateSummary } from '../api/models/template';
import { message } from '../l10n/strings';
import { Button } from '../ui/button';
import { ArrowRightIcon } from '../ui/icons';
import { PageHeader } from '../ui/page_header';
import { EmptyState, ErrorState, LoadingState } from '../ui/state';
import { CreateSheetDialog } from './create_sheet_dialog';
import { ResumeNotice } from './resume_notice';
import { useCatalog } from './queries';
import './catalog.css';

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
  readonly onOpen: (templateId: string) => void;
}

function CatalogBody({ phase, onRetry, onOpen }: CatalogBodyProps) {
  switch (phase.kind) {
    case 'loading':
      return <LoadingState label={message('catalog.loading')} />;
    case 'empty':
      return (
        <EmptyState
          message={message('catalog.empty')}
          action={
            <Button
              variant="secondary"
              size="md"
              label={message('common.retry')}
              onClick={onRetry}
            />
          }
        />
      );
    case 'loaded':
      return (
        <ul className="lg-catalog__list">
          {phase.entries.map((entry, index) => (
            <li className="lg-catalog__item" key={`${entry.id}-${index}`}>
              <Button
                variant="row"
                name={entry.name}
                className="lg-catalog__entry"
                trailingIcon={<ArrowRightIcon className="lg-catalog__open" />}
                onClick={() => onOpen(entry.id)}
              >
                <span className="lg-catalog__text">
                  <span className="lg-catalog__name">{entry.name}</span>
                  <span className="lg-catalog__id">{entry.id}</span>
                  {entry.gameSystem === undefined ? null : (
                    <span className="lg-catalog__compatibility" data-testid="catalog-compatibility">
                      {entry.gameSystem.compatibilityLabel}
                    </span>
                  )}
                </span>
              </Button>
            </li>
          ))}
        </ul>
      );
    case 'error':
    case 'parseError':
      return <ErrorState message={phase.copy} onRetry={onRetry} />;
  }
}

export function CatalogScreen() {
  const query = useCatalog();
  const [openTemplateId, setOpenTemplateId] = useState<string | undefined>(undefined);
  const phase = resolveCatalogPhase(query);

  const handleRetry = useCallback(() => {
    void query.refetch();
  }, [query]);

  const handleOpen = useCallback((templateId: string) => {
    setOpenTemplateId(templateId);
  }, []);

  const handleClose = useCallback(() => {
    setOpenTemplateId(undefined);
  }, []);

  return (
    <div className="lg-catalog" data-testid="catalog">
      <PageHeader title={message('catalog.title')} />
      <ResumeNotice />
      <CatalogBody phase={phase} onRetry={handleRetry} onOpen={handleOpen} />
      <AdSlot surface="catalog" />
      {openTemplateId !== undefined && (
        <CreateSheetDialog templateId={openTemplateId} onClose={handleClose} />
      )}
    </div>
  );
}
