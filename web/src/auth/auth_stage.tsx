import { Link } from 'react-router-dom';
import { LANDING_PATH } from '../app/routes';
import { message } from '../l10n/strings';
import { BrandMark } from '../shell/brand_mark';
import { useStageThemeColor } from '../landing/use_stage_theme_color';

const PHOTO_SRCSET = [
  '/images/hero-lava-1280.webp 1280w',
  '/images/hero-lava-1920.webp 1920w',
].join(', ');

interface AuthStageProps {
  readonly linked?: boolean;
}

export function AuthStage({ linked = true }: AuthStageProps) {
  useStageThemeColor();

  const lockup = (
    <>
      <span className="lg-auth__mark" aria-hidden="true">
        <BrandMark />
      </span>
      <span className="lg-auth__wordmark">{message('app.name')}</span>
    </>
  );

  return (
    <div className="lg-auth__stage">
      <img
        className="lg-auth__photo"
        src="/images/hero-lava-1920.webp"
        srcSet={PHOTO_SRCSET}
        sizes="(min-width: 56rem) 60vw, 100vw"
        width={1920}
        height={1080}
        alt=""
        decoding="async"
        fetchPriority="high"
      />
      <div className="lg-auth__scrim" aria-hidden="true" />
      {linked ? (
        <Link className="lg-auth__brand" to={LANDING_PATH} aria-label={message('app.name')}>
          {lockup}
        </Link>
      ) : (
        <div className="lg-auth__brand">{lockup}</div>
      )}
    </div>
  );
}
