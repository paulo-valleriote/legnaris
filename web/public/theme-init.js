/*
 * Resolves the theme before the first paint (UIF-20). A same-origin classic script in the
 * document head, so `script-src 'self'` already permits it; it introduces no inline script.
 * The accepted values and the fallback match `src/theme/theme.ts`.
 */
(function () {
  var KEY = 'lg.theme';
  var theme = null;
  try {
    var stored = window.localStorage.getItem(KEY);
    if (stored === 'light' || stored === 'dark') {
      theme = stored;
    }
  } catch {
    theme = null;
  }
  if (theme === null) {
    try {
      theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    } catch {
      theme = 'light';
    }
  }
  document.documentElement.setAttribute('data-theme', theme);
})();
