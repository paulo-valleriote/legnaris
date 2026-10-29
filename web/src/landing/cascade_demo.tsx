import { useState } from 'react';
import { message } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';
import { MIGHT_MAX, MIGHT_MIN, MIGHT_DEFAULT, deriveCascade, formatSigned } from './cascade';

/** A minus or plus drawn as a stroke, so the glyph is identical in every font. */
function StepIcon({ plus }: { readonly plus: boolean }) {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
      <path d="M3 8h10" strokeLinecap="round" />
      {plus && <path d="M8 3v10" strokeLinecap="round" />}
    </svg>
  );
}

interface DerivedRowProps {
  readonly label: string;
  readonly formula: string;
  readonly value: string;
  readonly depth: number;
}

/** The caption under the example: the plain disclaimer, or a remark when a bound is reached. */
function noteKey(might: number): MessageKey {
  if (might >= MIGHT_MAX) {
    return 'landing.demoNoteMax';
  }
  if (might <= MIGHT_MIN) {
    return 'landing.demoNoteMin';
  }
  return 'landing.demoNote';
}

/**
 * One derived value. The value and its heat line are re-keyed on change, so only a row whose
 * value actually changed lights up, and `depth` staggers the rows to follow the dependency order.
 */
function DerivedRow({ label, formula, value, depth }: DerivedRowProps) {
  return (
    <div className={`lg-landing__demo-row lg-landing__demo-row--depth-${depth}`}>
      <span key={value} className="lg-landing__demo-heat" aria-hidden="true" />
      <dt className="lg-landing__demo-term">
        <span className="lg-landing__demo-name">{label}</span>
        <span className="lg-landing__demo-formula">{formula}</span>
      </dt>
      <dd className="lg-landing__demo-definition">
        <output key={value} className="lg-landing__demo-value">
          {value}
        </output>
      </dd>
    </div>
  );
}

/**
 * The landing's proof of the product's mechanism: one editable base score and three values that
 * follow it, so a visitor sees a derived value recalculate before ever signing up.
 */
export function CascadeDemo() {
  const [might, setMight] = useState(MIGHT_DEFAULT);
  const derived = deriveCascade(might);

  return (
    <figure className="lg-landing__demo" aria-label={message('landing.demoLabel')}>
      <div className="lg-landing__demo-base">
        <span className="lg-landing__demo-name" id="demo-base-label">
          {message('landing.demoBase')}
        </span>
        <div className="lg-landing__demo-stepper" role="group" aria-labelledby="demo-base-label">
          <button
            type="button"
            className="lg-landing__step"
            aria-label={message('landing.demoBaseDown')}
            disabled={derived.might <= MIGHT_MIN}
            onClick={() => setMight(derived.might - 1)}
          >
            <StepIcon plus={false} />
          </button>
          <output key={derived.might} className="lg-landing__demo-base-value">
            {derived.might}
          </output>
          <button
            type="button"
            className="lg-landing__step"
            aria-label={message('landing.demoBaseUp')}
            disabled={derived.might >= MIGHT_MAX}
            onClick={() => setMight(derived.might + 1)}
          >
            <StepIcon plus />
          </button>
        </div>
      </div>

      <dl className="lg-landing__demo-rows">
        <DerivedRow
          label={message('landing.demoModifier')}
          formula={message('landing.demoFormulaModifier')}
          value={formatSigned(derived.modifier)}
          depth={1}
        />
        <DerivedRow
          label={message('landing.demoClimbing')}
          formula={message('landing.demoFormulaClimbing')}
          value={formatSigned(derived.climbing)}
          depth={2}
        />
        <DerivedRow
          label={message('landing.demoCarry')}
          formula={message('landing.demoFormulaCarry')}
          value={String(derived.carry)}
          depth={1}
        />
      </dl>

      <figcaption className="lg-landing__demo-note" aria-live="polite">
        {message(noteKey(derived.might))}
      </figcaption>
    </figure>
  );
}
