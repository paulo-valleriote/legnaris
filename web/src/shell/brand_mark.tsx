import './brand_mark.css';
import { BRAND_CORE_POINTS, BRAND_GLYPH_POINTS } from './brand_mark_geometry';

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
