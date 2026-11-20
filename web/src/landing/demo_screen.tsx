import { lazy, Suspense, useCallback, useState } from 'react';
import { Link } from 'react-router-dom';
import { useApiBaseUrl } from '../app/providers';
import { LANDING_PATH } from '../app/routes';
import { message } from '../l10n/strings';
import { BrandMark } from '../shell/brand_mark';
import { reportCtaClick } from './funnel_beacon';
import { signUpCtaPath } from './start_intent';
import { useStageThemeColor } from './use_stage_theme_color';
import './landing.css';
import './demo_page.css';

const DemoSheet = lazy(() => import('./demo_sheet'));

const FANTASY_SIGN_UP_PATH = signUpCtaPath('fantasy');

function DemoLoading() {
  return (
    <div className="lg-landing__demo-loading" data-testid="landing-demo-loading">
      {message('landing.demoLoading')}
    </div>
  );
}

export default function DemoScreen() {
  useStageThemeColor();
  const apiBaseUrl = useApiBaseUrl();
  const [interacted, setInteracted] = useState(false);
  const reportInteraction = useCallback(() => setInteracted(true), []);
  const onCtaClick = () => reportCtaClick(apiBaseUrl);

  return (
    <div className="lg-landing lg-demo" data-testid="demo-page">
      <header className="lg-demo__header">
        <div className="lg-landing__bar">
          <Link className="lg-landing__brand" to={LANDING_PATH} aria-label={message('app.name')}>
            <span className="lg-landing__mark" aria-hidden="true">
              <BrandMark />
            </span>
            <span className="lg-landing__brand-name">{message('app.name')}</span>
          </Link>
          <Link className="lg-landing__cta lg-landing__cta--outline" to={LANDING_PATH}>
            {message('demo.backHome')}
          </Link>
        </div>
        <h1 className="lg-demo__title">{message('demo.title')}</h1>
        <p className="lg-demo__lead">{message('demo.lead')}</p>
      </header>

      <main className="lg-demo__main">
        <section className="lg-demo__sheet" aria-label={message('landing.demoSheetLabel')}>
          <Suspense fallback={<DemoLoading />}>
            <DemoSheet onInteract={reportInteraction} />
          </Suspense>
          {interacted && (
            <div className="lg-landing__demo-cta" data-testid="landing-demo-cta">
              <p className="lg-landing__demo-cta-text">{message('landing.demoCtaText')}</p>
              <Link
                className="lg-landing__cta lg-landing__cta--primary"
                to={FANTASY_SIGN_UP_PATH}
                onClick={onCtaClick}
              >
                {message('landing.demoCtaAction')}
              </Link>
            </div>
          )}
        </section>
      </main>

      <div className="lg-landing__sticky" data-testid="landing-sticky-cta">
        <Link
          className="lg-landing__cta lg-landing__cta--primary"
          to={FANTASY_SIGN_UP_PATH}
          onClick={onCtaClick}
        >
          {message('landing.stickyCta')}
        </Link>
      </div>
    </div>
  );
}
