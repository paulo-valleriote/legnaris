import './brand_mark.css';
import { BRAND_CORE_POINTS, BRAND_GLYPH_POINTS } from './brand_mark_geometry';

/**
 * The product sigil: a diamond frame around a bold "L" with a brand-orange core.
 * The frame and the "L" take the surrounding text colour (currentColor), and the core takes the
 * brand token, which is the same orange as the app icon. It is the one mark shared by the
 * navigation lockup, the landing and the auth surface, so the brand is never redrawn per screen.
 */
export function BrandMark() {
  return (
    <svg viewBox="0 0 120 120" fill="none" aria-hidden="true" focusable="false">
      <polygon
        points="60,7 113,60 60,113 7,60"
        stroke="currentColor"
        strokeWidth="6"
        strokeLinejoin="miter"
      />
      <polygon points={BRAND_GLYPH_POINTS} fill="currentColor" />
      <polygon className="lg-brand__core" points={BRAND_CORE_POINTS} />
    </svg>
  );
}
