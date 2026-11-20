import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import DemoSheet from '../landing/demo_sheet';

const here = path.dirname(fileURLToPath(import.meta.url));
import { message } from '../l10n/strings';
import { rollName } from './dice_names';

function medallion(ability: string) {
  return screen.getByTestId(`sheet-ability-${ability}`);
}

async function editAttribute(
  user: ReturnType<typeof userEvent.setup>,
  ability: string,
  text: string,
) {
  const card = medallion(ability);
  await user.click(within(card).getByRole('button', { name: 'Might' }));
  const input = within(card).getByRole('textbox', { name: 'Might' });
  await user.clear(input);
  await user.type(input, `${text}{Enter}`);
}

async function changeHitPoints(
  user: ReturnType<typeof userEvent.setup>,
  amount: string,
  action: 'damage' | 'heal',
) {
  const card = screen.getByTestId('sheet-hp-card');
  const input = within(card).getByLabelText(message('sheet.play.amount'));
  await user.clear(input);
  await user.type(input, amount);
  await user.click(
    within(card).getByRole('button', {
      name: message(action === 'damage' ? 'sheet.play.damage' : 'sheet.play.heal'),
    }),
  );
}

describe('the landing demo sheet', () => {
  const fetchSpy = vi.fn(() => Promise.reject(new Error('the demo must stay offline')));

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchSpy);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchSpy.mockClear();
  });

  it('Happy path: it draws the real Fantasy Play components over the sample character', () => {
    render(<DemoSheet />);

    expect(screen.getByTestId('sheet-play')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-hero')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-combat-strip')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-ability-strength')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-saves')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-play-skills')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-attacks')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-spells')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-table-rules')).toBeInTheDocument();
    expect(within(medallion('strength')).getByText('Might')).toBeInTheDocument();
    expect(screen.queryByText('Strength')).not.toBeInTheDocument();
  });

  it('Happy path: editing an attribute recalculates the modifier in place', async () => {
    const user = userEvent.setup();
    render(<DemoSheet />);

    expect(
      within(medallion('strength')).getByRole('button', { name: rollName('Might modifier') }),
    ).toHaveTextContent('+3');

    await editAttribute(user, 'strength', '18');

    expect(
      within(medallion('strength')).getByRole('button', { name: rollName('Might modifier') }),
    ).toHaveTextContent('+4');
    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: the pending card commits the demo locally', async () => {
    const user = userEvent.setup();
    render(<DemoSheet />);

    await editAttribute(user, 'strength', '18');
    const card = screen.getByTestId('sheet-pending-card');
    await user.click(within(card).getByRole('button', { name: message('sheet.pending.save') }));

    await waitFor(() => expect(screen.queryByTestId('sheet-pending-card')).toBeNull());
    expect(
      within(medallion('strength')).getByRole('button', { name: rollName('Might modifier') }),
    ).toHaveTextContent('+4');
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: damage and healing work on the local hit points', async () => {
    const user = userEvent.setup();
    render(<DemoSheet />);

    await changeHitPoints(user, '5', 'damage');
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent(/19\s*\/\s*30/);

    await changeHitPoints(user, '5', 'heal');
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent(/24\s*\/\s*30/);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: rolling a die opens the shared dice panel', async () => {
    const user = userEvent.setup();
    render(<DemoSheet />);

    await user.click(screen.getByRole('button', { name: rollName('Might modifier') }));

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Edge: the demo is a separate module with a default export', async () => {
    const module = await import('../landing/demo_sheet');
    const landing = readFileSync(path.resolve(here, '../landing/landing_screen.tsx'), 'utf8');

    expect(module.default).toBeTypeOf('function');
    expect(landing).not.toContain("from './demo_sheet'");
    expect(landing).toContain("lazy(() => import('./demo_sheet'))");
  });

  it('Happy path: an edit and a roll each report the first demo interaction', async () => {
    const onInteract = vi.fn();
    const user = userEvent.setup();
    render(<DemoSheet onInteract={onInteract} />);

    expect(onInteract).not.toHaveBeenCalled();

    await editAttribute(user, 'strength', '18');
    expect(onInteract).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole('button', { name: rollName('Might modifier') }));
    expect(onInteract).toHaveBeenCalledTimes(2);
  });

  it('Edge: the demo runs without an interaction callback', async () => {
    const user = userEvent.setup();
    render(<DemoSheet />);

    await changeHitPoints(user, '5', 'damage');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent(/19\s*\/\s*30/);
  });

  it('Edge: every mount starts from the sample character again', async () => {
    const user = userEvent.setup();
    const first = render(<DemoSheet />);

    await editAttribute(user, 'strength', '18');
    first.unmount();

    render(<DemoSheet />);

    expect(
      within(medallion('strength')).getByRole('button', { name: rollName('Might modifier') }),
    ).toHaveTextContent('+3');
    expect(screen.queryByTestId('sheet-pending-card')).toBeNull();
  });
});
