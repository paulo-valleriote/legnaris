import { editName } from './dice_names';
import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { IdentityHeader } from '../sheet/preset/compose/identity_header';
import { Pips } from '../sheet/preset/compose/pips';
import { StatPills } from '../sheet/preset/compose/stat_pills';
import { PresetPlayView } from '../sheet/preset/play_view';
import { presetConfig, presetSheet, renderPreset } from './preset_fixtures';

const identity = { name: 'identity.name', subtitle: 'identity.subtitle' };

function statPills(paths: readonly string[]) {
  return { kind: 'statPills' as const, key: 'vitals', pills: paths.map((path) => ({ path })) };
}

function pips(count: string, max: string | number) {
  return { kind: 'pips' as const, key: 'charges', count, max };
}

describe('the identity header piece', () => {
  it('Happy path: level 1 draws an h1 with the name and subtitle Fields', () => {
    renderPreset(<IdentityHeader header={identity} />);

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Nova');
    expect(screen.getByRole('button', { name: editName('Subtitle') })).toHaveTextContent('Scout');
  });

  it('Happy path: level 2 draws an h2 for a subheading context', () => {
    renderPreset(<IdentityHeader header={identity} level={2} />);

    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Nova');
  });

  it('Edge: an empty name falls back to the sheet name', () => {
    renderPreset(<IdentityHeader header={identity} />, {
      sheet: presetSheet({ data: { identity: { name: '', subtitle: '' } } }),
    });

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Sheet Nova');
  });

  it('Edge: an omitted subtitle Field draws no subtitle control', () => {
    renderPreset(<IdentityHeader header={{ name: 'identity.name' }} />);

    expect(screen.queryByRole('button', { name: editName('Subtitle') })).not.toBeInTheDocument();
  });

  it('Happy path: the name and subtitle draw bordered controls with no pencil icon', () => {
    renderPreset(<IdentityHeader header={identity} />);

    for (const label of ['Name', 'Subtitle']) {
      const control = screen.getByRole('button', { name: editName(label) });
      expect(control).toHaveClass('lg-inline-edit');
      expect(control).toHaveAccessibleName(editName(label));
      expect(control.querySelector('.lg-inline-edit__pencil')).toBeNull();
    }
  });

  it('Happy path: the name affordance still opens the inline editor', async () => {
    const user = userEvent.setup();
    renderPreset(<IdentityHeader header={identity} />);

    await user.click(screen.getByRole('button', { name: editName('Name') }));

    expect(screen.getByRole('textbox', { name: 'Name' })).toBeInTheDocument();
  });

  it('Happy path: Enter commits the name through the edit store', async () => {
    const user = userEvent.setup();
    renderPreset(<IdentityHeader header={identity} />);

    await user.click(screen.getByRole('button', { name: editName('Name') }));
    const input = screen.getByRole('textbox', { name: 'Name' });
    await user.clear(input);
    await user.type(input, 'Rune{Enter}');

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Rune');
  });

  it('Happy path: the name is editable from the keyboard alone', async () => {
    const user = userEvent.setup();
    renderPreset(<IdentityHeader header={identity} />);

    screen.getByRole('button', { name: editName('Name') }).focus();
    await user.keyboard('{Enter}');
    const input = screen.getByRole('textbox', { name: 'Name' });
    await user.clear(input);
    await user.type(input, 'Kestrel{Enter}');

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Kestrel');
  });
});

describe('the stat pills piece', () => {
  it('Happy path: each bound Field draws its original label and stored value', () => {
    renderPreset(<StatPills piece={statPills(['vitals.armor', 'vitals.hit'])} />);

    expect(screen.getByText('Armor Class')).toBeInTheDocument();
    expect(screen.getByText('14')).toBeInTheDocument();
    expect(screen.getByText('Hit Points')).toBeInTheDocument();
    expect(screen.getByText('12')).toBeInTheDocument();
  });

  it('Happy path: a Row with dice draws a RollableValue, one without draws plain text', () => {
    renderPreset(<StatPills piece={statPills(['vitals.armor', 'vitals.hit'])} />);

    expect(screen.getByRole('button', { name: /Armor Class/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Hit Points/ })).not.toBeInTheDocument();
  });

  it('Happy path: the rollable pill rolls from the keyboard', async () => {
    const user = userEvent.setup();
    renderPreset(<StatPills piece={statPills(['vitals.armor'])} />);

    screen.getByRole('button', { name: /Armor Class/ }).focus();
    await user.keyboard('{Enter}');

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Edge: an empty value draws the shared empty-value copy', () => {
    renderPreset(<StatPills piece={statPills(['vitals.empty'])} />);

    expect(screen.getByText(message('sheet.play.noValue'))).toBeInTheDocument();
  });

  it('Edge: no bound pill draws nothing', () => {
    renderPreset(<StatPills piece={statPills([])} />);

    expect(screen.queryByTestId('compose-stat-pills')).not.toBeInTheDocument();
  });
});

describe('the pips piece', () => {
  it('Happy path: the max Field sets the pip count and the count Field sets the filled pips', () => {
    renderPreset(<Pips piece={pips('charges.used', 'charges.limit')} />);

    const buttons = screen.getAllByRole('button', { name: /Charges Used/ });
    expect(buttons).toHaveLength(4);
    expect(buttons.filter((button) => button.getAttribute('aria-pressed') === 'true')).toHaveLength(
      1,
    );
    expect(screen.getByRole('group', { name: /Charges Used/ })).toBeInTheDocument();
  });

  it('Happy path: a pip commits the new count through the edit store', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<Pips piece={pips('charges.used', 'charges.limit')} />);

    await user.click(screen.getAllByRole('button', { name: /Charges Used/ })[2]);

    expect(store.getPendingDelta()).toEqual({ charges: { used: 3 } });
  });

  it('Edge: activating the last filled pip toggles it back down', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<Pips piece={pips('charges.used', 'charges.limit')} />);

    const buttons = screen.getAllByRole('button', { name: /Charges Used/ });
    await user.click(buttons[2]);
    await user.click(buttons[2]);

    expect(store.getPendingDelta()).toEqual({ charges: { used: 2 } });
  });

  it('Happy path: the pips are operable from the keyboard', async () => {
    const user = userEvent.setup();
    const { store } = renderPreset(<Pips piece={pips('charges.used', 'charges.limit')} />);

    screen.getAllByRole('button', { name: /Charges Used/ })[2].focus();
    await user.keyboard('{Enter}');

    expect(store.getPendingDelta()).toEqual({ charges: { used: 3 } });
  });

  it('Edge: a numeric maximum needs no Field', () => {
    renderPreset(<Pips piece={pips('charges.used', 3)} />);

    expect(screen.getAllByRole('button', { name: /Charges Used/ })).toHaveLength(3);
  });

  it('Edge: a missing maximum Field draws no pips', () => {
    renderPreset(<Pips piece={pips('charges.used', 'ghost.max')} />);

    expect(screen.queryAllByRole('button', { name: /Charges Used/ })).toHaveLength(0);
  });

  it('Edge: a count above the maximum fills every pip', () => {
    renderPreset(<Pips piece={pips('charges.used', 'charges.limit')} />, {
      sheet: presetSheet({ data: { charges: { used: 9, limit: 4 } } }),
    });

    const buttons = screen.getAllByRole('button', { name: /Charges Used/ });
    expect(buttons.every((button) => button.getAttribute('aria-pressed') === 'true')).toBe(true);
  });
});

describe('the config adapter in the Play view', () => {
  it('Happy path: a config renders its header and ordered pieces unchanged', () => {
    const preset = presetFromConfig(presetConfig());

    renderPreset(
      <>
        <preset.Header />
        <PresetPlayView preset={preset} />
      </>,
    );

    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Nova');
    expect(screen.getByText('Armor Class')).toBeInTheDocument();
    expect(screen.getByRole('group', { name: /Charges Used/ })).toBeInTheDocument();
  });
});
