/**
 * PROTOTYPE (LGN-177) — throwaway, never merged. Walks the decided generation UX (concept → wait →
 * preview/accept) against a fake API. `?scenario=` picks the starting account/outcome; the bottom
 * bar cycles scenarios (← →) and shows the full prototype state. Strings are inline on purpose.
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { SheetForm } from '../../engine/sheet_form';
import { Button } from '../../ui/button';
import { Dialog } from '../../ui/dialog';
import { FAKE_TEMPLATE, generatedValues, recalculate, withValue } from './fake_generation';
import './generate_prototype.css';

type Plan = 'free' | 'paid';
type Outcome = 'success' | 'failed' | 'provider_unavailable' | 'slow';
type Step = 'concept' | 'waiting' | 'timeout' | 'preview' | 'failed' | 'confirming' | 'accepted';
type Modal = null | 'consent' | 'upsell' | 'regenConfirm' | 'leave';
type Data = Record<string, Record<string, unknown>>;

interface Scenario {
  readonly key: string;
  readonly name: string;
  readonly plan: Plan;
  readonly remaining: number;
  readonly consentRequired: boolean;
  readonly outcome: Outcome;
  readonly step?: Step;
  readonly pendingNotice?: boolean;
  readonly sheetLimitOnAccept?: boolean;
}

const SCENARIOS: readonly Scenario[] = [
  {
    key: 'free-first',
    name: 'Free, 3 left, no consent yet',
    plan: 'free',
    remaining: 3,
    consentRequired: true,
    outcome: 'success',
  },
  {
    key: 'free-last',
    name: 'Free, 1 left (last one)',
    plan: 'free',
    remaining: 1,
    consentRequired: false,
    outcome: 'success',
  },
  {
    key: 'free-zero',
    name: 'Free, allowance exhausted / anti-abuse',
    plan: 'free',
    remaining: 0,
    consentRequired: false,
    outcome: 'success',
  },
  {
    key: 'paid',
    name: 'Paid, 30 left today',
    plan: 'paid',
    remaining: 30,
    consentRequired: false,
    outcome: 'success',
  },
  {
    key: 'paid-zero',
    name: 'Paid, daily quota used',
    plan: 'paid',
    remaining: 0,
    consentRequired: false,
    outcome: 'success',
  },
  {
    key: 'failed',
    name: 'Generation fails (validation/refusal)',
    plan: 'paid',
    remaining: 5,
    consentRequired: false,
    outcome: 'failed',
  },
  {
    key: 'provider',
    name: 'Provider unavailable',
    plan: 'paid',
    remaining: 5,
    consentRequired: false,
    outcome: 'provider_unavailable',
  },
  {
    key: 'timeout',
    name: 'Slow job → client timeout',
    plan: 'paid',
    remaining: 5,
    consentRequired: false,
    outcome: 'slow',
  },
  {
    key: 'resume',
    name: 'Pending generation on another device',
    plan: 'paid',
    remaining: 4,
    consentRequired: false,
    outcome: 'success',
    pendingNotice: true,
  },
  {
    key: 'limit',
    name: 'Sheet limit hit on accept (409)',
    plan: 'free',
    remaining: 2,
    consentRequired: false,
    outcome: 'success',
    sheetLimitOnAccept: true,
  },
];

const WAIT_MS = 3000;
const TIMEOUT_MS = 6000;
const PREVIEW_DEBOUNCE_MS = 400;
const MIN_CONCEPT = 10;
const MAX_CONCEPT = 1000;
const RESET_LOCAL = new Date(Date.UTC(2026, 10, 14, 0, 0)).toLocaleTimeString([], {
  hour: '2-digit',
  minute: '2-digit',
});

function scenarioFor(key: string | null): Scenario {
  return SCENARIOS.find((s) => s.key === key) ?? (SCENARIOS[0] as Scenario);
}

/** PROTOTYPE route screen for LGN-177. */
export function GeneratePrototype() {
  const [params, setParams] = useSearchParams();
  const scenario = scenarioFor(params.get('scenario'));
  return (
    <Flow key={scenario.key} scenario={scenario} onScenario={(k) => setParams({ scenario: k })} />
  );
}

function Flow({ scenario, onScenario }: { scenario: Scenario; onScenario: (key: string) => void }) {
  const navigate = useNavigate();
  const [plan, setPlan] = useState<Plan>(scenario.plan);
  const [remaining, setRemaining] = useState(scenario.remaining);
  const [consentRequired, setConsentRequired] = useState(scenario.consentRequired);
  const [step, setStep] = useState<Step>(
    scenario.pendingNotice ? 'concept' : (scenario.step ?? 'concept'),
  );
  const [pendingNotice, setPendingNotice] = useState(Boolean(scenario.pendingNotice));
  const [modal, setModal] = useState<Modal>(null);
  const [concept, setConcept] = useState('');
  const [failure, setFailure] = useState<Outcome | null>(null);
  const [generated, setGenerated] = useState<Data | null>(null);
  const [values, setValues] = useState<Data | null>(null);
  const [sheetName, setSheetName] = useState('');
  const [recalculating, setRecalculating] = useState(false);
  const [acceptError, setAcceptError] = useState<string | null>(null);
  const [billingError, setBillingError] = useState(false);
  const [log, setLog] = useState<string[]>([]);
  const timers = useRef<number[]>([]);

  const trace = useCallback((line: string) => setLog((l) => [...l.slice(-7), line]), []);
  const later = (ms: number, fn: () => void) => {
    timers.current.push(window.setTimeout(fn, ms));
  };
  useEffect(() => () => timers.current.forEach((t) => window.clearTimeout(t)), []);

  const edited =
    generated !== null && values !== null && JSON.stringify(generated) !== JSON.stringify(values);
  const conceptValid = concept.trim().length >= MIN_CONCEPT && concept.length <= MAX_CONCEPT;
  const quotaLine =
    plan === 'free'
      ? `${remaining} of 3 free generations left`
      : remaining > 0
        ? `${remaining} left today · resets at ${RESET_LOCAL}`
        : `No generations left today · resets at ${RESET_LOCAL}`;

  const finish = (outcome: Outcome) => {
    if (outcome === 'success') {
      const v = generatedValues();
      setGenerated(v);
      setValues(v);
      setSheetName(String(v.identity?.characterName ?? ''));
      setStep('preview');
      trace('GET /generations/{id} → succeeded');
      return;
    }
    setRemaining((r) => r + 1);
    setFailure(outcome);
    setStep('failed');
    trace(`GET /generations/{id} → failed (${outcome}); quota refunded`);
  };

  const startGeneration = () => {
    setModal(null);
    setRemaining((r) => r - 1);
    setFailure(null);
    setStep('waiting');
    trace('POST /generations → 202 {id}; polling every 5 s');
    if (scenario.outcome === 'slow') {
      later(TIMEOUT_MS, () => {
        setStep((s) => (s === 'waiting' ? 'timeout' : s));
        trace('client gave up polling at 120 s');
      });
      return;
    }
    later(WAIT_MS, () => finish(scenario.outcome));
  };

  const onGenerate = () => {
    if (remaining <= 0) {
      if (plan === 'free') {
        trace('PLAN_UPGRADE_REQUIRED 403 → upsell');
        setModal('upsell');
      }
      return;
    }
    if (consentRequired) {
      trace('consent required → modal');
      setModal('consent');
      return;
    }
    startGeneration();
  };

  const onRegenerate = () => {
    if (remaining <= 0) {
      onGenerate();
      return;
    }
    if (remaining === 1 || edited) {
      setModal('regenConfirm');
      return;
    }
    setStep('concept');
    trace('back to concept (prefilled) to regenerate');
  };

  const onEdit = (path: string, value: unknown) => {
    setValues((v) => (v ? withValue(v, path, value) : v));
    setRecalculating(true);
    later(PREVIEW_DEBOUNCE_MS, () => {
      setValues((v) => (v ? recalculate(v) : v));
      setRecalculating(false);
      trace('POST /templates/fantasy_v1/preview → cascade (throttled)');
    });
  };

  const onAccept = () => {
    if (!sheetName.trim()) {
      setAcceptError('Give the Sheet a name.');
      return;
    }
    if (scenario.sheetLimitOnAccept) {
      setAcceptError(null);
      trace('POST /generations/{id}/accept → SHEET_LIMIT_REACHED 409');
      setModal('upsell');
      return;
    }
    trace('POST /generations/{id}/accept → 200 {sheet_id}; navigate /sheets/:id');
    setStep('accepted');
  };

  const onSubscribe = () => {
    if (!billingError && scenario.key === 'paid-zero') {
      setBillingError(true);
      trace('POST /billing/checkout → BILLING_UNAVAILABLE 503');
      return;
    }
    setModal(null);
    trace('POST /billing/checkout → redirect to Stripe… returned');
    setStep('confirming');
    later(3000, () => {
      setPlan('paid');
      setRemaining(30);
      setStep(generated ? 'preview' : 'concept');
      trace('GET /me/plan → paid (webhook landed); concept restored from sessionStorage');
    });
  };

  const backToCatalog = () => {
    if (step === 'preview' && edited) {
      setModal('leave');
      return;
    }
    trace('navigate / (generation kept server-side for 24 h)');
    navigate('/prototype/generate?scenario=resume');
  };

  return (
    <div className="proto-gen">
      {pendingNotice && (
        <div className="proto-gen__notice" role="status">
          <span>A generated character is waiting for you.</span>
          <Button
            variant="secondary"
            size="sm"
            label="Open"
            onClick={() => {
              setPendingNotice(false);
              finish('success');
              trace('active_generation_id from GET /me/plan → resume');
            }}
          />
        </div>
      )}

      <header className="proto-gen__header">
        <p className="proto-gen__eyebrow">Fantasy · Generate with AI</p>
        <h1 className="proto-gen__title">{titleFor(step)}</h1>
      </header>

      {step === 'concept' && (
        <section className="proto-gen__panel">
          <label className="lg-field proto-gen__concept" htmlFor="concept">
            <span className="lg-field__label">Describe your character</span>
            <textarea
              id="concept"
              className="lg-field__input proto-gen__textarea"
              value={concept}
              maxLength={MAX_CONCEPT + 50}
              rows={6}
              placeholder="A disgraced knight who now guards a mountain pass, sarcastic but loyal, bad with money."
              onChange={(e) => setConcept(e.target.value)}
            />
            <span className={`proto-gen__counter ${concept.length > MAX_CONCEPT ? 'is-over' : ''}`}>
              {concept.length}/{MAX_CONCEPT}
              {concept.trim().length > 0 && concept.trim().length < MIN_CONCEPT
                ? ` · at least ${MIN_CONCEPT}`
                : ''}
            </span>
          </label>
          <p className="proto-gen__hint">
            Write in any language — the character comes back in the same one.
          </p>
          <div className="proto-gen__actions">
            <Button
              label={remaining > 0 || plan === 'free' ? 'Generate' : `Available at ${RESET_LOCAL}`}
              disabled={!conceptValid || (plan === 'paid' && remaining <= 0)}
              onClick={onGenerate}
            />
            <span className="proto-gen__quota">{quotaLine}</span>
          </div>
        </section>
      )}

      {step === 'waiting' && (
        <section className="proto-gen__panel" aria-busy="true">
          <p className="proto-gen__line">
            <span className="proto-gen__pulse" aria-hidden="true" />
            Generating your character — this takes up to a minute. You can leave; it will be ready
            when you come back.
          </p>
          <Button variant="secondary" label="Back to catalog" onClick={backToCatalog} />
        </section>
      )}

      {step === 'timeout' && (
        <section className="proto-gen__panel" role="alert">
          <p className="proto-gen__line">
            This is taking longer than usual. It may still finish — you can keep waiting or come
            back later.
          </p>
          <div className="proto-gen__actions">
            <Button
              label="Keep waiting"
              onClick={() => {
                setStep('waiting');
                trace('polling resumed');
                later(WAIT_MS, () => finish('success'));
              }}
            />
            <Button variant="ghost" label="Back to catalog" onClick={backToCatalog} />
          </div>
        </section>
      )}

      {step === 'failed' && (
        <section className="proto-gen__panel" role="alert">
          <p className="proto-gen__line">
            {failure === 'provider_unavailable'
              ? 'The generation service is unavailable right now. Your generation was refunded — try again later.'
              : "We couldn't generate this character. Your generation was refunded."}
          </p>
          <div className="proto-gen__actions">
            <Button label="Edit concept and try again" onClick={() => setStep('concept')} />
            <span className="proto-gen__quota">{quotaLine}</span>
          </div>
        </section>
      )}

      {step === 'confirming' && (
        <section className="proto-gen__panel" aria-busy="true">
          <p className="proto-gen__line">
            <span className="proto-gen__pulse" aria-hidden="true" />
            Confirming your subscription…
          </p>
        </section>
      )}

      {step === 'accepted' && (
        <section className="proto-gen__panel">
          <p className="proto-gen__line">
            (Prototype) Sheet “{sheetName}” created → would navigate to <code>/sheets/:id</code>.
          </p>
          <Button
            variant="secondary"
            label="Restart scenario"
            onClick={() => onScenario(scenario.key)}
          />
        </section>
      )}

      {step === 'preview' && values && (
        <section className="proto-gen__preview">
          <div className="proto-gen__accept">
            <label className="lg-field" htmlFor="sheet-name">
              <span className="lg-field__label">Sheet name</span>
              <input
                id="sheet-name"
                className={`lg-field__input ${acceptError ? 'lg-field--invalid' : ''}`}
                value={sheetName}
                onChange={(e) => setSheetName(e.target.value)}
              />
              {acceptError && <span className="lg-field__error">{acceptError}</span>}
            </label>
            <div className="proto-gen__actions">
              <Button label="Create Sheet" loading={recalculating} onClick={onAccept} />
              <Button
                variant="secondary"
                label={
                  remaining > 0
                    ? `Regenerate · ${plan === 'free' ? `${remaining} free left` : `${remaining} left today`}`
                    : 'Regenerate'
                }
                disabled={plan === 'paid' && remaining <= 0}
                onClick={onRegenerate}
              />
              <Button variant="ghost" label="Back to catalog" onClick={backToCatalog} />
            </div>
            <p className="proto-gen__hint">
              Edits here are free and recalculate derived values.{' '}
              {edited ? 'You have unsaved edits.' : ''}
            </p>
          </div>
          <SheetForm fields={FAKE_TEMPLATE.fields} data={values} onChanged={onEdit} />
        </section>
      )}

      <Dialog
        open={modal === 'consent'}
        title="Before you generate"
        description="To create your character, Legnaris sends your concept text to an AI provider."
        onClose={() => setModal(null)}
      >
        <ul className="proto-gen__list">
          <li>Gateway: OpenCode. Model: DeepSeek, hosted in the US/EU.</li>
          <li>
            Your concept and the result stay in Legnaris for at most 24 hours unless you keep the
            Sheet.
          </li>
          <li>The provider states it does not keep or train on your data.</li>
          <li>You can withdraw this consent at any time from your account.</li>
        </ul>
        <p className="proto-gen__hint">
          <a href="#privacy">Privacy policy</a>
        </p>
        <div className="proto-gen__actions">
          <AutoFocusSafe label="Not now" onClick={() => setModal(null)} />
          <Button
            label="Accept and generate"
            onClick={() => {
              setConsentRequired(false);
              trace('PUT /me/llm-consent {version} → 204');
              startGeneration();
            }}
          />
        </div>
      </Dialog>

      <Dialog
        open={modal === 'upsell'}
        title={
          scenario.sheetLimitOnAccept
            ? 'Your Sheet slots are full'
            : 'Keep generating with Legnaris Plus'
        }
        description={
          scenario.sheetLimitOnAccept
            ? 'Free accounts hold 1 character. Subscribe for 10, or remove a Sheet first. This generation stays available for 24 hours.'
            : 'You have used your free generations. Plus gives you 30 generations a day and up to 10 characters.'
        }
        onClose={() => {
          setModal(null);
          setBillingError(false);
        }}
      >
        <p className="proto-gen__price">USD 6.99 / month · cancel anytime</p>
        {billingError && (
          <p className="lg-field__error" role="alert">
            Checkout is unavailable right now. Try again in a few minutes.
          </p>
        )}
        <div className="proto-gen__actions">
          <AutoFocusSafe label="Not now" onClick={() => setModal(null)} />
          <Button label="Subscribe" onClick={onSubscribe} />
        </div>
      </Dialog>

      <Dialog
        open={modal === 'regenConfirm'}
        title="Regenerate?"
        description={[
          remaining === 1
            ? 'This uses your last generation' + (plan === 'free' ? '.' : ' for today.')
            : 'This uses 1 generation.',
          edited ? 'Your edits to this preview will be lost.' : '',
        ].join(' ')}
        onClose={() => setModal(null)}
      >
        <div className="proto-gen__actions">
          <AutoFocusSafe label="Cancel" onClick={() => setModal(null)} />
          <Button
            label="Regenerate"
            onClick={() => {
              setStep('concept');
              setModal(null);
              trace('back to concept (prefilled) to regenerate');
            }}
          />
        </div>
      </Dialog>

      <Dialog
        open={modal === 'leave'}
        title="Leave this preview?"
        description="Your changes will be lost. The generated character stays available for 24 hours."
        onClose={() => setModal(null)}
      >
        <div className="proto-gen__actions">
          <AutoFocusSafe label="Stay" onClick={() => setModal(null)} />
          <Button
            variant="danger"
            label="Leave"
            onClick={() => {
              setModal(null);
              navigate('/prototype/generate?scenario=resume');
            }}
          />
        </div>
      </Dialog>

      <ScenarioBar
        current={scenario}
        onScenario={onScenario}
        state={{
          step,
          modal,
          plan,
          remaining,
          consentRequired,
          edited,
          outcome: scenario.outcome,
          log,
        }}
      />
    </div>
  );
}

function titleFor(step: Step): string {
  switch (step) {
    case 'concept':
      return 'New character from a concept';
    case 'waiting':
      return 'Generating…';
    case 'timeout':
      return 'Still working';
    case 'preview':
      return 'Review your character';
    case 'failed':
      return 'Generation failed';
    case 'confirming':
      return 'Almost there';
    case 'accepted':
      return 'Sheet created';
  }
}

function AutoFocusSafe({ label, onClick }: { label: string; onClick: () => void }) {
  const ref = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    ref.current?.querySelector('button')?.focus();
  }, []);
  return (
    <span ref={ref}>
      <Button variant="secondary" label={label} onClick={onClick} />
    </span>
  );
}

function ScenarioBar({
  current,
  onScenario,
  state,
}: {
  current: Scenario;
  onScenario: (key: string) => void;
  state: Record<string, unknown>;
}) {
  const [open, setOpen] = useState(false);
  const index = SCENARIOS.indexOf(current);
  const go = useCallback(
    (delta: number) => {
      const next = SCENARIOS[(index + delta + SCENARIOS.length) % SCENARIOS.length] as Scenario;
      onScenario(next.key);
    },
    [index, onScenario],
  );
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (t.closest('input, textarea, [contenteditable], [role="dialog"]')) return;
      if (e.key === 'ArrowLeft') go(-1);
      if (e.key === 'ArrowRight') go(1);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [go]);
  if (import.meta.env.PROD) return null;
  return (
    <div className="proto-bar">
      {open && <pre className="proto-bar__state">{JSON.stringify(state, null, 2)}</pre>}
      <div className="proto-bar__row">
        <button type="button" onClick={() => go(-1)} aria-label="Previous scenario">
          ←
        </button>
        <span>
          {index + 1}/{SCENARIOS.length} · {current.name}
        </span>
        <button type="button" onClick={() => go(1)} aria-label="Next scenario">
          →
        </button>
        <button type="button" onClick={() => setOpen((o) => !o)}>
          {open ? 'hide state' : 'state'}
        </button>
      </div>
    </div>
  );
}
