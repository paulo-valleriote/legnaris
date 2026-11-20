import { lazy, Suspense, useEffect } from 'react';
import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { useApiBaseUrl } from '../app/providers';
import { DEMO_PATH, PRIVACY_PATH, SIGN_IN_PATH } from '../app/routes';
import { message, messageWith } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';
import { BrandMark } from '../shell/brand_mark';
import { ChevronDownIcon } from '../ui/icons';
import { reportCtaClick, reportHomeVisit } from './funnel_beacon';
import { signUpCtaPath } from './start_intent';
import { useStageThemeColor } from './use_stage_theme_color';
import './landing.css';

const GameMoments = lazy(() => import('./game_moments'));

const HERO_SRCSET = ['/images/hero-lava-1280.webp 1280w', '/images/hero-lava-1920.webp 1920w'].join(
  ', ',
);

const FANTASY_SIGN_UP_PATH = signUpCtaPath('fantasy');

function MomentsLoading() {
  return (
    <div className="lg-landing__demo-loading" data-testid="landing-moments-loading">
      {message('landing.momentsLoading')}
    </div>
  );
}

function TemplateVisual() {
  return (
    <span
      className="lg-landing__mini lg-landing__mini--template"
      data-testid="landing-step-visual-template"
    >
      <span className="lg-landing__mini-row" />
      <span className="lg-landing__mini-row lg-landing__mini-row--active" />
      <span className="lg-landing__mini-row" />
    </span>
  );
}

function SheetVisual() {
  return (
    <span
      className="lg-landing__mini lg-landing__mini--sheet"
      data-testid="landing-step-visual-sheet"
    >
      <span className="lg-landing__mini-stat">
        <span className="lg-landing__mini-label">{message('landing.stepVisualName')}</span>
        <span className="lg-landing__mini-score">{message('landing.stepVisualScore')}</span>
      </span>
      <span className="lg-landing__mini-derived">{message('landing.stepVisualDerived')}</span>
    </span>
  );
}

function PlayVisual() {
  return (
    <span
      className="lg-landing__mini lg-landing__mini--play"
      data-testid="landing-step-visual-play"
    >
      <span className="lg-landing__mini-hp">
        <span className="lg-landing__mini-hp-fill" />
      </span>
      <span className="lg-landing__mini-pips">
        <span className="lg-landing__mini-pip" />
        <span className="lg-landing__mini-pip lg-landing__mini-pip--spent" />
        <span className="lg-landing__mini-pip" />
      </span>
    </span>
  );
}

interface Step {
  readonly title: MessageKey;
  readonly body: MessageKey;
  readonly visual: () => ReactNode;
}

const STEPS: readonly Step[] = [
  { title: 'landing.step1Title', body: 'landing.step1Body', visual: TemplateVisual },
  { title: 'landing.step2Title', body: 'landing.step2Body', visual: SheetVisual },
  { title: 'landing.step3Title', body: 'landing.step3Body', visual: PlayVisual },
];

const FAQ: readonly { readonly question: MessageKey; readonly answer: MessageKey }[] = [
  { question: 'landing.faq1Question', answer: 'landing.faq1Answer' },
  { question: 'landing.faq2Question', answer: 'landing.faq2Answer' },
  { question: 'landing.faq3Question', answer: 'landing.faq3Answer' },
  { question: 'landing.faq4Question', answer: 'landing.faq4Answer' },
  { question: 'landing.faq5Question', answer: 'landing.faq5Answer' },
];

export function LandingScreen() {
  useStageThemeColor();
  const apiBaseUrl = useApiBaseUrl();

  useEffect(() => {
    reportHomeVisit(apiBaseUrl);
  }, [apiBaseUrl]);

  const onCtaClick = () => reportCtaClick(apiBaseUrl);

  return (
    <div className="lg-landing" data-testid="landing">
      <header className="lg-landing__hero">
        <img
          className="lg-landing__photo"
          src="/images/hero-lava-1920.webp"
          srcSet={HERO_SRCSET}
          sizes="100vw"
          width={1920}
          height={1080}
          alt=""
          decoding="async"
          fetchPriority="high"
        />
        <div className="lg-landing__scrim" aria-hidden="true" />

        <div className="lg-landing__bar">
          <Link className="lg-landing__brand" to="/" aria-label={message('app.name')}>
            <span className="lg-landing__mark" aria-hidden="true">
              <BrandMark />
            </span>
            <span className="lg-landing__brand-name">{message('app.name')}</span>
          </Link>
          <Link className="lg-landing__cta lg-landing__cta--outline" to={SIGN_IN_PATH}>
            {message('auth.submitLogin')}
          </Link>
        </div>

        <div className="lg-landing__hero-body">
          <h1 className="lg-landing__hero-title">{message('landing.heroTitle')}</h1>
          <p className="lg-landing__hero-lead">{message('landing.heroLead')}</p>
          <div className="lg-landing__actions">
            <Link
              className="lg-landing__cta lg-landing__cta--primary"
              to={FANTASY_SIGN_UP_PATH}
              onClick={onCtaClick}
            >
              {message('landing.create')}
            </Link>
            <Link className="lg-landing__cta lg-landing__cta--outline" to={DEMO_PATH}>
              {message('landing.tryDemo')}
            </Link>
          </div>
        </div>
      </header>

      <main>
        <section
          className="lg-landing__section lg-landing__section--band"
          aria-labelledby="moments-title"
        >
          <div className="lg-landing__inner">
            <h2 className="lg-landing__title" id="moments-title">
              {message('landing.momentsTitle')}
            </h2>
            <p className="lg-landing__text lg-landing__moments-lead">
              {message('landing.momentsLead')}
            </p>
            <Suspense fallback={<MomentsLoading />}>
              <GameMoments />
            </Suspense>
            <p className="lg-landing__text lg-landing__moments-note">
              {message('landing.momentNote')}
            </p>
          </div>
        </section>

        <section className="lg-landing__section" aria-labelledby="how-title">
          <div className="lg-landing__inner">
            <div className="lg-landing__how">
              <h2 className="lg-landing__title" id="how-title">
                {message('landing.howTitle')}
              </h2>
              <ol className="lg-landing__steps" aria-label={message('landing.howStepsLabel')}>
                {STEPS.map((step) => (
                  <li className="lg-landing__step-item" key={step.title}>
                    <h3 className="lg-landing__step-title">{message(step.title)}</h3>
                    <p className="lg-landing__text">{message(step.body)}</p>
                    <span className="lg-landing__step-visual" aria-hidden="true">
                      {step.visual()}
                    </span>
                  </li>
                ))}
              </ol>
            </div>
          </div>
        </section>

        <section
          className="lg-landing__section lg-landing__section--band"
          aria-labelledby="faq-title"
        >
          <div className="lg-landing__inner lg-landing__split">
            <h2 className="lg-landing__title" id="faq-title">
              {message('landing.faqTitle')}
            </h2>
            <div className="lg-landing__faq">
              {FAQ.map((entry) => (
                <details className="lg-landing__faq-item" key={entry.question}>
                  <summary className="lg-landing__faq-question">
                    <span>{message(entry.question)}</span>
                    <span className="lg-landing__faq-chevron">
                      <ChevronDownIcon />
                    </span>
                  </summary>
                  <p className="lg-landing__text lg-landing__faq-answer">{message(entry.answer)}</p>
                </details>
              ))}
            </div>
          </div>
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

      <footer
        className="lg-landing__section lg-landing__close"
        aria-label={message('landing.footerLabel')}
      >
        <div className="lg-landing__inner lg-landing__close-body">
          <h2 className="lg-landing__title lg-landing__close-title">
            {message('landing.closeTitle')}
          </h2>
          <div className="lg-landing__actions">
            <Link
              className="lg-landing__cta lg-landing__cta--primary"
              to={FANTASY_SIGN_UP_PATH}
              onClick={onCtaClick}
            >
              {message('landing.create')}
            </Link>
            <Link className="lg-landing__cta lg-landing__cta--outline" to={SIGN_IN_PATH}>
              {message('auth.submitLogin')}
            </Link>
          </div>
        </div>
        <div className="lg-landing__inner lg-landing__legal">
          <small className="lg-landing__copyright">
            {messageWith('landing.copyright', { year: new Date().getFullYear() })}
          </small>
          <Link className="lg-landing__legal-link" to={PRIVACY_PATH}>
            {message('landing.footer.privacy')}
          </Link>
        </div>
      </footer>
    </div>
  );
}
