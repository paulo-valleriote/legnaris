import { useEffect, useId, useRef, useState, useSyncExternalStore } from 'react';
import type { ReactNode } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useAuthSession } from '../app/providers';
import { CAMPAIGNS_PATH, PLANS_PATH } from '../app/routes';
import { message } from '../l10n/strings';
import { ChevronRightIcon, MenuIcon } from '../ui/icons';
import { BrandMark } from './brand_mark';
import { useCompactNav } from './compact_nav';
import { leaveIsGuarded, requestLeave } from './leave_guard';

const CATALOG_PATH = '/';
const OWN_SHEET_PATH = '/your-sheet';
const ACCOUNT_PATH = '/account';

function PlansIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="3" y="6" width="18" height="13" rx="2" />
      <path d="M3 10h18" />
      <path d="M7 15h4" />
    </svg>
  );
}

function CampaignsIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="9" cy="8" r="3" />
      <path d="M3 19c0-3 2.7-5 6-5s6 2 6 5" />
      <path d="M16 5.5a3 3 0 0 1 0 5.5" />
      <path d="M18 14c2 .7 3 2.4 3 5" />
    </svg>
  );
}

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

function AccountIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21a8 8 0 0 1 16 0" />
    </svg>
  );
}

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

interface NavEntryProps {
  readonly path: string;
  readonly label: string;
  readonly icon: ReactNode;
  readonly onActivate?: () => void;
}

function NavEntry({ path, label, icon, onActivate }: NavEntryProps) {
  const navigate = useNavigate();

  return (
    <NavLink
      className="lg-nav__link"
      to={path}
      onClick={(event) => {
        onActivate?.();
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
        <ChevronRightIcon />
      </span>
    </NavLink>
  );
}

export function LeftNav() {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const [signingOut, setSigningOut] = useState(false);
  const [signOutFailed, setSignOutFailed] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const signOutFailureId = useId();
  const menuId = useId();
  const compact = useCompactNav();
  const [compactSeen, setCompactSeen] = useState(compact);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  if (compactSeen !== compact) {
    setCompactSeen(compact);
    setMenuOpen(false);
  }

  useEffect(() => {
    if (!menuOpen) {
      return undefined;
    }
    const panel = panelRef.current;
    const trigger = triggerRef.current;
    panel?.querySelector<HTMLElement>('a[href], button:not(:disabled)')?.focus();
    const closeAndRestore = () => {
      setMenuOpen(false);
      trigger?.focus();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        closeAndRestore();
      }
    };
    const onClick = (event: MouseEvent) => {
      const target = event.target;
      if (target instanceof Node && (panel?.contains(target) || trigger?.contains(target))) {
        return;
      }
      closeAndRestore();
    };
    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('click', onClick);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('click', onClick);
    };
  }, [menuOpen]);

  const signOut = () => {
    setMenuOpen(false);
    if (signingOut) {
      return;
    }
    setSigningOut(true);
    setSignOutFailed(false);
    void session
      .logout()
      .catch(() => setSignOutFailed(true))
      .finally(() => setSigningOut(false));
  };

  const closeMenu = () => setMenuOpen(false);

  const destinations = (
    <>
      <div className="lg-nav__group">
        <p className="lg-nav__group-label">{message('nav.sectionWorkspace')}</p>
        <NavEntry
          path={CATALOG_PATH}
          label={message('nav.catalog')}
          icon={<CatalogIcon />}
          onActivate={closeMenu}
        />
        {snapshot.state === 'authenticated' && (
          <NavEntry
            path={OWN_SHEET_PATH}
            label={message('nav.ownSheet')}
            icon={<SheetIcon />}
            onActivate={closeMenu}
          />
        )}
        {snapshot.state === 'authenticated' && (
          <NavEntry
            path={PLANS_PATH}
            label={message('nav.plans')}
            icon={<PlansIcon />}
            onActivate={closeMenu}
          />
        )}
        {snapshot.state === 'authenticated' && (
          <NavEntry
            path={CAMPAIGNS_PATH}
            label={message('nav.campaigns')}
            icon={<CampaignsIcon />}
            onActivate={closeMenu}
          />
        )}
        {snapshot.state === 'authenticated' && (
          <NavEntry
            path={ACCOUNT_PATH}
            label={message('nav.account')}
            icon={<AccountIcon />}
            onActivate={closeMenu}
          />
        )}
      </div>

      <div className="lg-nav__footer">
        {snapshot.state === 'authenticated' && (
          <div className="lg-nav__signout-wrap">
            <button
              type="button"
              className="lg-nav__action lg-nav__signout"
              onClick={signOut}
              disabled={signingOut}
              aria-busy={signingOut ? true : undefined}
              aria-describedby={signOutFailed ? signOutFailureId : undefined}
            >
              <span className="lg-nav__icon" aria-hidden="true">
                {signingOut ? <span className="lg-spinner" /> : <SignOutIcon />}
              </span>
              <span className="lg-nav__label">
                {signingOut ? message('common.signingOut') : message('nav.logout')}
              </span>
            </button>
            {signOutFailed ? (
              <p className="lg-nav__error" id={signOutFailureId} role="alert">
                {message('common.signOutFailed')}
              </p>
            ) : null}
          </div>
        )}
      </div>
    </>
  );

  return (
    <nav className="lg-nav" aria-label={message('nav.primaryLabel')}>
      <div className="lg-nav__rail">
        <div className="lg-nav__brand">
          <span className="lg-nav__mark" aria-hidden="true">
            <BrandMark />
          </span>
          <span className="lg-nav__brand-text">
            <span className="lg-nav__wordmark">{message('app.name')}</span>
            <span className="lg-nav__tagline">{message('nav.tagline')}</span>
          </span>
        </div>

        {compact ? (
          <>
            <button
              ref={triggerRef}
              type="button"
              className="lg-nav__menu-toggle"
              aria-label={message('nav.menu')}
              aria-expanded={menuOpen}
              aria-controls={menuId}
              onClick={() => setMenuOpen((open) => !open)}
            >
              <MenuIcon />
            </button>
            <div ref={panelRef} id={menuId} className="lg-nav__menu" hidden={!menuOpen}>
              {destinations}
            </div>
          </>
        ) : (
          destinations
        )}
      </div>
    </nav>
  );
}
