const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

function focusablesIn(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (element) => !element.hasAttribute('disabled'),
  );
}

/**
 * focusTrap keeps Tab inside `container`: Tab on the last focusable wraps to the first and
 * Shift+Tab on the first wraps to the last, while Tab between them keeps the browser's own
 * order. The focusables are read on every key, so content that changes while open is covered.
 */
export function focusTrap(container: HTMLElement) {
  const move = (dir: 'forward' | 'backward'): boolean => {
    const focusables = focusablesIn(container);
    if (focusables.length === 0) return false;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const active = document.activeElement;
    const outside = !container.contains(active) || active === container;
    if (dir === 'forward' && (active === last || outside)) {
      first.focus();
      return true;
    }
    if (dir === 'backward' && (active === first || outside)) {
      last.focus();
      return true;
    }
    return false;
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key !== 'Tab') return;
    if (move(e.shiftKey ? 'backward' : 'forward')) e.preventDefault();
  };
  container.addEventListener('keydown', onKey);
  return {
    move: (dir: 'forward' | 'backward') => {
      move(dir);
    },
    release: () => container.removeEventListener('keydown', onKey),
  };
}
