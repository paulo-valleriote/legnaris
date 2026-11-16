import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { failureCopy, generationErrorKind } from '../api/error_mapper';
import { DomainFailure } from '../api/errors';
import type { Generation } from '../api/models/generation';
import { useApiClient } from '../app/providers';
import { PLAN_QUERY_KEY, usePlanUsage, useTemplateDetail } from '../catalog/queries';
import { setPathValue } from '../engine/sheet_defaults';
import { SheetForm } from '../engine/sheet_form';
import { PrivacyDialog } from '../legal/privacy_dialog';
import { message, messageWith } from '../l10n/strings';
import { registerLeaveGuard, requestLeave } from '../shell/leave_guard';
import { Button } from '../ui/button';
import { Dialog } from '../ui/dialog';
import {
  POLL_INTERVAL_MS,
  conceptLength,
  conceptState,
  decideWait,
  isLastGeneration,
  mergeCascade,
  previewDelay,
  trialAllowance,
  trimConcept,
} from './generation_flow';
import type { Allowance } from './generation_flow';
import { CONSENT_VERSION, consentDisclosure } from './llm_consent';
import type { ConsentDisclosure } from './llm_consent';

interface PreviewState {
  readonly generationId: string;
  readonly values: Record<string, unknown>;
  readonly edits: Record<string, unknown>;
  readonly revision: number;
  readonly suggestedName: string;
  readonly sheetName: string;
  readonly nameTouched: boolean;
  readonly dirty: boolean;
  readonly accepting: boolean;
  readonly acceptError: string | undefined;
  readonly previewError: string | undefined;
}

type Step =
  | { readonly kind: 'concept' }
  | { readonly kind: 'waiting'; readonly generationId: string; readonly startedAt: number }
  | { readonly kind: 'timedOut'; readonly generationId: string }
  | { readonly kind: 'failed'; readonly copy: string }
  | { readonly kind: 'alreadyAccepted' }
  | { readonly kind: 'error'; readonly copy: string }
  | ({ readonly kind: 'preview' } & PreviewState);

function stepForGeneration(generation: Generation): Step {
  if (generation.status === 'accepted') {
    return { kind: 'alreadyAccepted' };
  }
  if (generation.status === 'failed' || generation.values === undefined) {
    const kind = generationErrorKind(generation.errorCode);
    return { kind: 'failed', copy: failureCopy(new DomainFailure({ kind })) };
  }
  return {
    kind: 'preview',
    generationId: generation.id,
    values: structuredClone(generation.values),
    edits: {},
    revision: 0,
    suggestedName: generation.suggestedName ?? '',
    sheetName: '',
    nameTouched: false,
    dirty: false,
    accepting: false,
    acceptError: undefined,
    previewError: undefined,
  };
}

function generationIdFromState(state: unknown): string | undefined {
  if (typeof state !== 'object' || state === null || !('generationId' in state)) {
    return undefined;
  }
  const id = state.generationId;
  return typeof id === 'string' && id !== '' ? id : undefined;
}

function deltaOf(edits: Record<string, unknown>): Record<string, unknown> {
  const delta: Record<string, unknown> = {};
  for (const [path, value] of Object.entries(edits)) {
    setPathValue(delta, path, value);
  }
  return delta;
}

function applyEdits(
  values: Record<string, unknown>,
  edits: Record<string, unknown>,
): Record<string, unknown> {
  const next = structuredClone(values);
  for (const [path, value] of Object.entries(edits)) {
    setPathValue(next, path, value);
  }
  return next;
}

function ConsentDialog({
  open,
  disclosure,
  busy,
  error,
  onAccept,
  onDecline,
}: {
  readonly open: boolean;
  readonly disclosure: ConsentDisclosure;
  readonly busy: boolean;
  readonly error: string | undefined;
  readonly onAccept: () => void;
  readonly onDecline: () => void;
}) {
  const [privacyOpen, setPrivacyOpen] = useState(false);

  return (
    <>
      <Dialog open={open} title={disclosure.title} onClose={onDecline}>
        <p>{disclosure.intro}</p>
        <dl>
          {disclosure.points.map((point) => (
            <div key={point.heading}>
              <dt>{point.heading}</dt>
              <dd>{point.body}</dd>
            </div>
          ))}
        </dl>
        <button
          type="button"
          className="lg-button lg-button--ghost"
          aria-haspopup="dialog"
          onClick={() => setPrivacyOpen(true)}
        >
          {disclosure.detailsLinkText}
        </button>
        {error !== undefined && <p role="alert">{error}</p>}
        <div>
          <Button
            variant="secondary"
            label={message('generate.consentDecline')}
            onClick={onDecline}
          />
          <Button
            variant="primary"
            label={disclosure.acceptLabel}
            loading={busy}
            onClick={onAccept}
          />
        </div>
      </Dialog>
      {open && privacyOpen ? <PrivacyDialog onClose={() => setPrivacyOpen(false)} /> : null}
    </>
  );
}

function quotaCopy(allowance: Allowance | undefined): string | undefined {
  if (allowance === undefined) {
    return undefined;
  }
  if (allowance.kind === 'unknown') {
    return message('generate.quotaUnknown');
  }
  if (allowance.remaining === 0) {
    return message('generate.quotaNone');
  }
  return messageWith('generate.quotaTrial', { remaining: allowance.remaining });
}

export function GenerateScreen() {
  const { templateId = '' } = useParams();
  const location = useLocation();
  const client = useApiClient();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const plan = usePlanUsage();
  const template = useTemplateDetail(templateId);

  const [step, setStep] = useState<Step>(() => {
    const handedOver = generationIdFromState(location.state);
    return handedOver === undefined
      ? { kind: 'concept' }
      : { kind: 'waiting', generationId: handedOver, startedAt: Date.now() };
  });
  const [concept, setConcept] = useState('');
  const [submitError, setSubmitError] = useState<string | undefined>(undefined);
  const [submitting, setSubmitting] = useState(false);
  const [consentOpen, setConsentOpen] = useState(false);
  const [consentBusy, setConsentBusy] = useState(false);
  const [consentError, setConsentError] = useState<string | undefined>(undefined);
  const [leaveProceed, setLeaveProceed] = useState<(() => void) | undefined>(undefined);
  const [lastConfirmOpen, setLastConfirmOpen] = useState(false);

  const dirtyRef = useRef(false);
  dirtyRef.current = step.kind === 'preview' && step.dirty;
  const latestRef = useRef<Step>(step);
  latestRef.current = step;
  const lastPreviewAt = useRef<number | undefined>(undefined);

  useEffect(
    () =>
      registerLeaveGuard({
        intercepts: () => dirtyRef.current,
        requestLeave: (proceed) => setLeaveProceed(() => proceed),
      }),
    [],
  );

  const waitingId = step.kind === 'waiting' ? step.generationId : undefined;
  const waitingStart = step.kind === 'waiting' ? step.startedAt : undefined;

  useEffect(() => {
    if (waitingId === undefined || waitingStart === undefined) {
      return undefined;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const poll = async () => {
      let generation: Generation;
      try {
        generation = await client.getGeneration(waitingId);
      } catch (error: unknown) {
        if (!cancelled) {
          setStep({ kind: 'error', copy: failureCopy(error) });
        }
        return;
      }
      if (cancelled) {
        return;
      }
      const outcome = decideWait(generation, Date.now() - waitingStart);
      if (outcome === 'keepWaiting') {
        timer = setTimeout(() => void poll(), POLL_INTERVAL_MS);
      } else if (outcome === 'timedOut') {
        setStep({ kind: 'timedOut', generationId: waitingId });
      } else {
        setStep(stepForGeneration(generation));
      }
    };

    void poll();
    return () => {
      cancelled = true;
      if (timer !== undefined) {
        clearTimeout(timer);
      }
    };
  }, [client, waitingId, waitingStart]);

  const previewRevision = step.kind === 'preview' ? step.revision : 0;

  useEffect(() => {
    if (previewRevision === 0) {
      return undefined;
    }
    const timer = setTimeout(
      () => {
        const current = latestRef.current;
        if (current.kind !== 'preview') {
          return;
        }
        const sentRevision = current.revision;
        const delta = deltaOf(current.edits);
        lastPreviewAt.current = Date.now();
        const isCurrent = (state: Step) =>
          state.kind === 'preview' && state.revision === sentRevision;
        client.previewTemplate(templateId, delta).then(
          (cascade) =>
            setStep((state) =>
              isCurrent(state) && state.kind === 'preview'
                ? {
                    ...state,
                    values: applyEdits(mergeCascade(state.values, cascade), state.edits),
                    previewError: undefined,
                  }
                : state,
            ),
          () =>
            setStep((state) =>
              isCurrent(state) && state.kind === 'preview'
                ? { ...state, previewError: message('generate.previewFailed') }
                : state,
            ),
        );
      },
      previewDelay(lastPreviewAt.current, Date.now()),
    );
    return () => clearTimeout(timer);
  }, [client, templateId, previewRevision]);

  function startWaiting(generationId: string) {
    setStep({ kind: 'waiting', generationId, startedAt: Date.now() });
  }

  async function handleSubmitFailure(error: unknown) {
    if (error instanceof DomainFailure && error.kind === 'llmConsentRequired') {
      setConsentOpen(true);
      return;
    }
    if (error instanceof DomainFailure && error.kind === 'generationInProgress') {
      let refreshed;
      try {
        refreshed = await queryClient.fetchQuery({
          queryKey: PLAN_QUERY_KEY,
          queryFn: () => client.getMyPlan(),
          staleTime: 0,
        });
      } catch (refreshError: unknown) {
        setSubmitError(failureCopy(refreshError));
        return;
      }
      if (refreshed.activeGenerationId !== null) {
        startWaiting(refreshed.activeGenerationId);
        return;
      }
    }
    setSubmitError(failureCopy(error));
  }

  async function submitConcept() {
    setSubmitting(true);
    setSubmitError(undefined);
    try {
      const generationId = await client.createGeneration({
        templateId,
        kind: 'character',
        concept: trimConcept(concept),
      });
      setSubmitting(false);
      void queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
      startWaiting(generationId);
    } catch (error: unknown) {
      setSubmitting(false);
      await handleSubmitFailure(error);
    }
  }

  function onGenerate() {
    if (plan.data?.llmConsentRequired === true) {
      setConsentOpen(true);
      return;
    }
    void submitConcept();
  }

  async function onAcceptConsent() {
    setConsentBusy(true);
    setConsentError(undefined);
    try {
      await client.grantLlmConsent(CONSENT_VERSION);
      await queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
      setConsentBusy(false);
      setConsentOpen(false);
      void submitConcept();
    } catch (error: unknown) {
      setConsentBusy(false);
      setConsentError(failureCopy(error));
    }
  }

  function updatePreview(update: (current: PreviewState) => PreviewState) {
    setStep((current) =>
      current.kind === 'preview' ? { kind: 'preview', ...update(current) } : current,
    );
  }

  function onEdit(path: string, value: unknown) {
    updatePreview((current) => {
      const values = structuredClone(current.values);
      setPathValue(values, path, value);
      return {
        ...current,
        values,
        edits: { ...current.edits, [path]: value },
        revision: current.revision + 1,
        dirty: true,
      };
    });
  }

  async function onAccept(preview: PreviewState, name: string) {
    if (name === '') {
      return;
    }
    updatePreview((current) => ({ ...current, accepting: true, acceptError: undefined }));
    try {
      const accepted = await client.acceptGeneration(preview.generationId, {
        sheetName: name,
        values: preview.values,
      });
      await queryClient.invalidateQueries({ queryKey: PLAN_QUERY_KEY });
      navigate(`/sheets/${accepted.sheetId}`);
    } catch (error: unknown) {
      updatePreview((current) => ({
        ...current,
        accepting: false,
        acceptError: failureCopy(error),
      }));
    }
  }

  function toConcept() {
    setStep({ kind: 'concept' });
  }

  function onRegenerate() {
    if (isLastGeneration(plan.data)) {
      setLastConfirmOpen(true);
      return;
    }
    requestLeave(toConcept);
  }

  function confirmLastRegenerate() {
    setLastConfirmOpen(false);
    requestLeave(toConcept);
  }

  const allowance = plan.data === undefined ? undefined : trialAllowance(plan.data);
  const state = conceptState(concept);
  const length = conceptLength(concept);
  const canSubmit = state === 'ok' && !submitting;
  const noAllowance = allowance?.kind === 'trial' && allowance.remaining === 0;
  const activeId = plan.data?.activeGenerationId ?? null;

  return (
    <main className="lg-generate">
      <h1>{message('generate.title')}</h1>
      {step.kind === 'concept' && (
        <section aria-label={message('generate.title')}>
          {activeId !== null && (
            <div role="status">
              <p>{message('generate.resumeNotice')}</p>
              <Button
                variant="secondary"
                label={message('generate.resumeOpen')}
                onClick={() => startWaiting(activeId)}
              />
            </div>
          )}
          <label htmlFor="generate-concept">{message('generate.conceptLabel')}</label>
          <textarea
            id="generate-concept"
            value={concept}
            onChange={(event) => setConcept(event.target.value)}
            aria-describedby="generate-concept-hint"
          />
          <p id="generate-concept-hint">{message('generate.conceptExample')}</p>
          <p>{message('generate.languageNote')}</p>
          <p>
            {messageWith('generate.counter', { count: length })}
            {state === 'tooShort' && length > 0 && ` ${message('generate.tooShort')}`}
            {state === 'tooLong' && ` ${message('generate.tooLong')}`}
          </p>
          {quotaCopy(allowance) !== undefined && <p>{quotaCopy(allowance)}</p>}
          {submitError !== undefined && <p role="alert">{submitError}</p>}
          <Button
            variant="primary"
            label={submitting ? message('generate.submitting') : message('generate.submit')}
            disabled={!canSubmit || noAllowance}
            loading={submitting}
            onClick={onGenerate}
          />
        </section>
      )}
      {step.kind === 'waiting' && (
        <section role="status">
          <p>{message('generate.waiting')}</p>
          <Button
            variant="ghost"
            label={message('generate.backToCatalog')}
            onClick={() => navigate('/')}
          />
        </section>
      )}
      {step.kind === 'timedOut' && (
        <section role="status">
          <p>{message('generate.timedOut')}</p>
          <Button
            variant="secondary"
            label={message('generate.keepWaiting')}
            onClick={() => startWaiting(step.generationId)}
          />
          <Button
            variant="ghost"
            label={message('generate.backToCatalog')}
            onClick={() => navigate('/')}
          />
        </section>
      )}
      {step.kind === 'failed' && (
        <section role="alert">
          <p>{step.copy}</p>
          <Button variant="primary" label={message('sheet.retry')} onClick={toConcept} />
        </section>
      )}
      {step.kind === 'alreadyAccepted' && (
        <section role="status">
          <p>{message('generate.alreadySaved')}</p>
          <Button
            variant="ghost"
            label={message('generate.backToCatalog')}
            onClick={() => navigate('/')}
          />
        </section>
      )}
      {step.kind === 'error' && (
        <section role="alert">
          <p>{step.copy}</p>
          <Button
            variant="ghost"
            label={message('generate.backToCatalog')}
            onClick={() => navigate('/')}
          />
        </section>
      )}
      {step.kind === 'preview' && (
        <ReviewSection
          preview={step}
          fields={template.data?.fields}
          allowance={allowance}
          onEdit={onEdit}
          onName={(name) =>
            updatePreview((current) => ({ ...current, sheetName: name, nameTouched: true }))
          }
          onAccept={(name) => void onAccept(step, name)}
          onRegenerate={onRegenerate}
        />
      )}
      {consentOpen && (
        <ConsentDialog
          open={consentOpen}
          disclosure={consentDisclosure()}
          busy={consentBusy}
          error={consentError}
          onAccept={() => void onAcceptConsent()}
          onDecline={() => {
            setConsentOpen(false);
            setConsentError(undefined);
          }}
        />
      )}
      <Dialog
        open={lastConfirmOpen}
        title={message('generate.lastTitle')}
        onClose={() => setLastConfirmOpen(false)}
      >
        <p>{message('generate.lastBody')}</p>
        <Button
          variant="secondary"
          label={message('generate.lastKeep')}
          onClick={() => setLastConfirmOpen(false)}
        />
        <Button
          variant="primary"
          label={message('generate.lastConfirm')}
          onClick={confirmLastRegenerate}
        />
      </Dialog>
      <Dialog
        open={leaveProceed !== undefined}
        title={message('generate.leaveTitle')}
        onClose={() => setLeaveProceed(undefined)}
      >
        <p>{message('generate.leaveBody')}</p>
        <Button
          variant="primary"
          label={message('generate.leaveStay')}
          onClick={() => setLeaveProceed(undefined)}
        />
        <Button
          variant="danger"
          label={message('generate.leaveConfirm')}
          onClick={() => {
            const proceed = leaveProceed;
            setLeaveProceed(undefined);
            proceed?.();
          }}
        />
      </Dialog>
    </main>
  );
}

function ReviewSection({
  preview,
  fields,
  allowance,
  onEdit,
  onName,
  onAccept,
  onRegenerate,
}: {
  readonly preview: PreviewState;
  readonly fields: Parameters<typeof SheetForm>[0]['fields'] | undefined;
  readonly allowance: Allowance | undefined;
  readonly onEdit: (path: string, value: unknown) => void;
  readonly onName: (name: string) => void;
  readonly onAccept: (name: string) => void;
  readonly onRegenerate: () => void;
}) {
  const name = preview.nameTouched ? preview.sheetName : preview.suggestedName;
  const trimmed = name.trim();
  const remaining = allowance?.kind === 'trial' ? allowance.remaining : undefined;

  return (
    <section aria-label={message('generate.previewTitle')}>
      <h2>{message('generate.previewTitle')}</h2>
      <label htmlFor="generate-sheet-name">{message('sheet.nameLabel')}</label>
      <input
        id="generate-sheet-name"
        value={name}
        onChange={(event) => onName(event.target.value)}
        aria-required="true"
      />
      {fields !== undefined && (
        <SheetForm fields={fields} data={preview.values} onChanged={onEdit} />
      )}
      {preview.previewError !== undefined && <p role="status">{preview.previewError}</p>}
      {preview.acceptError !== undefined && <p role="alert">{preview.acceptError}</p>}
      <Button
        variant="primary"
        label={preview.accepting ? message('generate.accepting') : message('generate.accept')}
        disabled={trimmed === '' || preview.accepting}
        loading={preview.accepting}
        onClick={() => onAccept(trimmed)}
      />
      <Button
        variant="secondary"
        label={
          remaining === undefined
            ? message('generate.regenerateNone')
            : messageWith('generate.regenerate', { remaining })
        }
        disabled={remaining === 0}
        onClick={onRegenerate}
      />
    </section>
  );
}
