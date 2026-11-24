import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { message } from '../l10n/strings';
import { fantasyPreset } from '../sheet/preset/fantasy';
import { PresetPlayView } from '../sheet/preset/play_view';
import { presetFor } from '../sheet/preset/registry';
import {
  FANTASY_SHEET_MODEL,
  fantasyTemplatePayload,
  FANTASY_TEMPLATE_ID,
} from './fantasy_fixtures';
import { renderPreset } from './preset_fixtures';
import { parseTemplateDetail } from '../api/models/template';

function fantasyTemplate() {
  return parseTemplateDetail(fantasyTemplatePayload());
}

function renderFantasy() {
  const preset = presetFor(fantasyTemplate());
  if (preset === undefined) {
    throw new Error('fantasy_v1 must resolve to the frozen preset');
  }
  const view = renderPreset(
    <>
      <preset.Header />
      <PresetPlayView preset={preset} />
    </>,
    { template: fantasyTemplate(), sheet: FANTASY_SHEET_MODEL },
  );
  return { ...view, preset };
}

describe('the frozen fantasy preset', () => {
  it('Happy path: the registry still resolves fantasy_v1 to the same preset object', () => {
    expect(presetFor(fantasyTemplate())).toBe(fantasyPreset);
  });

  it('Happy path: the preset keeps its frozen section order and fantasy surfaces', () => {
    const preset = presetFor(fantasyTemplate());

    expect(preset?.sections.map((section) => section.key)).toEqual([
      'combat',
      'attacks',
      'stats',
      'saves',
      'skills',
      'spells',
      'rules',
    ]);
    expect(preset?.Header).toBe(fantasyPreset.Header);
    expect(preset?.Vitals).toBe(fantasyPreset.Vitals);
  });

  it('Happy path: the landing demo still imports the frozen fantasy preset', () => {
    const source = readFileSync(path.resolve(process.cwd(), 'src/landing/demo_sheet.tsx'), 'utf8');

    expect(source).toContain("import { fantasyPreset } from '../sheet/preset/fantasy';");
    expect(source).toContain('<PresetPlayView preset={fantasyPreset} />');
  });

  it('Happy path: a fantasy Sheet fixture still renders every Play surface', () => {
    renderFantasy();

    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-hero')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-combat-strip')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-ability-strength')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-saves')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-skills')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-xp')).toBeInTheDocument();
  });

  it('Happy path: a fantasy_v1 Sheet still edits the character name in place', async () => {
    const user = userEvent.setup();
    const { store } = renderFantasy();

    const heading = screen.getByTestId('sheet-name');
    await user.click(within(heading).getByRole('button'));
    const input = within(heading).getByRole('textbox');
    await user.clear(input);
    await user.type(input, 'Eowyn{Enter}');

    expect(within(heading).getByRole('button')).toHaveTextContent('Eowyn');
    expect(store.getPendingDelta()).toMatchObject({ identity: { characterName: 'Eowyn' } });
  });

  it('Happy path: a fantasy_v1 Sheet still rolls an ability check', async () => {
    const user = userEvent.setup();
    renderFantasy();

    const ability = screen.getByTestId('sheet-ability-strength');
    await user.click(within(ability).getByRole('button', { name: /roll/i }));

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
  });

  it('Happy path: a fantasy_v1 Sheet still commits through the pending card', async () => {
    const user = userEvent.setup();
    renderFantasy();

    const heading = screen.getByTestId('sheet-name');
    await user.click(within(heading).getByRole('button'));
    const input = within(heading).getByRole('textbox');
    await user.clear(input);
    await user.type(input, 'Arwen{Enter}');

    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: message('sheet.pending.discard') }));

    expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
  });

  it('Edge: an unregistered Template id still has no preset beside the frozen fantasy one', () => {
    const template = { ...fantasyTemplate(), id: 'ghost_v1' };

    expect(presetFor(template)).toBeUndefined();
    expect(FANTASY_TEMPLATE_ID).toBe(fantasyPreset.templateId);
  });
});
