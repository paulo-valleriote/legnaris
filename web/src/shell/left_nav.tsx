import { useState, useSyncExternalStore } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useAuthSession } from '../app/providers';
import { message } from '../l10n/strings';
import { leaveIsGuarded, requestLeave } from './leave_guard';
import { ThemeToggle } from './theme_toggle';

/** The route the root navigation item targets. */
const CATALOG_PATH = '/';
/** The route the session-only own-sheet item targets. */
const OWN_SHEET_PATH = '/your-sheet';

interface NavEntryProps {
  readonly path: string;
  readonly label: string;
}

/**
 * One navigation item. A click is routed through the leave registry (WSC-37): a
 * surface with unsaved work asks its guard to confirm before the navigation runs,
 * while any other click navigates through the link untouched.
 */
function NavEntry({ path, label }: NavEntryProps) {
  const navigate = useNavigate();

  return (
    <NavLink
      className="lg-nav__link"
      to={path}
      onClick={(event) => {
        if (!leaveIsGuarded()) {
          return;
        }
        event.preventDefault();
        requestLeave(() => navigate(path));
      }}
    >
      {label}
    </NavLink>
  );
}

/**
 * The persistent navigation region (order 1). It lists the declared routes; it
 * never collapses into an overlay at desktop widths (UIF-24, UIF-25). The root
 * item is labelled through the copy catalog, never a literal (WCT-29).
 *
 * The own-sheet entry renders only while a session is held (WAS-19, WAS-25); the
 * footer hosts the sign-out control and the theme control, sign-out disabled
 * while its teardown is in flight (WAS-23, UIF-27).
 */
export function LeftNav() {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const [signingOut, setSigningOut] = useState(false);

  const signOut = () => {
    if (signingOut) {
      return;
    }
    setSigningOut(true);
    void session.logout().finally(() => setSigningOut(false));
  };

  return (
    <nav className="lg-nav" aria-label="Primary">
      <p className="lg-nav__title">Legnaris</p>
      <NavEntry path={CATALOG_PATH} label={message('nav.catalog')} />
      {snapshot.state === 'authenticated' && (
        <NavEntry path={OWN_SHEET_PATH} label={message('nav.ownSheet')} />
      )}
      <div className="lg-nav__footer">
        {snapshot.state === 'authenticated' && (
          <button
            type="button"
            className="lg-nav__logout"
            onClick={signOut}
            disabled={signingOut}
            aria-busy={signingOut}
          >
            {message('nav.logout')}
          </button>
        )}
        <ThemeToggle />
      </div>
    </nav>
  );
}
