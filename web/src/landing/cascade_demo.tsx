import { useState } from 'react';
import { message, messageWith } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';
import { MinusIcon, PlusIcon } from '../ui/icons';
import { MIGHT_MAX, MIGHT_MIN, MIGHT_DEFAULT, deriveCascade, formatSigned } from './cascade';

interface DerivedRowProps {
  readonly label: string;
  readonly formula: string;
  readonly value: string;
  readonly depth: number;
}

function noteKey(might: number): MessageKey {
  if (might >= MIGHT_MAX) {
    return 'landing.demoNoteMax';
  }
  if (might <= MIGHT_MIN) {
    return 'landing.demoNoteMin';
  }
  return 'landing.demoNote';
}

function DerivedRow({ label, formula, value, depth }: DerivedRowProps) {
  return (
    <div className={`lg-landing__demo-row lg-landing__demo-row--depth-${depth}`}>
      <span key={value} className="lg-landing__demo-heat" aria-hidden="true" />
      <dt className="lg-landing__demo-term">
        <span className="lg-landing__demo-name">{label}</span>
        <span className="lg-landing__demo-formula">{formula}</span>
      </dt>
      <dd className="lg-landing__demo-definition">
        <output key={value} className="lg-landing__demo-value" aria-live="off">
          {value}
        </output>
      </dd>
    </div>
  );
}

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
            <MinusIcon />
          </button>
          <output key={derived.might} className="lg-landing__demo-base-value" aria-live="off">
            {derived.might}
          </output>
          <button
            type="button"
            className="lg-landing__step"
            aria-label={message('landing.demoBaseUp')}
            disabled={derived.might >= MIGHT_MAX}
            onClick={() => setMight(derived.might + 1)}
          >
            <PlusIcon />
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

      <figcaption className="lg-landing__demo-note">{message(noteKey(derived.might))}</figcaption>

      <p className="lg-landing__demo-live" role="status" aria-live="polite" data-testid="demo-live">
        {messageWith('landing.demoChange', {
          might: derived.might,
          modifier: formatSigned(derived.modifier),
          climbing: formatSigned(derived.climbing),
          carry: derived.carry,
        })}
      </p>
    </figure>
  );
}
