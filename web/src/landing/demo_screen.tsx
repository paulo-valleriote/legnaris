import { lazy, Suspense, useCallback, useState } from 'react';
import { Link } from 'react-router-dom';
import { useApiBaseUrl } from '../app/providers';
import { LANDING_PATH, SRD_PATH } from '../app/routes';
import { message } from '../l10n/strings';
import { BrandMark } from '../shell/brand_mark';
import { reportCtaClick } from './funnel_beacon';
import { signUpCtaPath } from './start_intent';
import { useStageThemeColor } from './use_stage_theme_color';
import './landing.css';
import './demo_page.css';

const FifthDemoSheet = lazy(() => import('./fifth_demo_sheet'));

const FIFTH_SIGN_UP_PATH = signUpCtaPath('fifth');

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
          <nav className="lg-demo__actions" aria-label={message('demo.actionsLabel')}>
            <Link className="lg-landing__cta lg-landing__cta--outline" to={LANDING_PATH}>
              {message('demo.backHome')}
            </Link>
            <Link
              className="lg-landing__cta lg-landing__cta--primary lg-demo__header-cta"
              to={FIFTH_SIGN_UP_PATH}
              onClick={onCtaClick}
              aria-label={message('landing.create')}
            >
              <span className="lg-demo__header-cta-full" aria-hidden="true">
                {message('landing.create')}
              </span>
              <span
                className="lg-demo__header-cta-compact"
                aria-hidden="true"
                data-testid="demo-header-cta-compact"
              >
                {message('demo.headerCtaCompact')}
              </span>
            </Link>
          </nav>
        </div>
        <h1 className="lg-demo__title">{message('demo.title')}</h1>
        <p className="lg-demo__lead">{message('demo.lead')}</p>
      </header>

      <main className="lg-demo__main">
        <section
          className="lg-demo__sheet"
          aria-label={message('landing.demoSheetLabel')}
          data-testid="landing-demo-sheet"
        >
          <Suspense fallback={<DemoLoading />}>
            <FifthDemoSheet onInteract={reportInteraction} />
          </Suspense>
          <div
            className={`lg-landing__demo-cta lg-demo__end${interacted ? ' lg-demo__end--played' : ''}`}
            data-testid="landing-demo-cta"
          >
            <p className="lg-demo__end-title">
              {message(interacted ? 'landing.demoCtaText' : 'demo.endIdle')}
            </p>
            <Link
              className="lg-landing__cta lg-landing__cta--primary"
              to={FIFTH_SIGN_UP_PATH}
              onClick={onCtaClick}
            >
              {message('landing.demoCtaAction')}
            </Link>
          </div>
        </section>
      </main>

      <div
        className={`lg-landing__sticky${interacted ? ' lg-landing__sticky--shown' : ''}`}
        inert={!interacted}
        data-testid="landing-sticky-cta"
      >
        <Link
          className="lg-landing__cta lg-landing__cta--primary"
          to={FIFTH_SIGN_UP_PATH}
          onClick={onCtaClick}
        >
          {message('landing.create')}
        </Link>
      </div>

      <footer className="lg-demo__footer" aria-label={message('landing.footerLabel')}>
        <Link className="lg-demo__footer-link" to={SRD_PATH}>
          {message('landing.footer.srd')}
        </Link>
      </footer>
    </div>
  );
}
