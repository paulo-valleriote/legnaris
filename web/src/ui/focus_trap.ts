export function focusTrap(container: HTMLElement) {
  const focusables = container.querySelectorAll<HTMLElement>(
    'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
  );
  if (focusables.length === 0) return { move: () => {}, release: () => {} };
  const first = focusables[0];
  const last = focusables[focusables.length - 1];
  const move = (dir: 'forward' | 'backward') => {
    const active = document.activeElement as HTMLElement;
    if (dir === 'forward' && (active === last || !container.contains(active))) first.focus();
    else if (dir === 'backward' && (active === first || !container.contains(active))) last.focus();
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Tab') {
      e.preventDefault();
      move(e.shiftKey ? 'backward' : 'forward');
    }
  };
  container.addEventListener('keydown', onKey);
  return { move, release: () => container.removeEventListener('keydown', onKey) };
}
