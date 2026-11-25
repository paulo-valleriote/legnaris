import { describe, expect, it } from 'vitest';
import { nameWithVisibleText } from '../sheet/inline_edit';

describe('nameWithVisibleText', () => {
  it.each([
    {
      name: 'Happy path: appends the visible value after the label',
      label: 'Strength',
      visible: '12',
      expected: 'Strength, 12',
    },
    {
      name: 'Happy path: a roll name gains its visible total',
      label: 'Roll Initiative, d20',
      visible: '+0',
      expected: 'Roll Initiative, d20, +0',
    },
    {
      name: 'Edge: empty visible text keeps the name',
      label: 'Strength',
      visible: '',
      expected: 'Strength',
    },
    {
      name: 'Edge: whitespace visible text keeps the name',
      label: 'Strength',
      visible: '   ',
      expected: 'Strength',
    },
    {
      name: 'Edge: visible text already inside the name is not repeated',
      label: 'Add Ancestry',
      visible: 'Add Ancestry',
      expected: 'Add Ancestry',
    },
    {
      name: 'Edge: surrounding whitespace is trimmed before appending',
      label: 'Speed',
      visible: ' 30 ',
      expected: 'Speed, 30',
    },
    {
      name: 'Edge: a value that only appears inside another word is still appended',
      label: 'Roll Initiative, d20',
      visible: '0',
      expected: 'Roll Initiative, d20, 0',
    },
    {
      name: 'Edge: regex characters in the visible text are matched literally',
      label: 'Bonus',
      visible: '+2 (x)',
      expected: 'Bonus, +2 (x)',
    },
  ])('$name', ({ label, visible, expected }) => {
    expect(nameWithVisibleText(label, visible)).toBe(expected);
  });
});
