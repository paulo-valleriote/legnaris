import { editName } from './dice_names';
import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { configPathErrors } from '../sheet/preset/compose/config_paths';
import { TextFields } from '../sheet/preset/compose/text_fields';
import type { PresetTextField, PresetTextFieldsPiece } from '../sheet/preset/compose/types';
import { presetConfig, presetSheet, presetTemplateDetail, renderPreset } from './preset_fixtures';

function fields(paths: readonly string[]): PresetTextFieldsPiece {
  return {
    kind: 'textFields',
    key: 'texts',
    fields: paths.map((path): PresetTextField => ({ path })),
  };
}

describe('the text fields piece', () => {
  it('Happy path: each bound text leaf draws its original label and stored value', () => {
    renderPreset(<TextFields piece={fields(['identity.name', 'identity.subtitle'])} />);

    expect(screen.getByRole('button', { name: editName('Name') })).toHaveTextContent('Nova');
    expect(screen.getByRole('button', { name: editName('Subtitle') })).toHaveTextContent('Scout');
  });

  it('Happy path: a text leaf is editable inline through the edit store', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<TextFields piece={fields(['identity.name'])} />);

    await user.click(screen.getByRole('button', { name: editName('Name') }));
    const input = screen.getByRole('textbox', { name: 'Name' });
    await user.clear(input);
    await user.type(input, 'Rune{Enter}');

    expect(store.getPendingDelta()).toEqual({ identity: { name: 'Rune' } });
  });

  it('Edge: an empty piece draws nothing', () => {
    renderPreset(<TextFields piece={fields([])} />);

    expect(screen.queryByTestId('compose-text-fields')).not.toBeInTheDocument();
  });

  it('Edge: a missing value draws the shared inline add copy', () => {
    renderPreset(<TextFields piece={fields(['identity.subtitle'])} />, {
      sheet: presetSheet({ data: { identity: { name: 'Nova', subtitle: '' } } }),
    });

    expect(screen.getByRole('button', { name: editName('Subtitle') })).toHaveTextContent(
      'Add Subtitle',
    );
  });

  it('Happy path: the piece binds every listed leaf through configPathErrors', () => {
    const config = presetConfig({ pieces: [fields(['identity.name', 'identity.subtitle'])] });

    expect(configPathErrors(config, presetTemplateDetail())).toEqual([]);
  });

  it('Error: an unknown text path is reported', () => {
    const config = presetConfig({ pieces: [fields(['details.ghost'])] });

    expect(configPathErrors(config, presetTemplateDetail())).toEqual(['details.ghost']);
  });
});
