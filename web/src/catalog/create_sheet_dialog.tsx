import { useRef, useState } from 'react';
import type { UseQueryResult } from '@tanstack/react-query';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import { failureCopy } from '../api/error_mapper';
import type { TemplateDetail } from '../api/models/template';
import { useApiClient } from '../app/providers';
import { isSheetLimitReached } from '../api/models/plan';
import type { SheetKind } from '../api/models/plan';
import { isFreePlan } from '../generate/generation_flow';
import { UpgradeDialog } from '../generate/upgrade_dialog';
import type { MessageKey } from '../l10n/strings';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import { Field } from '../ui/field';
import { Notice } from '../ui/notice';
import { Select } from '../ui/select';
import { ErrorState, LoadingState } from '../ui/state';
import { PLAN_QUERY_KEY, seedCreatedSheet, usePlanUsage, useTemplateDetail } from './queries';

const LIMIT_NOTICE_ID = 'sheet-kind-limit';

const KIND_LABEL_KEYS = {
  character: 'catalog.kindCharacter',
  npc: 'kind.npc',
  enemy: 'kind.enemy',
} as const satisfies Record<SheetKind, MessageKey>;

type DetailRenderPhase =
  | { readonly kind: 'loading' }
  | { readonly kind: 'loaded'; readonly name: string; readonly kinds: readonly SheetKind[] }
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
  return data === undefined
    ? { kind: 'loading' }
    : { kind: 'loaded', name: data.name, kinds: data.kinds };
}

interface DetailBodyProps {
  readonly phase: DetailRenderPhase;
  readonly templateId: string;
  readonly onRetry: () => void;
}

function DetailBody({ phase, templateId, onRetry }: DetailBodyProps) {
  switch (phase.kind) {
    case 'loading':
      return <LoadingState label={message('sheet.loading')} />;
    case 'loaded':
      return (
        <div className="lg-create-sheet__template">
          <div className="lg-create-sheet__template-main">
            <p className="lg-create-sheet__eyebrow">{message('sheet.templateLabel')}</p>
            <p className="lg-create-sheet__name" data-testid="detail-name">
              {phase.name}
            </p>
          </div>
          <p className="lg-create-sheet__code">{templateId}</p>
        </div>
      );
    case 'error':
    case 'notFound':
    case 'parseError':
      return <ErrorState message={phase.copy} onRetry={onRetry} />;
  }
}

function blockedReason(hasName: boolean, limitReached: boolean): string | undefined {
  if (limitReached) {
    return message('sheet.limitReached');
  }
  return hasName ? undefined : message('sheet.nameRequired');
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
  const [chosenKind, setChosenKind] = useState<SheetKind | undefined>(undefined);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const phase = resolveDetailPhase(query);
  const trimmedName = sheetName.trim();
  const hasName = trimmedName !== '';
  const availableKinds = phase.kind === 'loaded' ? phase.kinds : [];
  const kind = availableKinds.find((candidate) => candidate === chosenKind) ?? availableKinds[0];
  const limitReached = kind !== undefined && isSheetLimitReached(plan.data, kind);
  const kindUsage = kind === undefined ? undefined : plan.data?.sheets[kind];
  const kindHint =
    kindUsage === undefined
      ? undefined
      : messageWith('sheet.usage', { used: kindUsage.used, limit: kindUsage.limit });
  const canSubmit = kind !== undefined && hasName && !limitReached;
  const reason = canSubmit ? undefined : blockedReason(hasName, limitReached);

  const handleCreate = () => {
    if (kind === undefined || !canSubmit || isSubmitting) {
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
  };

  const generateTo =
    kind === undefined
      ? undefined
      : `/generate/${encodeURIComponent(templateId)}?kind=${encodeURIComponent(kind)}`;
  const generateLabel = message('generate.entry');
  const generate =
    kind === undefined ? (
      <Button variant="secondary" label={generateLabel} disabled />
    ) : limitReached ? (
      <Button
        variant="secondary"
        label={generateLabel}
        disabled
        ariaDescribedBy={LIMIT_NOTICE_ID}
      />
    ) : isFreePlan(plan.data) ? (
      <Button variant="secondary" name={generateLabel} onClick={() => setUpgradeOpen(true)}>
        <span className="lg-create-sheet__generate">
          <span>{generateLabel}</span>
          <span className="lg-create-sheet__pro">{message('generate.upgrade.badge')}</span>
        </span>
      </Button>
    ) : (
      <Button variant="secondary" label={generateLabel} to={generateTo} />
    );

  return (
    <>
      <Dialog
        open
        title={message('sheet.dialogTitle')}
        onClose={onClose}
        closeLabel={message('sheet.close')}
        initialFocusRef={inputRef}
        actions={
          <>
            {generate}
            <Button
              variant="primary"
              label={isSubmitting ? message('sheet.creating') : message('sheet.create')}
              disabled={!canSubmit || isSubmitting}
              loading={isSubmitting}
              disabledReason={isSubmitting || limitReached ? undefined : reason}
              ariaDescribedBy={limitReached ? LIMIT_NOTICE_ID : undefined}
              onClick={handleCreate}
            />
          </>
        }
      >
        <div className="lg-create-sheet">
          <DetailBody
            phase={phase}
            templateId={templateId}
            onRetry={() => {
              void query.refetch();
            }}
          />
          <Field
            label={message('sheet.nameLabel')}
            id="sheet-name"
            inputRef={inputRef}
            value={sheetName}
            placeholder={message('sheet.namePlaceholder')}
            onChange={setSheetName}
          />
          {kind === undefined ? null : availableKinds.length === 1 ? (
            <div
              className="lg-form-field lg-create-sheet__kind"
              role="group"
              aria-labelledby="sheet-kind-label"
              aria-describedby={kindHint === undefined ? undefined : 'sheet-kind-hint'}
            >
              <span className="lg-form-field__label" id="sheet-kind-label">
                {message('catalog.kindLabel')}
              </span>
              <span className="lg-create-sheet__kind-value">{message(KIND_LABEL_KEYS[kind])}</span>
              {kindHint === undefined ? null : (
                <span id="sheet-kind-hint" className="lg-form-field__hint">
                  {kindHint}
                </span>
              )}
            </div>
          ) : (
            <Select
              label={message('catalog.kindLabel')}
              id="sheet-kind"
              value={kind}
              hint={kindHint}
              onChange={(value) => {
                setChosenKind(value as SheetKind);
                setSubmitError(undefined);
              }}
            >
              {availableKinds.map((option) => (
                <option key={option} value={option}>
                  {message(KIND_LABEL_KEYS[option])}
                </option>
              ))}
            </Select>
          )}
          {limitReached ? (
            <Notice tone="warning" live="off" id={LIMIT_NOTICE_ID}>
              {message('sheet.limitReached')}
            </Notice>
          ) : null}
          {submitError !== undefined && (
            <p className="lg-create-sheet__error" role="alert" data-testid="create-error">
              {submitError}
            </p>
          )}
        </div>
      </Dialog>
      <UpgradeDialog open={upgradeOpen} onClose={() => setUpgradeOpen(false)} />
    </>
  );
}
