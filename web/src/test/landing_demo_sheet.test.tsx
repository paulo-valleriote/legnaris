import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import FifthDemoSheet from '../landing/fifth_demo_sheet';
import { message } from '../l10n/strings';
import { rollName } from './dice_names';

const WISDOM_SCORE = 'abilities.wisdom.score';

function wisdomCell() {
  return screen.getByTestId(`compose-ability-${WISDOM_SCORE}`);
}

async function editWisdom(user: ReturnType<typeof userEvent.setup>, text: string) {
  const cell = wisdomCell();
  await user.click(within(cell).getByRole('button', { name: 'Wisdom' }));
  const input = within(cell).getByRole('textbox', { name: 'Wisdom' });
  await user.clear(input);
  await user.type(input, `${text}{Enter}`);
}

async function changeHitPoints(
  user: ReturnType<typeof userEvent.setup>,
  amount: string,
  action: 'damage' | 'heal',
) {
  const card = screen.getByTestId('compose-hit-points');
  const input = within(card).getByLabelText(message('sheet.play.amount'));
  await user.clear(input);
  await user.type(input, amount);
  await waitFor(() => expect(input).toHaveValue(amount));
  await user.click(
    within(card).getByRole('button', {
      name: message(action === 'damage' ? 'sheet.play.damage' : 'sheet.play.heal'),
    }),
  );
}

describe('the landing 5E demo sheet', () => {
  const fetchSpy = vi.fn(() => Promise.reject(new Error('the demo must stay offline')));

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchSpy);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchSpy.mockClear();
  });

  it('Happy path: editing Wisdom recalculates the modifier in place', async () => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);

    expect(
      within(wisdomCell()).getByRole('button', { name: rollName('Wisdom modifier') }),
    ).toHaveTextContent('+3');

    await editWisdom(user, '18');

    expect(
      within(wisdomCell()).getByRole('button', { name: rollName('Wisdom modifier') }),
    ).toHaveTextContent('+4');
    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: the pending card commits the demo locally', async () => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);

    await editWisdom(user, '18');
    const card = screen.getByTestId('sheet-pending-card');
    await user.click(within(card).getByRole('button', { name: message('sheet.pending.save') }));

    await waitFor(() => expect(screen.queryByTestId('sheet-pending-card')).toBeNull());
    expect(
      within(wisdomCell()).getByRole('button', { name: rollName('Wisdom modifier') }),
    ).toHaveTextContent('+4');
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: damage and healing work on the local hit points', async () => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);

    await changeHitPoints(user, '5', 'damage');
    expect(
      within(screen.getByTestId('compose-hit-points')).getByRole('button', {
        name: 'Current hit points',
      }),
    ).toHaveTextContent('19');

    await changeHitPoints(user, '5', 'damage');
    expect(
      within(screen.getByTestId('compose-hit-points')).getByRole('button', {
        name: 'Current hit points',
      }),
    ).toHaveTextContent('14');

    await changeHitPoints(user, '5', 'heal');
    expect(
      within(screen.getByTestId('compose-hit-points')).getByRole('button', {
        name: 'Current hit points',
      }),
    ).toHaveTextContent('19');
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: rolling a die opens the shared dice panel', async () => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);

    await user.click(
      within(wisdomCell()).getByRole('button', { name: rollName('Wisdom modifier') }),
    );

    expect(document.querySelector('.lg-dice-panel')).not.toBeNull();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: an edit and a roll each report the first demo interaction', async () => {
    const onInteract = vi.fn();
    const user = userEvent.setup();
    render(<FifthDemoSheet onInteract={onInteract} />);

    expect(onInteract).not.toHaveBeenCalled();

    await editWisdom(user, '18');
    expect(onInteract).toHaveBeenCalledTimes(1);

    await user.click(
      within(wisdomCell()).getByRole('button', { name: rollName('Wisdom modifier') }),
    );
    expect(onInteract).toHaveBeenCalledTimes(2);
  });

  it('Edge: a rerender hands later interactions to the latest callback', async () => {
    const first = vi.fn();
    const latest = vi.fn();
    const user = userEvent.setup();
    const view = render(<FifthDemoSheet onInteract={first} />);

    view.rerender(<FifthDemoSheet onInteract={latest} />);

    await editWisdom(user, '18');

    expect(latest).toHaveBeenCalled();
    expect(first).not.toHaveBeenCalled();
  });

  it('Edge: every mount starts from the sample character again', async () => {
    const user = userEvent.setup();
    const first = render(<FifthDemoSheet />);

    await editWisdom(user, '18');
    first.unmount();

    render(<FifthDemoSheet />);

    expect(
      within(wisdomCell()).getByRole('button', { name: rollName('Wisdom modifier') }),
    ).toHaveTextContent('+3');
    expect(screen.queryByTestId('sheet-pending-card')).toBeNull();
  });
});
