/**
 * The public landing page a visitor without a session sees on `/`: a photographic hero, the
 * product's principles, a live recalculation example, the order of play, plain answers and a
 * closing call to action. It owns no state beyond the example and never touches the API.
 */

import { Link } from 'react-router-dom';
import { SIGN_IN_PATH, SIGN_UP_PATH } from '../app/routes';
import { message, messageWith } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';
import { BrandMark } from '../shell/brand_mark';
import { CascadeDemo } from './cascade_demo';
import { useStageThemeColor } from './use_stage_theme_color';
import './landing.css';

const HERO_SRCSET = ['/images/hero-lava-800.webp 800w', '/images/hero-lava-1336.webp 1336w'].join(
  ', ',
);

interface Entry {
  readonly title: MessageKey;
  readonly body: MessageKey;
}

const PRINCIPLES: readonly Entry[] = [
  { title: 'landing.principle1Title', body: 'landing.principle1Body' },
  { title: 'landing.principle2Title', body: 'landing.principle2Body' },
  { title: 'landing.principle3Title', body: 'landing.principle3Body' },
  { title: 'landing.principle4Title', body: 'landing.principle4Body' },
];

const STEPS: readonly Entry[] = [
  { title: 'landing.step1Title', body: 'landing.step1Body' },
  { title: 'landing.step2Title', body: 'landing.step2Body' },
  { title: 'landing.step3Title', body: 'landing.step3Body' },
];

const FAQ: readonly { readonly question: MessageKey; readonly answer: MessageKey }[] = [
  { question: 'landing.faq1Question', answer: 'landing.faq1Answer' },
  { question: 'landing.faq2Question', answer: 'landing.faq2Answer' },
  { question: 'landing.faq3Question', answer: 'landing.faq3Answer' },
  { question: 'landing.faq4Question', answer: 'landing.faq4Answer' },
  { question: 'landing.faq5Question', answer: 'landing.faq5Answer' },
];

function Chevron() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
      <path d="m4 6 4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/** The landing page for visitors without a session. */
export function LandingScreen() {
  useStageThemeColor();

  return (
    <div className="lg-landing" data-testid="landing">
      <header className="lg-landing__hero">
        <img
          className="lg-landing__photo"
          src="/images/hero-lava-1336.webp"
          srcSet={HERO_SRCSET}
          sizes="100vw"
          width={1336}
          height={1781}
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
          <div className="lg-landing__hero-side">
            <p className="lg-landing__hero-lead">{message('landing.heroLead')}</p>
            <div className="lg-landing__actions">
              <Link className="lg-landing__cta lg-landing__cta--primary" to={SIGN_UP_PATH}>
                {message('landing.create')}
              </Link>
            </div>
          </div>
        </div>
      </header>

      <main>
        <section
          className="lg-landing__section lg-landing__section--band"
          aria-labelledby="principles-title"
        >
          <div className="lg-landing__inner lg-landing__split">
            <h2 className="lg-landing__title" id="principles-title">
              {message('landing.principlesTitle')}
            </h2>
            <ul className="lg-landing__principles">
              {PRINCIPLES.map((entry) => (
                <li className="lg-landing__principle" key={entry.title}>
                  <h3 className="lg-landing__principle-title">{message(entry.title)}</h3>
                  <p className="lg-landing__text">{message(entry.body)}</p>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className="lg-landing__section" aria-labelledby="how-title">
          <div className="lg-landing__inner lg-landing__split">
            <div className="lg-landing__how">
              <h2 className="lg-landing__title" id="how-title">
                {message('landing.howTitle')}
              </h2>
              <ol className="lg-landing__steps" aria-label={message('landing.howStepsLabel')}>
                {STEPS.map((step) => (
                  <li className="lg-landing__step-item" key={step.title}>
                    <h3 className="lg-landing__step-title">{message(step.title)}</h3>
                    <p className="lg-landing__text">{message(step.body)}</p>
                  </li>
                ))}
              </ol>
            </div>
            <CascadeDemo />
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
                      <Chevron />
                    </span>
                  </summary>
                  <p className="lg-landing__text lg-landing__faq-answer">{message(entry.answer)}</p>
                </details>
              ))}
            </div>
          </div>
        </section>
      </main>

      <footer
        className="lg-landing__section lg-landing__close"
        aria-label={message('landing.footerLabel')}
      >
        <div className="lg-landing__inner lg-landing__close-body">
          <h2 className="lg-landing__title lg-landing__close-title">
            {message('landing.closeTitle')}
          </h2>
          <div className="lg-landing__actions">
            <Link className="lg-landing__cta lg-landing__cta--primary" to={SIGN_UP_PATH}>
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
        </div>
      </footer>
    </div>
  );
}
