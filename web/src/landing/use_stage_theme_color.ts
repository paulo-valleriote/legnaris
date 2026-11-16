import { useEffect } from 'react';

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
