import './brand_mark.css';

/**
 * The product sigil: a diamond frame around a bold "L" with an accent core.
 * The frame and the "L" take the surrounding text colour (currentColor), and the
 * accent takes the theme accent token, so light and dark themes both keep contrast
 * without a second component. It is the one mark shared by the navigation lockup,
 * the landing and the auth surface, so the brand is never redrawn per screen.
 */
export function BrandMark() {
  return (
    <svg viewBox="0 0 120 120" fill="none" aria-hidden="true" focusable="false">
      <polygon
        points="60,7 113,60 60,113 7,60"
        fill="none"
        stroke="currentColor"
        strokeWidth="6"
        strokeLinejoin="miter"
      />
      <polygon
        points="43.9,34.7 56.55,34.7 56.55,72.65 76.1,72.65 76.1,85.3 43.9,85.3"
        fill="currentColor"
      />
      <polygon className="lg-brand__core" points="68.5,49 75.5,56 68.5,63 61.5,56" />
    </svg>
  );
}
