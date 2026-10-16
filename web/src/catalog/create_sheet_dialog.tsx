/**
 * The creation surface for one template (WCT-11 … WCT-25, WCT-27).
 *
 * An accessible overlay over the catalog: it loads the template's detail inline,
 * carries the sheet-name field with its trim/required validation, and issues no
 * mutation — creating the sheet belongs to a later feature.
 */

import { useEffect, useRef, useState } from 'react';
import type { UseQueryResult } from '@tanstack/react-query';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import type { TemplateDetail } from '../api/models/template';
import { message } from '../l10n/strings';
import { useTemplateDetail } from './queries';

/** The single situation the surface's detail area renders (WCT-12 … WCT-18). */
type DetailRenderPhase =
  | { readonly kind: 'loading' }
  | { readonly kind: 'loaded'; readonly name: string }
  | { readonly kind: 'error'; readonly copy: string }
  | { readonly kind: 'notFound'; readonly copy: string }
  | { readonly kind: 'parseError'; readonly copy: string };

function resolveDetailPhase(query: UseQueryResult<TemplateDetail, Error>): DetailRenderPhase {
  if (query.isPending) {
    return { kind: 'loading' };
  }

  if (query.isError) {
    if (query.error instanceof DefensiveParseFailure) {
      return { kind: 'parseError', copy: parseFailureMessage() };
    }
    if (query.error instanceof DomainFailure && query.error.kind === 'notFound') {
      return { kind: 'notFound', copy: failureMessage('notFound') };
    }
    return {
      kind: 'error',
      copy: failureMessage(query.error instanceof DomainFailure ? query.error.kind : 'unknown'),
    };
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
  const [sheetName, setSheetName] = useState('');
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
        <div className="lg-dialog__actions">
          <button
            type="button"
            className="lg-button lg-button--primary"
            disabled={!canSubmit}
            title={canSubmit ? undefined : message('sheet.nameRequired')}
          >
            {message('sheet.create')}
          </button>
        </div>
      </div>
    </div>
  );
}
