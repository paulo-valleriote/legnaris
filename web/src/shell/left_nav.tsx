import { NavLink } from 'react-router-dom';
import { message } from '../l10n/strings';

/**
 * The persistent navigation region (order 1). It lists the declared routes; it
 * never collapses into an overlay at desktop widths (FND-01, FND-03). The root
 * item is labelled through the copy catalog, never a literal (WCT-29).
 */
export function LeftNav() {
  return (
    <nav className="lg-nav" aria-label="Primary">
      <p className="lg-nav__title">Legnaris</p>
      <NavLink className="lg-nav__link" to="/">
        {message('nav.catalog')}
      </NavLink>
    </nav>
  );
}
