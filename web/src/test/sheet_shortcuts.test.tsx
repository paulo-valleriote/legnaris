import { describe, expect, it, vi } from 'vitest';
import { render } from '@testing-library/react';
import { dispatchShortcut, matchShortcut, useSheetShortcuts } from '../shell/shortcut_registry';

function keyEvent(init: KeyboardEventInit & { key: string }): KeyboardEvent {
  return new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init });
}

function Harness({ onSave, onRefresh }: { onSave: () => void; onRefresh?: () => void }) {
  useSheetShortcuts({ onSave, onRefresh });
  return null;
}

describe('shortcut registry (WSC-20)', () => {
  it('Edge: Ctrl+S and Cmd+S resolve to saveSheet (WSC-20)', () => {
    expect(matchShortcut(keyEvent({ key: 's', ctrlKey: true }))).toBe('saveSheet');
    expect(matchShortcut(keyEvent({ key: 'S', metaKey: true }))).toBe('saveSheet');
  });

  it('Edge: F5 resolves to refreshSheet (WSC-36)', () => {
    expect(matchShortcut(keyEvent({ key: 'F5' }))).toBe('refreshSheet');
  });

  it('Edge: an unrelated key resolves to null (WSC-20)', () => {
    expect(matchShortcut(keyEvent({ key: 'a' }))).toBeNull();
    expect(matchShortcut(keyEvent({ key: 's' }))).toBeNull();
    expect(matchShortcut(keyEvent({ key: 'F5', ctrlKey: true }))).toBeNull();
  });

  it('Edge: dispatchShortcut with no mounted sheet runs nothing (WSC-20)', () => {
    expect(() => dispatchShortcut('saveSheet')).not.toThrow();
    expect(() => dispatchShortcut('refreshSheet')).not.toThrow();
  });

  it('Happy path: a mounted handler receives Ctrl+S and the browser default is prevented (WSC-20)', () => {
    const onSave = vi.fn();
    render(<Harness onSave={onSave} />);

    const event = keyEvent({ key: 's', ctrlKey: true });
    document.dispatchEvent(event);

    expect(onSave).toHaveBeenCalledTimes(1);
    expect(event.defaultPrevented).toBe(true);
  });

  it('Edge: the registered handler is gone after unmount (WSC-20)', () => {
    const onSave = vi.fn();
    const { unmount } = render(<Harness onSave={onSave} />);
    unmount();

    document.dispatchEvent(keyEvent({ key: 's', ctrlKey: true }));

    expect(onSave).not.toHaveBeenCalled();
  });
});
