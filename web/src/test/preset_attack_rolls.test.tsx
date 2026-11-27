import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { parseTemplateDetail } from '../api/models/template';
import { DiceProvider } from '../sheet/dice_row';
import { PresetProvider } from '../sheet/preset/preset_context';
import { RecordTable } from '../sheet/preset/compose/record_table';
import type { PresetRecordTablePiece } from '../sheet/preset/compose/types';
import { fifthCharacterSheet, fifthCharacterTemplate } from './fifth_fixtures';
import { createPresetRuntime, presetSheet } from './preset_fixtures';

const RECORD = 'r0123456789abcdef0123456789abcdef';

const PIECE: PresetRecordTablePiece = {
  kind: 'recordTable',
  key: 'attacks',
  list: 'attacks.list',
  nameColumn: 'name',
  rollColumn: 'toHit',
  damageColumn: 'damage',
};

function renderLocked(
  template = fifthCharacterTemplate(),
  sheet = fifthCharacterSheet(),
  piece = PIECE,
) {
  const runtime = createPresetRuntime(template, sheet);
  return render(
    <PresetProvider {...runtime.preset} editing={false}>
      <DiceProvider value={runtime.dice}>
        <RecordTable piece={piece} />
      </DiceProvider>
    </PresetProvider>,
  );
}

describe('attack rolls in a record table', () => {
  it('Happy path: the attack roll sits on the attack name and carries its to-hit bonus', async () => {
    const user = userEvent.setup();
    renderLocked();

    await user.click(screen.getByRole('button', { name: 'Attack roll, Longbow, +0' }));

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Happy path: a damage expression rolls from its own value', async () => {
    const user = userEvent.setup();
    renderLocked();

    await user.click(screen.getByRole('button', { name: 'Damage roll, Longbow, 1d8 + 1' }));

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Edge: damage text that is not a dice expression stays plain text', () => {
    renderLocked(
      plainAttacksTemplate(),
      presetSheet({
        data: { attacks: { [RECORD]: { name: 'Longsword', damage: '5 slashing' } } },
      }),
      { ...PIECE, list: 'attacks' },
    );

    expect(screen.queryByRole('button', { name: /^Damage roll/ })).toBeNull();
    expect(screen.getByText('5 slashing')).toBeInTheDocument();
  });

  it('Edge: a list without a dice expression keeps its table label, not an attack roll', () => {
    renderLocked(
      plainAttacksTemplate(false),
      presetSheet({
        data: { attacks: { [RECORD]: { name: 'Longsword', damage: '1d8 + 1' } } },
      }),
      { ...PIECE, list: 'attacks' },
    );

    expect(screen.queryByRole('button', { name: /^Attack roll/ })).toBeNull();
    expect(screen.queryByRole('button', { name: /^Damage roll/ })).toBeNull();
  });
});

function plainAttacksTemplate(withDice = true) {
  return parseTemplateDetail({
    id: 'compose_attacks_v1',
    name: 'Attacks',
    default: false,
    kinds: ['character'],
    fields: {
      attacks: {
        list: {
          label: 'Attacks',
          capacity: 4,
          columns: ['name', 'toHit', 'damage'],
          shape: {
            name: { schema: { type: 'text', label: 'Name', is_editable: true, default: 0 } },
            toHit: {
              schema: { type: 'number', label: 'To hit', is_editable: false, default: 0 },
            },
            damage: { schema: { type: 'text', label: 'Damage', is_editable: true, default: 0 } },
          },
          ...(withDice ? { dice: '1d20 + @.toHit' } : {}),
        },
      },
    },
  });
}
