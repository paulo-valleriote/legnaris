import { forwardRef, useEffect, useId, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { message, messageWith } from '../l10n/strings';
import { HitPointsCard } from '../sheet/preset/fantasy/hp_card';
import {
  HIT_POINTS_CURRENT,
  HIT_POINTS_MAX,
  HIT_POINTS_TEMP,
  applyDamage,
  hitPointsFrom,
} from '../sheet/preset/fantasy/hit_points';
import { slotPool, spendSlot } from '../sheet/preset/fantasy/slot_pips';
import { SpellSlots } from '../sheet/preset/fantasy/spells';
import { XpBar } from '../sheet/preset/fantasy/xp_panel';
import { LEVEL_PATH, XP_PATH, xpProgress } from '../sheet/preset/fantasy/xp_progress';
import { usePresetCommit, usePresetContext, usePresetValue } from '../sheet/preset/preset_context';
import { numberValue } from '../sheet/preset/values';
import { Button } from '../ui/button';
import { DemoProvider } from './demo_provider';
import './game_moments.css';

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';

function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState(prefersReducedMotion);

  useEffect(() => {
    if (typeof window.matchMedia !== 'function') {
      return;
    }
    const query = window.matchMedia(REDUCED_MOTION_QUERY);
    const update = () => setReduced(query.matches);
    update();
    query.addEventListener?.('change', update);
    return () => query.removeEventListener?.('change', update);
  }, []);

  return reduced;
}

interface FlashProps {
  readonly pulse: number;
  readonly reduced: boolean;
}

function Flash({ pulse, reduced }: FlashProps) {
  if (reduced || pulse === 0) {
    return null;
  }
  return <span key={pulse} className="lg-moment__flash" aria-hidden="true" />;
}

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

function XpMoment() {
  const level = numberValue(usePresetValue(LEVEL_PATH));
  const xp = usePresetValue(XP_PATH);
  const commitXp = usePresetCommit(XP_PATH);
  const commitLevel = usePresetCommit(LEVEL_PATH);
  const reduced = useReducedMotion();
  const [pulse, setPulse] = useState(0);
  const resultRef = useRef<HTMLParagraphElement>(null);
  const progress = xpProgress(level, xp);
  const gain =
    progress.nextThreshold === null ? 0 : Math.max(0, progress.nextThreshold - progress.xp);
  const nextLevel = progress.nextLevel;
  const result = progress.canLevelUp
    ? messageWith('landing.momentXpAnnounce', { count: progress.xp })
    : messageWith('landing.momentXpValue', { count: progress.level, total: progress.xp });

  const gainExperience = () => {
    if (progress.nextThreshold === null || gain === 0) {
      return;
    }
    commitXp(progress.xp + gain);
    setPulse((value) => value + 1);
    resultRef.current?.focus();
  };

  const takeLevel = () => {
    if (nextLevel === null) {
      return;
    }
    commitLevel(nextLevel);
    setPulse((value) => value + 1);
    resultRef.current?.focus();
  };

  return (
    <Moment
      title={message('landing.momentXpTitle')}
      body={message('landing.momentXpBody')}
      testId="landing-moment-xp"
    >
      <div className="lg-moment__stage">
        <Flash pulse={pulse} reduced={reduced} />
        <p className="lg-moment__level" data-testid="landing-moment-xp-level">
          {messageWith('sheet.play.level', { count: progress.level })}
        </p>
        <XpBar />
      </div>
      <div className="lg-moment__actions">
        {gain > 0 && (
          <Button
            variant="secondary"
            size="sm"
            label={messageWith('landing.momentXpGain', { count: gain })}
            onClick={gainExperience}
          />
        )}
        {progress.canLevelUp && nextLevel !== null && (
          <Button
            size="sm"
            label={messageWith('landing.momentXpLevelUp', { count: nextLevel })}
            onClick={takeLevel}
          />
        )}
      </div>
      <Result ref={resultRef} text={result} />
    </Moment>
  );
}

function HpMoment() {
  const { store } = usePresetContext();
  const current = numberValue(usePresetValue(HIT_POINTS_CURRENT));
  const max = numberValue(usePresetValue(HIT_POINTS_MAX));
  const reduced = useReducedMotion();
  const [pulse, setPulse] = useState(0);
  const resultRef = useRef<HTMLParagraphElement>(null);
  const exhausted = (current ?? 0) <= 0;
  const result =
    current === undefined
      ? ''
      : messageWith('landing.momentHpAnnounce', { count: current, total: max ?? 0 });

  const takeDamage = () => {
    const points = hitPointsFrom(store.getState().data);
    const change = applyDamage(points, 5);
    if (change === undefined) {
      return;
    }
    store.setPath(HIT_POINTS_CURRENT, change.current);
    store.setPath(HIT_POINTS_TEMP, change.temp);
    setPulse((value) => value + 1);
    resultRef.current?.focus();
  };

  return (
    <Moment
      title={message('landing.momentHpTitle')}
      body={message('landing.momentHpBody')}
      testId="landing-moment-hp"
    >
      <div className="lg-moment__stage">
        <Flash pulse={pulse} reduced={reduced} />
        <HitPointsCard />
      </div>
      <div className="lg-moment__actions">
        <Button
          variant="danger"
          size="sm"
          label={message('landing.momentHpDamage')}
          disabled={exhausted}
          onClick={takeDamage}
        />
      </div>
      <Result ref={resultRef} text={result} />
    </Moment>
  );
}

function SlotMoment() {
  const { store } = usePresetContext();
  const total = numberValue(usePresetValue('spells.slot1.total'));
  const expended = numberValue(usePresetValue('spells.slot1.expended'));
  const reduced = useReducedMotion();
  const [pulse, setPulse] = useState(0);
  const resultRef = useRef<HTMLParagraphElement>(null);
  const pool = slotPool(total, expended);
  const result = messageWith('landing.momentSlotAnnounce', { count: pool.remaining });

  const spend = () => {
    if (pool.remaining === 0) {
      return;
    }
    const next = spendSlot(pool);
    store.setPath('spells.slot1.expended', next.expended);
    setPulse((value) => value + 1);
    resultRef.current?.focus();
  };

  return (
    <Moment
      title={message('landing.momentSlotTitle')}
      body={message('landing.momentSlotBody')}
      testId="landing-moment-slot"
    >
      <div className="lg-moment__stage">
        <Flash pulse={pulse} reduced={reduced} />
        <SpellSlots />
      </div>
      <div className="lg-moment__actions">
        <Button
          variant="secondary"
          size="sm"
          label={message('landing.momentSlotSpend')}
          disabled={pool.remaining === 0}
          onClick={spend}
        />
      </div>
      <Result ref={resultRef} text={result} />
    </Moment>
  );
}

export default function GameMoments() {
  return (
    <DemoProvider>
      <div className="lg-moment-grid" data-testid="landing-game-moments">
        <XpMoment />
        <HpMoment />
        <SlotMoment />
      </div>
    </DemoProvider>
  );
}
