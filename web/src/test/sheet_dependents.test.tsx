import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { parseTemplateDetail } from '../api/models/template';
import { DependentReadout } from '../engine/dependent_readout';
import { SheetForm } from '../engine/sheet_form';
import { message } from '../l10n/strings';
import { revealRow } from '../sheet/reveal_row';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const SHEET_URL = `/sheets/${SHEET_ID}`;

function number(label: string, extra: Record<string, unknown> = {}) {
  return { schema: { type: 'number', label, is_editable: true, default: 0, ...extra } };
}

function calc(label: string, on: string[][], extra: Record<string, unknown> = {}) {
  return number(label, { is_editable: false, formula: 'f', depends_on: on, ...extra });
}

const FIELDS = {
  core: {
    label: 'Core',
    children: {
      strength: number('Strength'),
      strMod: calc('Strength modifier', [['core', 'strength']]),
      level: number('Level'),
      lvA: calc('Level A', [['core', 'level']]),
      lvB: calc('Level B', [['core', 'level']]),
      lvC: calc('Level C', [['core', 'level']]),
      baseAc: number('Base armor'),
      ac: calc('Armor class', [['core', 'baseAc']]),
      acNote: calc('Armor note', [['core', 'ac']]),
      useX: number('Use X'),
      flag: calc('Flag result', [['config', 'useX']]),
    },
  },
  skills: {
    label: 'Skills',
    children: {
      athletics: {
        label: 'Athletics',
        columns: ['bonus', 'total'],
        children: {
          bonus: number('Athletics bonus', { column_label: 'Bonus' }),
          total: calc('Athletics total', [['skills', 'athletics', 'bonus']], {
            column_label: 'Total',
          }),
        },
      },
    },
  },
};

const DATA = {
  core: {
    strength: 14,
    strMod: 2,
    level: 3,
    lvA: 1,
    lvB: 2,
    lvC: 3,
    baseAc: 10,
    ac: 10,
    acNote: 0,
  },
  skills: { athletics: { bonus: 1, total: 3 } },
};

function templateWith() {
  return templateDetailPayload({ fields: FIELDS });
}

async function open(...extra: ReturnType<typeof successOutcome>[]) {
  const fake = createFakeTransport(
    successOutcome(sheetPayload({ data: DATA })),
    successOutcome(templateWith()),
    ...extra,
  );
  const user = userEvent.setup();
  renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

const rowOf = (key: string) => document.querySelector<HTMLElement>(`[data-row-key="${key}"]`);
const besideOf = (key: string) => rowOf(key)?.querySelector<HTMLElement>('.lg-dependents') ?? null;

describe('derived values beside their input on the sheet screen', () => {
  it('Happy path: Strength shows its modifier beside it and base armor shows armor class', async () => {
    await open();

    expect(besideOf('core.strength')).toHaveTextContent('Strength modifier 2');
    expect(besideOf('core.baseAc')).toHaveTextContent('Armor class 10');
  });

  it('Edge: an input feeding three derived leaves, or none, shows nothing', async () => {
    await open();

    expect(besideOf('core.level')).toBeNull();
    expect(besideOf('core.useX')).toBeNull();
    expect(rowOf('core.level')).not.toHaveClass('lg-field-row--dependents');
  });

  it('Edge: a derived-of-derived value is not beside the grand-input', async () => {
    await open();

    expect(besideOf('core.baseAc')).not.toHaveTextContent('Armor note');
  });

  it('Edge: a table cell never draws a readout beside its input', async () => {
    await open();

    expect(rowOf('skills.athletics')?.querySelector('.lg-dependents')).toBeNull();
  });

  it('Edge: the derived value keeps its own Row, reachable by key', async () => {
    await open();

    expect(rowOf('core.strMod')).toBeInTheDocument();
    expect(rowOf('core.strMod')?.querySelector('.lg-field__value')).toHaveTextContent('2');
    expect(rowOf('core.strMod')?.querySelector('.lg-dependents')).toBeNull();
  });

  it('Edge: the readout carries no data-row-key of its own, so a key still names one Row', async () => {
    await open();

    expect(document.querySelectorAll('[data-row-key="core.strMod"]')).toHaveLength(1);
    expect(besideOf('core.strength')?.closest('[data-row-key]')).toBe(rowOf('core.strength'));
  });

  it('Happy path: the input is described by the readout (label, value) with no stale state at rest', async () => {
    await open();

    const input = screen.getByRole('textbox', { name: 'Strength' });

    expect(input).toHaveAccessibleDescription('Strength modifier 2');
    expect(besideOf('core.strength')?.id).toBe(input.getAttribute('aria-describedby'));
  });

  it('Happy path: an edit dims the readout and names it stale; Preview settles it with the new value', async () => {
    const { user } = await open(successOutcome({ core: { strMod: 4 } }));
    const input = screen.getByRole('textbox', { name: 'Strength' });

    await user.clear(input);
    await user.type(input, '18');

    const readout = besideOf('core.strength')?.querySelector('.lg-dependent');
    expect(readout).toHaveAttribute('data-stale', 'true');
    expect(readout).toHaveTextContent('2');
    expect(input).toHaveAccessibleDescription(
      `Strength modifier 2 ${message('engine.dependentStale')}`,
    );

    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));

    await waitFor(() =>
      expect(besideOf('core.strength')?.querySelector('.lg-dependent')).not.toHaveAttribute(
        'data-stale',
      ),
    );
    expect(besideOf('core.strength')).toHaveTextContent('Strength modifier 4');
    expect(input).toHaveAccessibleDescription('Strength modifier 4');
  });

  it('Edge: an edit after a Preview makes the readout stale again', async () => {
    const { user } = await open(successOutcome({ core: { strMod: 4 } }));
    const input = screen.getByRole('textbox', { name: 'Strength' });

    await user.clear(input);
    await user.type(input, '18');
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));
    await waitFor(() => expect(screen.getByText('Recalculated')).toBeInTheDocument());
    await user.type(input, '1');

    expect(besideOf('core.strength')?.querySelector('.lg-dependent')).toHaveAttribute(
      'data-stale',
      'true',
    );
  });

  it('Edge: editing an unrelated input leaves another input readout settled', async () => {
    const { user } = await open();

    const level = screen.getByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '9');

    expect(besideOf('core.strength')?.querySelector('.lg-dependent')).not.toHaveAttribute(
      'data-stale',
    );
  });

  it('Edge: the readout is a description, never a live region (no duplicate announcement)', async () => {
    await open();

    expect(
      besideOf('core.strength')?.closest('[aria-live], [role="status"], [role="alert"]'),
    ).toBeNull();
    expect(besideOf('core.strength')?.querySelector('[aria-live], output')).toBeNull();
    expect(screen.getAllByRole('status')).toHaveLength(1);
  });

  it('Edge: the stepper and sign targets stay full controls with a readout present', async () => {
    const { user } = await open();
    const input = screen.getByRole('textbox', { name: 'Strength' });

    await user.click(input);

    expect(screen.getByRole('button', { name: 'Increase Strength' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Decrease Strength' })).toBeInTheDocument();
  });

  it('Edge: revealRow still finds the derived Row by its key', async () => {
    await open();
    const found = revealRow('core.strMod');

    expect(found).toBe(true);
  });
});

describe('SheetForm data mode', () => {
  it('Happy path: draws the stored dependent values beside the input, never stale', () => {
    const { fields } = parseTemplateDetail(templateWith());

    render(<SheetForm fields={fields} data={DATA} onChanged={() => {}} />);

    const beside = document.querySelector('[data-row-key="core.strength"] .lg-dependents');
    expect(beside).toHaveTextContent('Strength modifier 2');
    expect(beside?.querySelector('[data-stale]')).toBeNull();
  });

  it('Edge: a missing stored value reads as not set instead of an empty readout', () => {
    const { fields } = parseTemplateDetail(templateWith());

    render(<SheetForm fields={fields} data={{ core: { strength: 14 } }} onChanged={() => {}} />);

    expect(
      document.querySelector('[data-row-key="core.strength"] .lg-dependents'),
    ).toHaveTextContent(`Strength modifier ${message('engine.notSet')}`);
  });
});

describe('DependentReadout', () => {
  it.each([
    { name: 'Happy path: a settled value', stale: false, state: false },
    { name: 'Edge: a stale value carries the state text', stale: true, state: true },
  ])('$name', ({ stale, state }) => {
    render(<DependentReadout label="Modifier" value={0} stale={stale} />);

    expect(screen.getByText('0')).toBeInTheDocument();
    expect(screen.queryByText(message('engine.dependentStale')) !== null).toBe(state);
  });

  it('Edge: markup in a label stays literal text', () => {
    render(<DependentReadout label="<b>x</b>" value="<i>1</i>" stale={false} />);

    expect(screen.getByText('<b>x</b>')).toBeInTheDocument();
    expect(document.querySelector('b, i')).toBeNull();
  });
});

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
});

beforeEach(() => {
  Element.prototype.scrollIntoView = vi.fn();
});
