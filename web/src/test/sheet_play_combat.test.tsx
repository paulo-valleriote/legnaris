import { describe, expect, it } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import {
  fantasySheetData,
  fantasySheetPayload,
  FANTASY_SHEET_ID,
  fantasyTemplatePayload,
} from './fantasy_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

interface Health {
  readonly current: number;
  readonly max: number;
  readonly temp: number;
}

const FULL_HEALTH: Health = { current: 10, max: 20, temp: 0 };

function healthData(health: Health): Record<string, unknown> {
  return {
    ...fantasySheetData(),
    combat: {
      hitPoints: health.current,
      hitPointsMax: health.max,
      hitPointsTemp: health.temp,
    },
  };
}

async function openHealth(health: Health = FULL_HEALTH) {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload({ data: healthData(health) })),
    successOutcome(fantasyTemplatePayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function amountInput() {
  return screen.getByRole('spinbutton', { name: message('sheet.play.amount') });
}

async function apply(
  user: ReturnType<typeof userEvent.setup>,
  amount: string,
  action: 'damage' | 'heal',
) {
  const input = amountInput();
  await user.clear(input);
  if (amount !== '') {
    await user.type(input, amount);
  }
  await user.click(screen.getByRole('button', { name: message(`sheet.play.${action}`) }));
}

function meter() {
  return screen.getByRole('meter', { name: message('sheet.play.hitPoints') });
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

function savePending(fake: ReturnType<typeof createFakeTransport>) {
  return fake.requests.filter((request) => request.method === 'PATCH');
}

describe('the combat strip', { timeout: 15_000 }, () => {
  it('Edge: a template without a preset draws no combat strip', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome({}),
    );
    renderApp(`/sheets/${SHEET_ID}`, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    await screen.findByTestId('sheet-name');

    expect(screen.queryByTestId('sheet-combat-strip')).not.toBeInTheDocument();
  });

  it('Edge: Play draws the strip and Edit mode hides it', async () => {
    const { user } = await openHealth();

    expect(screen.getByTestId('sheet-combat-strip')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));
    expect(screen.queryByTestId('sheet-combat-strip')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.play') }));
    expect(screen.getByTestId('sheet-combat-strip')).toBeInTheDocument();
  });
});

describe('the hit points card', () => {
  it('Edge: the card reads current and maximum and exposes a meter with its bounds', async () => {
    await openHealth();

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
    expect(meter()).toHaveAttribute('role', 'meter');
    expect(meter()).toHaveAttribute('aria-valuemin', '0');
    expect(meter()).toHaveAttribute('aria-valuemax', '20');
    expect(meter()).toHaveAttribute('aria-valuenow', '10');
  });

  it('Edge: a negative or oversized stored current is drawn inside the bar', async () => {
    await openHealth({ current: -4, max: 20, temp: 0 });
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('0 / 20');
    expect(meter()).toHaveAttribute('aria-valuenow', '0');
    expect(meter()).toHaveAttribute('data-band', 'low');

    cleanup();
    await openHealth({ current: 30, max: 20, temp: 0 });
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('20 / 20');
    expect(meter()).toHaveAttribute('aria-valuenow', '20');
    expect(meter()).toHaveAttribute('data-band', 'high');
  });

  it('Edge: temporary points draw a segment and its value only when they are positive', async () => {
    await openHealth();
    expect(screen.queryByTestId('sheet-hp-temp')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-hp-temp-value')).not.toBeInTheDocument();

    cleanup();
    await openHealth({ current: 10, max: 20, temp: 5 });
    expect(screen.getByTestId('sheet-hp-temp')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-hp-temp-value')).toHaveTextContent('+5 temp');
  });

  it('Edge: the bar colour follows the health percentage', async () => {
    await openHealth({ current: 11, max: 20, temp: 0 });
    expect(meter()).toHaveAttribute('data-band', 'high');

    cleanup();
    await openHealth({ current: 10, max: 20, temp: 0 });
    expect(meter()).toHaveAttribute('data-band', 'medium');

    cleanup();
    await openHealth({ current: 4, max: 20, temp: 0 });
    expect(meter()).toHaveAttribute('data-band', 'low');
  });

  it('Edge: a zero maximum reads as no hit points, not a full bar', async () => {
    await openHealth({ current: 0, max: 0, temp: 0 });

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('0 / 0');
    expect(meter()).toHaveAttribute('aria-valuemax', '0');
    expect(meter()).toHaveAttribute('data-band', 'low');
  });

  it('Happy path: the card shows the current value against the maximum', async () => {
    await openHealth();

    expect(screen.getByTestId('sheet-hp-card')).toHaveTextContent(message('sheet.play.hitPoints'));
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
  });
});

describe('the quick hit point actions', () => {
  function quickDamage(count: number) {
    return screen.getByRole('button', { name: messageWith('sheet.play.damageAmount', { count }) });
  }

  function quickHeal(count: number) {
    return screen.getByRole('button', { name: messageWith('sheet.play.healAmount', { count }) });
  }

  function quickTemp() {
    return screen.getByRole('button', { name: message('sheet.play.addTemp') });
  }

  it('Edge: minus five spends the temporary points first', async () => {
    const { fake, user } = await openHealth({ current: 10, max: 20, temp: 3 });

    await user.click(quickDamage(5));

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('8 / 20');
    expect(screen.queryByTestId('sheet-hp-temp-value')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-hp-change')).toHaveTextContent('10 → 8 (−2)');
    expect(screen.getByTestId('sheet-hp-temp-change')).toHaveTextContent('temp 3 → 0 (−3)');
    expect(savePending(fake)).toHaveLength(0);
  });

  it('Edge: plus five at the maximum writes nothing', async () => {
    const { fake, user } = await openHealth({ current: 20, max: 20, temp: 0 });

    await user.click(quickHeal(5));

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('20 / 20');
    expect(screen.queryByTestId('sheet-hp-pending')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
    expect(savePending(fake)).toHaveLength(0);
  });

  it('Edge: the temporary action stays disabled until the amount is positive', async () => {
    const { user } = await openHealth();

    expect(quickTemp()).toBeDisabled();

    await user.type(amountInput(), '0');
    expect(quickTemp()).toBeDisabled();

    await user.type(amountInput(), '{backspace}4');
    expect(quickTemp()).toBeEnabled();
  });

  it('Edge: a temporary amount below the current temporary points keeps the higher value', async () => {
    const { user } = await openHealth({ current: 10, max: 20, temp: 5 });

    await user.type(amountInput(), '3');
    await user.click(quickTemp());

    expect(screen.getByTestId('sheet-hp-temp-value')).toHaveTextContent('+5 temp');
    expect(screen.queryByTestId('sheet-hp-temp-change')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-hp-change')).not.toBeInTheDocument();
  });

  it('Edge: a fractional amount rounds to a whole number of temporary points', async () => {
    const { user } = await openHealth({ current: 10, max: 20, temp: 0 });

    await user.type(amountInput(), '2.6');
    await user.click(quickTemp());

    expect(screen.getByTestId('sheet-hp-temp-value')).toHaveTextContent('+3 temp');
  });

  it('Happy path: the temporary action takes its value from the amount field and clears it', async () => {
    const { fake, user } = await openHealth({ current: 10, max: 20, temp: 0 });

    await user.type(amountInput(), '6');
    await user.click(quickTemp());

    expect(screen.getByTestId('sheet-hp-temp-value')).toHaveTextContent('+6 temp');
    expect(screen.getByTestId('sheet-hp-temp-change')).toHaveTextContent('temp 0 → 6 (+6)');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Temporary hit points 0 → 6',
    );
    expect(amountInput()).toHaveValue(null);
    expect(savePending(fake)).toHaveLength(0);
  });

  it('Edge: a temporary action never rewrites a current value drawn above the maximum', async () => {
    const { user } = await openHealth({ current: 30, max: 20, temp: 0 });

    await user.type(amountInput(), '4');
    await user.click(quickTemp());

    expect(screen.getByTestId('sheet-hp-temp-value')).toHaveTextContent('+4 temp');
    expect(screen.queryByTestId('sheet-hp-change')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).not.toHaveTextContent('Current hit points');
  });

  it('Happy path: repeated quick taps accumulate in the pending delta', async () => {
    const { fake, user } = await openHealth();

    await user.click(quickHeal(1));
    await user.click(quickHeal(1));
    await user.click(quickHeal(1));

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('13 / 20');
    expect(screen.getByTestId('sheet-hp-change')).toHaveTextContent('10 → 13 (+3)');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Current hit points 10 → 13',
    );
    expect(savePending(fake)).toHaveLength(0);
  });

  it('Happy path: every quick action is named by its effect', async () => {
    await openHealth();

    expect(quickDamage(5)).toHaveTextContent('\u22125');
    expect(quickDamage(1)).toHaveTextContent('\u22121');
    expect(quickHeal(1)).toHaveTextContent('+1');
    expect(quickHeal(5)).toHaveTextContent('+5');
    expect(quickTemp()).toHaveTextContent('+ Temp');
  });

  it('Happy path: a quick action leaves an untouched amount in the field', async () => {
    const { user } = await openHealth();

    await user.type(amountInput(), '4');
    await user.click(quickDamage(1));

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('9 / 20');
    expect(amountInput()).toHaveValue(4);
  });
});

describe('damage and healing', () => {
  it('Edge: an empty, zero or negative amount writes nothing', async () => {
    const { fake, user } = await openHealth();

    await apply(user, '', 'damage');
    await apply(user, '0', 'damage');
    await apply(user, '-3', 'heal');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
    expect(screen.queryByTestId('sheet-hp-pending')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.pending.save') }),
    ).not.toBeInTheDocument();
    expect(savePending(fake)).toHaveLength(0);
    expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
  });

  it('Edge: an inline current above the maximum clamps to the maximum', async () => {
    const { user } = await openHealth({ current: 10, max: 20, temp: 0 });

    await user.click(screen.getByRole('button', { name: 'Current hit points' }));
    const current = screen.getByRole('textbox', { name: 'Current hit points' });
    await user.clear(current);
    await user.type(current, '40{Enter}');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('20 / 20');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Current hit points 10 → 20',
    );
  });

  it('Edge: lowering the maximum below the current value lowers the current value too', async () => {
    const { user } = await openHealth({ current: 10, max: 10, temp: 0 });

    await user.click(screen.getByRole('button', { name: 'Hit points (maximum)' }));
    const maximum = screen.getByRole('textbox', { name: 'Hit points (maximum)' });
    await user.clear(maximum);
    await user.type(maximum, '6{Enter}');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('6 / 6');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Current hit points 10 → 6',
    );
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Hit points (maximum) 10 → 6',
    );

    await apply(user, '2', 'damage');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('4 / 6');
    expect(meter()).toHaveAttribute('aria-valuenow', '4');
  });

  it('Edge: damage beyond the current value floors it at zero', async () => {
    const { user } = await openHealth({ current: 2, max: 20, temp: 0 });

    await apply(user, '10', 'damage');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('0 / 20');
    expect(screen.getByTestId('sheet-hp-change')).toHaveTextContent('2 → 0 (−2)');
  });

  it('Edge: damage fully absorbed by temporary points keeps the current value', async () => {
    const { fake, user } = await openHealth({ current: 10, max: 20, temp: 5 });

    await apply(user, '3', 'damage');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
    expect(screen.getByTestId('sheet-hp-temp-value')).toHaveTextContent('+2 temp');
    expect(screen.getByTestId('sheet-hp-temp-pending')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-hp-change')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-hp-temp-change')).toHaveTextContent('temp 5 → 2 (−3)');
    expect(savePending(fake)).toHaveLength(0);
  });

  it('Edge: healing at the maximum writes nothing', async () => {
    const { user } = await openHealth({ current: 20, max: 20, temp: 0 });

    await apply(user, '5', 'heal');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('20 / 20');
    expect(screen.queryByTestId('sheet-hp-pending')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.pending.save') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: Enter in the amount field applies damage', async () => {
    const { user } = await openHealth();

    await user.type(amountInput(), '4{Enter}');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('6 / 20');
    expect(screen.getByTestId('sheet-hp-change')).toHaveTextContent('10 → 6 (−4)');
    expect(screen.getByTestId('sheet-hp-pending')).toHaveAttribute('data-tone', 'damage');
    expect(amountInput()).toHaveValue(null);
  });

  it('Edge: Enter with an empty or zero amount writes nothing', async () => {
    const { user } = await openHealth();

    await user.click(amountInput());
    await user.keyboard('{Enter}');
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
    expect(screen.queryByTestId('sheet-hp-pending')).not.toBeInTheDocument();

    await user.type(amountInput(), '0{Enter}');
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
    expect(screen.queryByTestId('sheet-hp-pending')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.pending.save') }),
    ).not.toBeInTheDocument();
    expect(amountInput()).toHaveValue(0);
  });

  it('Edge: a pending heal draws the success hatch', async () => {
    const { user } = await openHealth({ current: 10, max: 20, temp: 0 });

    await apply(user, '5', 'heal');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('15 / 20');
    expect(screen.getByTestId('sheet-hp-pending')).toHaveAttribute('data-tone', 'heal');
    expect(screen.getByTestId('sheet-hp-change')).toHaveTextContent('10 → 15 (+5)');
  });

  it('Edge: damage then healing back to the saved value clears the preview', async () => {
    const { user } = await openHealth();

    await apply(user, '4', 'damage');
    await apply(user, '4', 'heal');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
    expect(screen.queryByTestId('sheet-hp-pending')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-hp-change')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.pending.save') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: the amount clears once it is applied', async () => {
    const { user } = await openHealth();

    await apply(user, '4', 'damage');

    expect(amountInput()).toHaveValue(null);
  });

  it('Happy path: Damage writes the pending delta and shows the lost segment and its change', async () => {
    const { fake, user } = await openHealth();

    await apply(user, '4', 'damage');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('6 / 20');
    expect(meter()).toHaveAttribute('aria-valuenow', '6');
    expect(screen.getByTestId('sheet-hp-pending')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-hp-pending')).toHaveAttribute('data-tone', 'damage');
    expect(screen.getByTestId('sheet-hp-change')).toHaveTextContent('10 → 6 (−4)');
    expect(screen.getByTestId('sheet-pending-card')).toHaveTextContent('Current hit points 10 → 6');
    expect(savePending(fake)).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { combat: { hitPoints: 6 } } }));
  });

  it('Happy path: damage spilling past the temporary points writes both values', async () => {
    const { fake, user } = await openHealth({ current: 10, max: 20, temp: 5 });

    await apply(user, '8', 'damage');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('7 / 20');
    expect(screen.queryByTestId('sheet-hp-temp-value')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-hp-temp-pending')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-hp-temp-change')).toHaveTextContent('temp 5 → 0 (−5)');
    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
    await waitFor(() =>
      expect(savedData(fake)).toEqual({
        data: { combat: { hitPoints: 7, hitPointsTemp: 0 } },
      }),
    );
  });

  it('Happy path: Heal is capped at the maximum and writes the pending delta', async () => {
    const { fake, user } = await openHealth({ current: 19, max: 20, temp: 0 });

    await apply(user, '5', 'heal');

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('20 / 20');
    expect(screen.getByTestId('sheet-hp-change')).toHaveTextContent('19 → 20 (+1)');
    expect(screen.getByTestId('sheet-hp-pending')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-hp-pending')).toHaveAttribute('data-tone', 'heal');
    expect(savePending(fake)).toHaveLength(0);

    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { combat: { hitPoints: 20 } } }));
  });

  it('Happy path: saving advances the baseline so the lost segment and the previous value go away', async () => {
    const { user } = await openHealth();

    await apply(user, '4', 'damage');
    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));

    await waitFor(() => expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument());
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('6 / 20');
    expect(screen.queryByTestId('sheet-hp-pending')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-hp-change')).not.toBeInTheDocument();
  });

  it('Happy path: Discard restores the stored hit points', async () => {
    const { user } = await openHealth();

    await apply(user, '4', 'damage');
    const card = screen.getByTestId('sheet-pending-card');
    await user.click(within(card).getByRole('button', { name: message('sheet.pending.discard') }));

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
    expect(screen.queryByTestId('sheet-hp-pending')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-pending-card')).not.toBeInTheDocument();
  });
});
