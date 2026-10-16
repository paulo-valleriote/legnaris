import { NavLink, useNavigate } from 'react-router-dom';
import { message } from '../l10n/strings';
import { leaveIsGuarded, requestLeave } from './leave_guard';

/** The route the root navigation item targets. */
const CATALOG_PATH = '/';

/**
 * The persistent navigation region (order 1). It lists the declared routes; it
 * never collapses into an overlay at desktop widths (FND-01, FND-03). The root
 * item is labelled through the copy catalog, never a literal (WCT-29).
 *
 * A click is routed through the leave registry (WSC-37): a surface with unsaved
 * work asks its guard to confirm before the navigation runs, while any other
 * click navigates through the link untouched.
 */
export function LeftNav() {
  const navigate = useNavigate();

  return (
    <nav className="lg-nav" aria-label="Primary">
      <p className="lg-nav__title">Legnaris</p>
      <NavLink
        className="lg-nav__link"
        to={CATALOG_PATH}
        onClick={(event) => {
          if (!leaveIsGuarded()) {
            return;
          }
          event.preventDefault();
          requestLeave(() => navigate(CATALOG_PATH));
        }}
      >
        {message('nav.catalog')}
      </NavLink>
    </nav>
  );
}
