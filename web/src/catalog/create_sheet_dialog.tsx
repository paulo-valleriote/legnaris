import { useCallback, useEffect, useRef, useState } from 'react';
import type { UseQueryResult } from '@tanstack/react-query';
import { useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { failureCopy } from '../api/error_mapper';
import type { TemplateDetail } from '../api/models/template';
import { useApiClient } from '../app/providers';
import { isSheetLimitReached, SHEET_KINDS } from '../api/models/plan';
import type { SheetKind } from '../api/models/plan';
import type { MessageKey } from '../l10n/strings';
import { message, messageWith } from '../l10n/strings';
import { PLAN_QUERY_KEY, seedCreatedSheet, usePlanUsage, useTemplateDetail } from './queries';

const DEFAULT_KIND: SheetKind = 'character';

const KIND_LABEL_KEYS = {
  character: 'catalog.kindCharacter',
  npc: 'campaign.kindNpc',
  enemy: 'campaign.kindEnemy',
} as const satisfies Record<SheetKind, MessageKey>;

type DetailRenderPhase =
  | { readonly kind: 'loading' }
  | { readonly kind: 'loaded'; readonly name: string }
  | { readonly kind: 'error'; readonly copy: string }
  | { readonly kind: 'notFound'; readonly copy: string }
  | { readonly kind: 'parseError'; readonly copy: string };

function describeFailure(error: unknown): string {
  return failureCopy(error);
}

function resolveDetailPhase(query: UseQueryResult<TemplateDetail, Error>): DetailRenderPhase {
  if (query.isPending) {
    return { kind: 'loading' };
  }

  if (query.isError) {
    const copy = describeFailure(query.error);
    if (query.error instanceof DomainFailure && query.error.kind === 'notFound') {
      return { kind: 'notFound', copy };
    }
    if (query.error instanceof DefensiveParseFailure) {
      return { kind: 'parseError', copy };
    }
    return { kind: 'error', copy };
  }

  const data = query.data;
  return data === undefined ? { kind: 'loading' } : { kind: 'loaded', name: data.name };
}

interface DetailBodyProps {
  readonly phase: DetailRenderPhase;
  readonly templateId: string;
  readonly onRetry: () => void;
}

function DetailBody({ phase, templateId, onRetry }: DetailBodyProps) {
  switch (phase.kind) {
    case 'loading':
      return <p className="lg-dialog__message">{message('sheet.loading')}</p>;
    case 'loaded':
      return (
        <div className="lg-dialog__template">
          <div className="lg-dialog__template-main">
            <p className="lg-dialog__eyebrow">{message('sheet.templateLabel')}</p>
            <p className="lg-dialog__template-name" data-testid="detail-name">
              {phase.name}
            </p>
          </div>
          <p className="lg-dialog__template-code">{templateId}</p>
        </div>
      );
    case 'error':
    case 'notFound':
    case 'parseError':
      return (
        <>
          <p className="lg-dialog__message">{phase.copy}</p>
          <button type="button" className="lg-button" onClick={onRetry}>
            {message('sheet.retry')}
          </button>
        </>
      );
  }
}

function disabledReason(hasName: boolean, limitReached: boolean): string {
  if (limitReached) {
    return message('sheet.limitReached');
  }
  return hasName ? '' : message('sheet.nameRequired');
}

export interface CreateSheetDialogProps {
  readonly templateId: string;
  readonly onClose: () => void;
}

export function CreateSheetDialog({ templateId, onClose }: CreateSheetDialogProps) {
  const query = useTemplateDetail(templateId);
  const plan = usePlanUsage();
  const client = useApiClient();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [sheetName, setSheetName] = useState('');
  const [kind, setKind] = useState<SheetKind>(DEFAULT_KIND);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);
  const inputRef = useRef<HTMLInputElement>(null);
  const phase = resolveDetailPhase(query);
  const trimmedName = sheetName.trim();
  const hasName = trimmedName !== '';
  const limitReached = isSheetLimitReached(plan.data, kind);
  const kindUsage = plan.data?.sheets[kind];
  const kindDescriptionIds = [
    ...(kindUsage === undefined ? [] : ['sheet-kind-usage']),
    ...(limitReached ? ['sheet-kind-limit'] : []),
  ].join(' ');
  const canSubmit = hasName && !limitReached;

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  const handleCreate = useCallback(() => {
    if (!canSubmit || isSubmitting) {
      return;
    }
    setIsSubmitting(true);
    setSubmitError(undefined);
    void client
      .createSheet({ templateId, kind, sheetName: trimmedName })
      .then((sheet) => {
        seedCreatedSheet(queryClient, sheet);
        void queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
        navigate(`/sheets/${encodeURIComponent(sheet.id)}`);
      })
      .catch((error: unknown) => {
        setSubmitError(describeFailure(error));
        setIsSubmitting(false);
        void queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
      });
  }, [canSubmit, isSubmitting, client, templateId, kind, trimmedName, queryClient, navigate]);

  return (
    <div className="lg-overlay">
      <div
        className="lg-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-sheet-dialog-title"
        data-testid="create-sheet-dialog"
      >
        <div className="lg-dialog__header">
          <h2 className="lg-dialog__title" id="create-sheet-dialog-title">
            {message('sheet.dialogTitle')}
          </h2>
          <button
            type="button"
            className="lg-button lg-button--ghost lg-button--sm"
            onClick={onClose}
          >
            {message('sheet.close')}
          </button>
        </div>
        <div className="lg-dialog__body">
          <DetailBody
            phase={phase}
            templateId={templateId}
            onRetry={() => {
              void query.refetch();
            }}
          />
        </div>
        <div className="lg-dialog__field">
          <label className="lg-dialog__label" htmlFor="sheet-name">
            {message('sheet.nameLabel')}
          </label>
          <input
            id="sheet-name"
            ref={inputRef}
            className="lg-dialog__input"
            type="text"
            value={sheetName}
            placeholder={message('sheet.namePlaceholder')}
            aria-describedby={hasName ? undefined : 'sheet-name-required'}
            onChange={(event) => setSheetName(event.target.value)}
          />
          {!hasName && (
            <p className="lg-dialog__note" id="sheet-name-required">
              {message('sheet.nameRequired')}
            </p>
          )}
        </div>
        <div className="lg-dialog__field">
          <label className="lg-dialog__label" htmlFor="sheet-kind">
            {message('catalog.kindLabel')}
          </label>
          <select
            id="sheet-kind"
            className="lg-dialog__select"
            value={kind}
            aria-describedby={kindDescriptionIds === '' ? undefined : kindDescriptionIds}
            onChange={(event) => {
              setKind(event.target.value as SheetKind);
              setSubmitError(undefined);
            }}
          >
            {SHEET_KINDS.map((option) => (
              <option key={option} value={option}>
                {message(KIND_LABEL_KEYS[option])}
              </option>
            ))}
          </select>
          {kindUsage !== undefined && (
            <p className="lg-dialog__note" id="sheet-kind-usage" data-testid="sheet-usage">
              {messageWith('sheet.usage', {
                used: kindUsage.used,
                limit: kindUsage.limit,
              })}
            </p>
          )}
          {limitReached && (
            <p
              className="lg-dialog__note"
              id="sheet-kind-limit"
              role="status"
              data-testid="sheet-limit-reached"
            >
              {message('sheet.limitReached')}
            </p>
          )}
        </div>
        {submitError !== undefined && (
          <p className="lg-dialog__message" role="alert" data-testid="create-error">
            {submitError}
          </p>
        )}
        <div className="lg-dialog__actions">
          {limitReached ? (
            <button
              type="button"
              className="lg-button lg-button--secondary lg-dialog__generate"
              disabled
              title={message('sheet.limitReached')}
            >
              {message('generate.entry')}
            </button>
          ) : (
            <Link
              className="lg-button lg-button--secondary lg-dialog__generate"
              to={`/generate/${encodeURIComponent(templateId)}?kind=${encodeURIComponent(kind)}`}
              onClick={onClose}
            >
              {message('generate.entry')}
            </Link>
          )}
          <button
            type="button"
            className="lg-button lg-button--primary"
            disabled={!canSubmit || isSubmitting}
            aria-busy={isSubmitting}
            title={canSubmit ? undefined : disabledReason(hasName, limitReached)}
            onClick={handleCreate}
          >
            {isSubmitting ? message('sheet.creating') : message('sheet.create')}
          </button>
        </div>
      </div>
    </div>
  );
}
