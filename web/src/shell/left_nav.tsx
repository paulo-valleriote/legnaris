import { useState, useSyncExternalStore } from 'react';
import type { ReactNode } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useAuthSession } from '../app/providers';
import { message } from '../l10n/strings';
import { BrandMark } from './brand_mark';
import { leaveIsGuarded, requestLeave } from './leave_guard';

/** The route the root navigation item targets. */
const CATALOG_PATH = '/';
/** The route the session-only own-sheet item targets. */
const OWN_SHEET_PATH = '/your-sheet';

/** A stack of layers, the template-catalog affordance. */
function CatalogIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 3 20 7l-8 4-8-4 8-4Z" />
      <path d="m4 12 8 4 8-4" />
      <path d="m4 17 8 4 8-4" />
    </svg>
  );
}

/** A page with written rows, the character-sheet affordance. */
function SheetIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5Z" />
      <path d="M14 3v5h5" />
      <path d="M9 13h6" />
      <path d="M9 17h4" />
    </svg>
  );
}

/** A doorway arrow, the session-ending affordance. */
function SignOutIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M10 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4" />
      <path d="m15 8 4 4-4 4" />
      <path d="M19 12H9" />
    </svg>
  );
}

/** A trailing arrow that reads as "goes somewhere". */
function ChevronIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="m9 6 6 6-6 6" />
    </svg>
  );
}

interface NavEntryProps {
  readonly path: string;
  readonly label: string;
  readonly icon: ReactNode;
}

/**
 * One navigation item. A click is routed through the leave registry (WSC-37): a
 * surface with unsaved work asks its guard to confirm before the navigation runs,
 * while any other click navigates through the link untouched.
 */
function NavEntry({ path, label, icon }: NavEntryProps) {
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
      <span className="lg-nav__icon" aria-hidden="true">
        {icon}
      </span>
      <span className="lg-nav__label">{label}</span>
      <span className="lg-nav__chevron" aria-hidden="true">
        <ChevronIcon />
      </span>
    </NavLink>
  );
}

/**
 * The persistent navigation region (order 1). It opens with the product lockup,
 * then lists the declared routes; it is a left column on desktop and a compact
 * top bar on a narrow viewport, never an overlay or a hamburger (UIF-24, UIF-25). The root item is labelled through the copy catalog,
 * never a literal (WCT-29).
 *
 * The own-sheet entry renders only while a session is held (WAS-19, WAS-25); the
 * footer hosts the sign-out control, disabled while its teardown is in flight
 * (WAS-23). The signed-in app is always dark, so the navigation carries no theme
 * control.
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
      <div className="lg-nav__brand">
        <span className="lg-nav__mark" aria-hidden="true">
          <BrandMark />
        </span>
        <span className="lg-nav__brand-text">
          <span className="lg-nav__wordmark">{message('app.name')}</span>
          <span className="lg-nav__tagline">{message('nav.tagline')}</span>
        </span>
      </div>

      <div className="lg-nav__group">
        <p className="lg-nav__group-label">{message('nav.sectionWorkspace')}</p>
        <NavEntry path={CATALOG_PATH} label={message('nav.catalog')} icon={<CatalogIcon />} />
        {snapshot.state === 'authenticated' && (
          <NavEntry path={OWN_SHEET_PATH} label={message('nav.ownSheet')} icon={<SheetIcon />} />
        )}
      </div>

      <div className="lg-nav__footer">
        {snapshot.state === 'authenticated' && (
          <button
            type="button"
            className="lg-nav__action lg-nav__signout"
            onClick={signOut}
            disabled={signingOut}
            aria-busy={signingOut}
          >
            <span className="lg-nav__icon" aria-hidden="true">
              <SignOutIcon />
            </span>
            <span className="lg-nav__label">{message('nav.logout')}</span>
          </button>
        )}
      </div>
    </nav>
  );
}
