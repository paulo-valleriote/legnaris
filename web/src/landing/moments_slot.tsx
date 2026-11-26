import { Component, lazy, Suspense, useState } from 'react';
import type { ComponentType, ReactNode } from 'react';
import { message } from '../l10n/strings';

type MomentsModule = { default: ComponentType };
type MomentsLoader = () => Promise<MomentsModule>;

const loadGameMoments: MomentsLoader = () => import('./game_moments');

const SHELLS = ['hp', 'slot', 'rest'] as const;

function MomentsSkeleton() {
  return (
    <div
      className="lg-landing__moments-skeleton"
      data-testid="landing-moments-loading"
      role="status"
    >
      <span className="lg-landing__visually-hidden">{message('landing.momentsLoading')}</span>
      {SHELLS.map((shell) => (
        <div className="lg-landing__moment-shell" key={shell} aria-hidden="true">
          <span className="lg-landing__shell-line lg-landing__shell-line--title" />
          <span className="lg-landing__shell-line" />
          <span className="lg-landing__shell-line lg-landing__shell-line--short" />
          <span className="lg-landing__shell-block" />
        </div>
      ))}
    </div>
  );
}

interface MomentsFailedProps {
  readonly onRetry: () => void;
}

function MomentsFailed({ onRetry }: MomentsFailedProps) {
  return (
    <div className="lg-landing__moments-failed" data-testid="landing-moments-failed" role="alert">
      <p className="lg-landing__text">{message('landing.momentsFailed')}</p>
      <button type="button" className="lg-landing__cta lg-landing__cta--outline" onClick={onRetry}>
        {message('landing.momentsRetry')}
      </button>
    </div>
  );
}

interface BoundaryProps {
  readonly fallback: ReactNode;
  readonly children: ReactNode;
}

interface BoundaryState {
  readonly failed: boolean;
}

class MomentsBoundary extends Component<BoundaryProps, BoundaryState> {
  state: BoundaryState = { failed: false };

  static getDerivedStateFromError(): BoundaryState {
    return { failed: true };
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}

interface MomentsSlotProps {
  readonly load?: MomentsLoader;
}

export function MomentsSlot({ load = loadGameMoments }: MomentsSlotProps) {
  const [current, setCurrent] = useState(() => ({ attempt: 0, component: lazy(load) }));

  const retry = () => {
    setCurrent((previous) => ({ attempt: previous.attempt + 1, component: lazy(load) }));
  };

  const GameMoments = current.component;

  return (
    <MomentsBoundary key={current.attempt} fallback={<MomentsFailed onRetry={retry} />}>
      <Suspense fallback={<MomentsSkeleton />}>
        <GameMoments />
      </Suspense>
    </MomentsBoundary>
  );
}
