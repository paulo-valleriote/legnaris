import { message } from '../l10n/strings';
import type { ThemeStore } from '../theme/theme';
import { useTheme } from '../theme/use_theme';

interface ThemeToggleProps {
  /** Injected store; the document-backed one is used by default. */
  readonly store?: ThemeStore;
}

/**
 * The two-state light/dark control (UIF-18, UIF-27). Its accessible name is stable and its
 * pressed state reflects the resolved theme, while the visible copy names the action it will
 * perform next — both resolved through the copy catalog, never a literal.
 */
export function ThemeToggle({ store }: ThemeToggleProps = {}) {
  const { theme, toggle } = useTheme(store);
  const action = theme === 'dark' ? message('theme.switchToLight') : message('theme.switchToDark');

  return (
    <button
      type="button"
      className="lg-nav__theme"
      aria-label={message('theme.toggleLabel')}
      aria-pressed={theme === 'dark'}
      title={action}
      onClick={toggle}
    >
      {action}
    </button>
  );
}
