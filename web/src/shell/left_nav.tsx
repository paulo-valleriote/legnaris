import { NavLink } from 'react-router-dom';

/**
 * The persistent navigation region (order 1). It lists the declared routes; it
 * never collapses into an overlay at desktop widths (FND-01, FND-03).
 */
export function LeftNav() {
  return (
    <nav className="lg-nav" aria-label="Primary">
      <p className="lg-nav__title">Legnaris</p>
      <NavLink className="lg-nav__link" to="/">
        Workspace
      </NavLink>
    </nav>
  );
}
