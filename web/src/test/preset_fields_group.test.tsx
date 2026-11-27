import { describe, expect, it } from 'vitest';
import { act, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { PresetFieldsPiece } from '../sheet/preset/compose/types';
import { FieldsGroup } from '../sheet/preset/compose/fields_group';
import { fifthCharacterSheet, fifthCharacterTemplate } from './fifth_fixtures';
import { renderPreset } from './preset_fixtures';

const TITLE = 'preset.fields.character' as const;

const armorChoices = [
  'armor.choiceNone',
  'armor.choiceLight',
  'armor.choiceMedium',
  'armor.choiceHeavy',
  'armor.choiceBarbarian',
  'armor.choiceMonk',
];

function fieldsPiece(
  paths: readonly string[],
  extras: Partial<PresetFieldsPiece> = {},
): PresetFieldsPiece {
  return {
    kind: 'fields',
    key: 'test',
    title: TITLE,
    fields: paths.map((path) => ({ path })),
    ...extras,
  };
}

function render(piece: PresetFieldsPiece) {
  return renderPreset(<FieldsGroup piece={piece} />, {
    template: fifthCharacterTemplate(),
    sheet: fifthCharacterSheet(),
  });
}

describe('the fields group piece', () => {
  it('Happy path: an exclusive select writes every sibling in one edit', async () => {
    const user = userEvent.setup();
    const { store } = render(fieldsPiece(armorChoices));

    await user.selectOptions(screen.getByRole('combobox', { name: 'Armor' }), 'armor.choiceLight');

    const armor = store.getState().data.armor as Record<string, number>;
    expect(armor.choiceNone).toBe(0);
    expect(armor.choiceLight).toBe(1);
    for (const path of ['choiceMedium', 'choiceHeavy', 'choiceBarbarian', 'choiceMonk'] as const) {
      expect(armor[path]).toBe(0);
    }
    expect(store.getPendingDelta()).toEqual({ armor: { choiceNone: 0, choiceLight: 1 } });
  });

  it('Happy path: numeric typing reports the parsed number to the store', async () => {
    const user = userEvent.setup();
    const { store } = render(fieldsPiece(['identity.level']));

    const input = screen.getByRole('textbox', { name: 'Level' });
    await user.clear(input);
    await user.type(input, '7');

    expect(store.getPendingDelta()).toEqual({ identity: { level: 7 } });
  });

  it('Edge: emptying a numeric field leaves nothing pending', async () => {
    const user = userEvent.setup();
    const { store } = render(fieldsPiece(['identity.level']));

    await user.clear(screen.getByRole('textbox', { name: 'Level' }));

    expect(store.getPendingDelta()).toEqual({});
  });

  it('Edge: a derived field draws a readout with no editing control', () => {
    render(fieldsPiece(['armor.armorClass']));

    const row = document.querySelector('[data-field-path="armor.armorClass"]');
    expect(row).not.toBeNull();
    expect(row).toHaveClass('lg-compose-fields__row--derived');
    expect(row?.querySelector('.lg-field__readonly--derived')).not.toBeNull();
    expect(row?.querySelector('input, select, textarea')).toBeNull();
  });

  it('Happy path: the summary follows the value it summarises', () => {
    const { store } = render(fieldsPiece(['armor.base'], { summary: 'armor.base' }));

    const group = screen.getByTestId('compose-fields-test');
    expect(group.querySelector('.lg-compose-fields__summary')).toHaveTextContent('10');

    act(() => store.setPath('armor.base', 12));

    expect(group.querySelector('.lg-compose-fields__summary')).toHaveTextContent('12');
  });

  it('Edge: an unknown path is ignored while the known siblings still draw', () => {
    render(fieldsPiece(['identity.level', 'ghost.field']));

    expect(screen.getByRole('textbox', { name: 'Level' })).toBeInTheDocument();
    expect(document.querySelector('[data-field-path="ghost.field"]')).toBeNull();
    expect(document.querySelectorAll('.lg-compose-fields__row')).toHaveLength(1);
  });

  it('Edge: an empty piece renders nothing', () => {
    const { container } = render(fieldsPiece([]));

    expect(container.firstElementChild).toBeNull();
    expect(screen.queryByTestId('compose-fields-test')).not.toBeInTheDocument();
  });
});
