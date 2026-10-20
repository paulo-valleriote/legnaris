import { message } from '../l10n/strings';
import type { ThemeStore } from '../theme/theme';
import { useTheme } from '../theme/use_theme';

interface ThemeToggleProps {
  /** Injected store; the document-backed one is used by default. */
  readonly store?: ThemeStore;
}

/** The light-mode end of the switch. */
function SunIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
    >
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2" />
      <path d="M12 20v2" />
      <path d="M4.9 4.9 6.3 6.3" />
      <path d="m17.7 17.7 1.4 1.4" />
      <path d="M2 12h2" />
      <path d="M20 12h2" />
      <path d="m4.9 19.1 1.4-1.4" />
      <path d="m17.7 6.3 1.4-1.4" />
    </svg>
  );
}

/** The dark-mode end of the switch. */
function MoonIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
      <path d="M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5Z" />
    </svg>
  );
}

/**
 * The two-state light/dark control (UIF-18, UIF-27). Its accessible name is stable and its
 * pressed state reflects the resolved theme, while the visible row names the setting and the
 * switch body shows both ends — every string resolves through the copy catalog, never a literal.
 */
export function ThemeToggle({ store }: ThemeToggleProps = {}) {
  const { theme, toggle } = useTheme(store);
  const action = theme === 'dark' ? message('theme.switchToLight') : message('theme.switchToDark');

  return (
    <button
      type="button"
      className="lg-nav__action lg-nav__theme"
      aria-label={message('theme.toggleLabel')}
      aria-pressed={theme === 'dark'}
      title={action}
      onClick={toggle}
    >
      <span className="lg-nav__label">{message('theme.label')}</span>
      <span className="lg-nav__switch" aria-hidden="true">
        <span className="lg-nav__switch-icon lg-nav__switch-icon--sun">
          <SunIcon />
        </span>
        <span className="lg-nav__switch-icon lg-nav__switch-icon--moon">
          <MoonIcon />
        </span>
        <span className="lg-nav__switch-thumb" />
      </span>
    </button>
  );
}
