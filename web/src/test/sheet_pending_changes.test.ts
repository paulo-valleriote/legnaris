import { describe, expect, it } from 'vitest';
import { message } from '../l10n/strings';
import { displayValue, pendingChanges } from '../sheet/preset/pending_changes';

describe('pending changes', () => {
  it('Edge: no edited path yields no change', () => {
    expect(pendingChanges({ identity: { level: 3 } }, { identity: { level: 3 } }, [])).toEqual([]);
  });

  it('Edge: a path edited back to its stored value is omitted', () => {
    const changes = pendingChanges({ identity: { level: 3 } }, { identity: { level: 3 } }, [
      'identity.level',
    ]);

    expect(changes).toEqual([]);
  });

  it('Edge: a path missing on both sides reads as the empty copy and is omitted', () => {
    expect(pendingChanges({}, {}, ['identity.origin'])).toEqual([]);
  });

  it('Edge: a path missing before and set after shows the empty copy as the old value', () => {
    expect(pendingChanges({}, { identity: { origin: 'Wanderer' } }, ['identity.origin'])).toEqual([
      { path: 'identity.origin', before: message('sheet.pending.emptyValue'), after: 'Wanderer' },
    ]);
  });

  it('Happy path: each edited path keeps its order and shows the old and new text', () => {
    const changes = pendingChanges(
      { identity: { level: 3, characterName: 'Aria' } },
      { identity: { level: 4, characterName: 'Brenna' } },
      ['identity.characterName', 'identity.level'],
    );

    expect(changes).toEqual([
      { path: 'identity.characterName', before: 'Aria', after: 'Brenna' },
      { path: 'identity.level', before: '3', after: '4' },
    ]);
  });
});

describe('pending change values', () => {
  it('Edge: empty, null and undefined all read as the empty copy', () => {
    const empty = message('sheet.pending.emptyValue');
    expect(displayValue('')).toBe(empty);
    expect(displayValue('   ')).toBe(empty);
    expect(displayValue(null)).toBe(empty);
    expect(displayValue(undefined)).toBe(empty);
  });

  it('Edge: a structured value reads as the empty copy instead of an object dump', () => {
    expect(displayValue({ nested: true })).toBe(message('sheet.pending.emptyValue'));
    expect(displayValue(['a'])).toBe(message('sheet.pending.emptyValue'));
  });

  it('Happy path: numbers, booleans and text render as plain text', () => {
    expect(displayValue(0)).toBe('0');
    expect(displayValue(-2.5)).toBe('-2.5');
    expect(displayValue(false)).toBe('false');
    expect(displayValue('Brenna')).toBe('Brenna');
  });
});
