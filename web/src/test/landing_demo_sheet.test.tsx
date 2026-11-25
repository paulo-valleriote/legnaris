import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import FifthDemoSheet from '../landing/fifth_demo_sheet';
import { message } from '../l10n/strings';
import { rollName, editName } from './dice_names';

const WISDOM_SCORE = 'abilities.wisdom.score';

function wisdomCell() {
  return screen.getByTestId(`compose-ability-${WISDOM_SCORE}`);
}

async function editWisdom(user: ReturnType<typeof userEvent.setup>, text: string) {
  const cell = wisdomCell();
  await user.click(within(cell).getByRole('button', { name: editName('Wisdom') }));
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
      name: editName(message(action === 'damage' ? 'sheet.play.damage' : 'sheet.play.heal')),
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
    await user.click(
      within(card).getByRole('button', { name: editName(message('sheet.pending.save')) }),
    );

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
        name: editName('Current hit points'),
      }),
    ).toHaveTextContent('19');

    await changeHitPoints(user, '5', 'damage');
    expect(
      within(screen.getByTestId('compose-hit-points')).getByRole('button', {
        name: editName('Current hit points'),
      }),
    ).toHaveTextContent('14');

    await changeHitPoints(user, '5', 'heal');
    expect(
      within(screen.getByTestId('compose-hit-points')).getByRole('button', {
        name: editName('Current hit points'),
      }),
    ).toHaveTextContent('19');
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it('Happy path: undo after damage clears the pending hit point change', async () => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);

    await changeHitPoints(user, '5', 'damage');
    expect(screen.getByTestId('sheet-pending-card')).toHaveTextContent(
      'Current hit points 24 → 19',
    );

    await user.click(screen.getByRole('button', { name: editName(message('sheet.play.hpUndo')) }));

    expect(screen.queryByTestId('sheet-pending-card')).toBeNull();
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
  it.each([
    {
      name: 'Edge: Damage and Heal stay disabled while the amount is empty',
      amount: '',
      enabled: false,
    },
    { name: 'Edge: Damage and Heal stay disabled for a zero amount', amount: '0', enabled: false },
    {
      name: 'Error: Damage and Heal stay disabled for text that is not a number',
      amount: 'abc',
      enabled: false,
    },
    { name: 'Happy path: a positive amount enables Damage and Heal', amount: '4', enabled: true },
  ] as const)('$name', async ({ amount, enabled }) => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);
    const card = screen.getByTestId('compose-hit-points');
    if (amount !== '') {
      await user.type(within(card).getByLabelText(message('sheet.play.amount')), amount);
    }

    for (const key of ['sheet.play.damage', 'sheet.play.heal'] as const) {
      const button = within(card).getByRole('button', { name: editName(message(key)) });
      if (enabled) {
        expect(button).toBeEnabled();
      } else {
        expect(button).toBeDisabled();
      }
    }
  });

  it('Edge: Enter with an empty amount changes no hit points', async () => {
    const user = userEvent.setup();
    render(<FifthDemoSheet />);
    const card = screen.getByTestId('compose-hit-points');
    await user.type(within(card).getByLabelText(message('sheet.play.amount')), '{Enter}');

    expect(screen.queryByTestId('sheet-pending-card')).toBeNull();
  });

  it('Happy path: an ability score shows its edit pencil next to the rollable modifier', () => {
    render(<FifthDemoSheet />);

    const score = within(wisdomCell()).getByRole('button', { name: editName('Wisdom') });
    expect(score).toHaveClass('lg-inline-edit--affordance');
    expect(score.querySelector('.lg-inline-edit__pencil')).not.toBeNull();
  });
  it('Happy path: the hit-point card keeps temporary hit points under the meter', () => {
    render(<FifthDemoSheet />);
    const card = screen.getByTestId('compose-hit-points');

    const meter = within(card).getByRole('meter', { name: message('sheet.play.hitPoints') });
    const temp = card.querySelector('.lg-compose-hp__temp');
    expect(temp).not.toBeNull();
    expect(meter.compareDocumentPosition(temp as Node) & Node.DOCUMENT_POSITION_FOLLOWING).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it('Edge: the compact amount field keeps its accessible name and shows an HP placeholder', () => {
    render(<FifthDemoSheet />);
    const card = screen.getByTestId('compose-hit-points');

    expect(within(card).getByLabelText(message('sheet.play.amount'))).toHaveAttribute(
      'placeholder',
      message('sheet.play.amountPlaceholder'),
    );
  });
  it('Happy path: demo group titles sit one level below the character heading', () => {
    render(<FifthDemoSheet />);

    expect(screen.getByRole('heading', { level: 2, name: 'Ysolde Ashgrave' })).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { level: 3, name: message('sheet.play.skills') }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { level: 2, name: message('sheet.play.skills') }),
    ).toBeNull();
  });

  it('Happy path: a roll button name carries the value it shows', () => {
    render(<FifthDemoSheet />);

    const modifier = within(wisdomCell()).getByRole('button', {
      name: rollName('Wisdom modifier'),
    });
    expect(modifier.getAttribute('aria-label')).toContain(modifier.textContent ?? '');
  });
});
