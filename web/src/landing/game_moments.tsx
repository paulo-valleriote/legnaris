import { forwardRef, useId, useRef } from 'react';
import type { ReactNode } from 'react';
import { message, messageWith } from '../l10n/strings';
import { HitPointsTrack } from '../sheet/preset/compose/hit_points_track';
import { Pips } from '../sheet/preset/compose/pips';
import type { PresetHitPointsPiece, PresetPipsPiece } from '../sheet/preset/compose/types';
import { usePresetContext, usePresetValue } from '../sheet/preset/preset_context';
import { numberValue } from '../sheet/preset/values';
import { Button } from '../ui/button';
import { DemoProvider } from './demo_provider';
import { longRestCascade, MOMENT_DERIVATION, MOMENT_SHEET, MOMENT_TEMPLATE } from './moment_demo';
import './game_moments.css';

const HIT_POINTS_PIECE: PresetHitPointsPiece = {
  kind: 'hitPoints',
  key: 'hitPoints',
  current: 'combat.hp.current',
  max: 'combat.hp.max',
  temp: 'combat.hp.temp',
};

const SLOT_PIECE: PresetPipsPiece = {
  kind: 'pips',
  key: 'slot1',
  count: 'spellcasting.slot1.spent',
  max: 'spellcasting.slot1.total',
};

interface MomentProps {
  readonly title: string;
  readonly body: string;
  readonly testId: string;
  readonly children: ReactNode;
}

function Moment({ title, body, testId, children }: MomentProps) {
  const titleId = useId();

  return (
    <article className="lg-moment" data-testid={testId} aria-labelledby={titleId}>
      <h3 className="lg-moment__title" id={titleId}>
        {title}
      </h3>
      <p className="lg-moment__body">{body}</p>
      {children}
    </article>
  );
}

interface ResultProps {
  readonly text: string;
}

const Result = forwardRef<HTMLParagraphElement, ResultProps>(function Result({ text }, ref) {
  return (
    <p
      ref={ref}
      className="lg-moment__result"
      role="status"
      aria-live="polite"
      aria-atomic="true"
      tabIndex={-1}
    >
      {text}
    </p>
  );
});

function HpMoment() {
  const current = numberValue(usePresetValue('combat.hp.current'));
  const max = numberValue(usePresetValue('combat.hp.max'));
  const result =
    current === undefined
      ? ''
      : messageWith('landing.momentHpAnnounce', { count: current, total: max ?? 0 });

  return (
    <Moment
      title={message('landing.momentHpTitle')}
      body={message('landing.momentHpBody')}
      testId="landing-moment-hp"
    >
      <div className="lg-moment__stage">
        <HitPointsTrack piece={HIT_POINTS_PIECE} />
      </div>
      <Result text={result} />
    </Moment>
  );
}

function SlotMoment() {
  const { store } = usePresetContext();
  const total = numberValue(usePresetValue('spellcasting.slot1.total')) ?? 0;
  const spent = numberValue(usePresetValue('spellcasting.slot1.spent')) ?? 0;
  const resultRef = useRef<HTMLParagraphElement>(null);
  const remaining = Math.max(0, total - spent);
  const result = messageWith('landing.momentSlotAnnounce', { count: remaining });

  const spend = () => {
    if (remaining === 0) {
      return;
    }
    store.setPath('spellcasting.slot1.spent', spent + 1);
    resultRef.current?.focus();
  };

  return (
    <Moment
      title={message('landing.momentSlotTitle')}
      body={message('landing.momentSlotBody')}
      testId="landing-moment-slot"
    >
      <div className="lg-moment__stage">
        <Pips piece={SLOT_PIECE} />
      </div>
      <div className="lg-moment__actions">
        <Button
          variant="secondary"
          size="sm"
          label={message('landing.momentSlotSpend')}
          disabled={remaining === 0}
          onClick={spend}
        />
      </div>
      <Result ref={resultRef} text={result} />
    </Moment>
  );
}

function RestMoment() {
  const { store } = usePresetContext();
  const current = numberValue(usePresetValue('combat.hp.current'));
  const max = numberValue(usePresetValue('combat.hp.max'));
  const remaining = numberValue(usePresetValue('spellcasting.slot1.remaining'));
  const resultRef = useRef<HTMLParagraphElement>(null);
  const result =
    current === undefined
      ? ''
      : messageWith('landing.momentRestAnnounce', {
          count: current,
          total: max ?? 0,
          slots: remaining ?? 0,
        });

  const rest = () => {
    store.applyReset(longRestCascade(store.getState().data));
    resultRef.current?.focus();
  };

  return (
    <Moment
      title={message('landing.momentRestTitle')}
      body={message('landing.momentRestBody')}
      testId="landing-moment-rest"
    >
      <div className="lg-moment__stage">
        <p className="lg-moment__stat" data-testid="landing-moment-rest-hp">
          {messageWith('landing.momentRestHp', { count: current ?? 0, total: max ?? 0 })}
        </p>
        <Pips piece={SLOT_PIECE} />
      </div>
      <div className="lg-moment__actions">
        <Button
          variant="secondary"
          size="sm"
          label={message('landing.momentRestAction')}
          onClick={rest}
        />
      </div>
      <Result ref={resultRef} text={result} />
    </Moment>
  );
}

export default function GameMoments() {
  return (
    <DemoProvider template={MOMENT_TEMPLATE} sheet={MOMENT_SHEET} derive={MOMENT_DERIVATION}>
      <div className="lg-moment-grid" data-testid="landing-game-moments">
        <HpMoment />
        <SlotMoment />
        <RestMoment />
      </div>
    </DemoProvider>
  );
}

export function HeroSheet() {
  return (
    <DemoProvider template={MOMENT_TEMPLATE} sheet={MOMENT_SHEET} derive={MOMENT_DERIVATION}>
      <div className="lg-moment-hero" data-testid="landing-hero-sheet">
        <HpMoment />
      </div>
    </DemoProvider>
  );
}
