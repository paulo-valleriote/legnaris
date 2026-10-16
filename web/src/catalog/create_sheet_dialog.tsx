/**
 * The creation surface for one template (WCT-11 … WCT-25, WCT-27, WSC-01 … WSC-04).
 *
 * An accessible overlay over the catalog: it loads the template's detail inline,
 * carries the sheet-name field with its trim/required validation, and issues one
 * `POST /sheets` on the primary action, seeding the created sheet and navigating
 * to it (WSC-01, WSC-02).
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import type { UseQueryResult } from '@tanstack/react-query';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { failureCopy } from '../api/error_mapper';
import type { TemplateDetail } from '../api/models/template';
import { useApiClient } from '../app/providers';
import { message } from '../l10n/strings';
import { seedCreatedSheet, useTemplateDetail } from './queries';

/** The single situation the surface's detail area renders (WCT-12 … WCT-18). */
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
  readonly onRetry: () => void;
}

function DetailBody({ phase, onRetry }: DetailBodyProps) {
  switch (phase.kind) {
    case 'loading':
      return <p className="lg-dialog__message">{message('sheet.loading')}</p>;
    case 'loaded':
      return (
        <p className="lg-dialog__message" data-testid="detail-name">
          {phase.name}
        </p>
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

export interface CreateSheetDialogProps {
  readonly templateId: string;
  readonly onClose: () => void;
}

export function CreateSheetDialog({ templateId, onClose }: CreateSheetDialogProps) {
  const query = useTemplateDetail(templateId);
  const client = useApiClient();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [sheetName, setSheetName] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);
  const inputRef = useRef<HTMLInputElement>(null);
  const phase = resolveDetailPhase(query);
  const trimmedName = sheetName.trim();
  const canSubmit = trimmedName !== '';

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
      .createSheet({ templateId, sheetName: trimmedName })
      .then((sheet) => {
        seedCreatedSheet(queryClient, sheet);
        navigate(`/sheets/${encodeURIComponent(sheet.id)}`);
      })
      .catch((error: unknown) => {
        setSubmitError(describeFailure(error));
        setIsSubmitting(false);
      });
  }, [canSubmit, isSubmitting, client, templateId, trimmedName, queryClient, navigate]);

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
          <button type="button" className="lg-button" onClick={onClose}>
            {message('sheet.close')}
          </button>
        </div>
        <div className="lg-dialog__body">
          <DetailBody
            phase={phase}
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
            aria-describedby={canSubmit ? undefined : 'sheet-name-required'}
            onChange={(event) => setSheetName(event.target.value)}
          />
          {!canSubmit && (
            <p className="lg-dialog__note" id="sheet-name-required">
              {message('sheet.nameRequired')}
            </p>
          )}
        </div>
        {submitError !== undefined && (
          <p className="lg-dialog__message" role="alert" data-testid="create-error">
            {submitError}
          </p>
        )}
        <div className="lg-dialog__actions">
          <button
            type="button"
            className="lg-button lg-button--primary"
            disabled={!canSubmit || isSubmitting}
            aria-busy={isSubmitting}
            title={canSubmit ? undefined : message('sheet.nameRequired')}
            onClick={handleCreate}
          >
            {isSubmitting ? message('sheet.creating') : message('sheet.create')}
          </button>
        </div>
      </div>
    </div>
  );
}
