import { useEffect } from 'react';

/**
 * Paints the browser chrome in the stage ground while the landing is mounted. The document's own
 * `theme-color` pair follows the app theme, so a first, unconditional entry takes precedence and
 * is removed on unmount.
 */
export function useStageThemeColor() {
  useEffect(() => {
    const color = getComputedStyle(document.documentElement)
      .getPropertyValue('--lg-color-stage')
      .trim();
    if (color === '') {
      return undefined;
    }
    const meta = document.createElement('meta');
    meta.name = 'theme-color';
    meta.content = color;
    document.head.prepend(meta);
    return () => meta.remove();
  }, []);
}
