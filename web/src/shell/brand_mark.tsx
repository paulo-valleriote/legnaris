import './brand_mark.css';

/**
 * The product sigil: a diamond frame around an amber core. It is the one mark
 * shared by the navigation lockup and the auth surface, so the brand is never
 * redrawn per screen.
 */
export function BrandMark() {
  return (
    <svg viewBox="0 0 32 32" fill="none" aria-hidden="true">
      <path d="M16 2.5 29.5 16 16 29.5 2.5 16 16 2.5Z" stroke="currentColor" strokeWidth="1.4" />
      <path className="lg-brand__core" d="M16 10 22 16 16 22 10 16 16 10Z" />
    </svg>
  );
}
