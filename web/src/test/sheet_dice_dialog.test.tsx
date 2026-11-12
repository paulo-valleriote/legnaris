import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useEffect, useRef } from 'react';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { succeed } from '../dice/errors';
import { DicePanel, FLOATING_FLOOR_ATTRIBUTE, floorOffset } from '../sheet/dice_panel';
import { Dialog } from '../ui/dialog';
import { focusTrap } from '../ui/focus_trap';
import { renderApp } from './render';
import { arrangeTemplatePayload, createArrangeStub } from './arrange_stub';
import { SHEET_ID } from './sheet_fixtures';

const SHEET_URL = `/sheets/${SHEET_ID}`;
const BASE_URL = 'http://api.test';
const DICE_TEMPLATE = {
  ...arrangeTemplatePayload(),
  dice: { section: {}, row: { 'alpha.one': '1d20+alpha.two' } },
};
const DATA = { alpha: { one: 5, two: 3, three: 0 } };

async function open() {
  const stub = createArrangeStub({ template: DICE_TEMPLATE, data: DATA });
  const user = userEvent.setup();
  renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { user };
}

const rollButton = (label: string) =>
  screen.getByRole('button', { name: messageWith('dice.roll', { label }) });
const editButton = (label: string) =>
  screen.getByRole('button', { name: messageWith('dice.edit', { label }) });
const panels = () => document.querySelectorAll('.lg-dice-panel');

afterEach(cleanup);

describe('floorOffset', () => {
  const tests: { name: string; viewport: number; top: number | undefined; want: number }[] = [
    { name: 'Edge: no floor element lifts nothing', viewport: 800, top: undefined, want: 0 },
    { name: 'Edge: a non-finite top lifts nothing', viewport: 800, top: Number.NaN, want: 0 },
    { name: 'Edge: a floor below the viewport lifts nothing', viewport: 800, top: 900, want: 0 },
    {
      name: 'Edge: a floor exactly at the viewport end lifts nothing',
      viewport: 800,
      top: 800,
      want: 0,
    },
    {
      name: 'Edge: a floor above the viewport is clamped to it',
      viewport: 800,
      top: -50,
      want: 800,
    },
    {
      name: 'Happy path: a sticky footer lifts by its visible height',
      viewport: 800,
      top: 712,
      want: 88,
    },
  ];
  for (const tc of tests) {
    it(tc.name, () => {
      expect(floorOffset(tc.viewport, tc.top)).toBe(tc.want);
    });
  }
});

describe('DicePanel floats above the floor', () => {
  it('Edge: with no floor element the panel sits on the viewport edge', () => {
    render(
      <DicePanel label="One" outcome={succeed({ total: 4, parts: [] })} onDismiss={() => {}} />,
    );

    const panel = document.querySelector<HTMLElement>('.lg-dice-panel');
    expect(panel?.style.getPropertyValue('--dice-floor')).toBe('0px');
  });

  it('Happy path: the panel is lifted by the floor element it must stay above', () => {
    const floor = document.createElement('footer');
    floor.setAttribute(FLOATING_FLOOR_ATTRIBUTE, '');
    floor.getBoundingClientRect = () => ({ top: window.innerHeight - 90 }) as DOMRect;
    document.body.append(floor);

    render(
      <DicePanel label="One" outcome={succeed({ total: 4, parts: [] })} onDismiss={() => {}} />,
    );

    const panel = document.querySelector<HTMLElement>('.lg-dice-panel');
    expect(panel?.style.getPropertyValue('--dice-floor')).toBe('90px');
    floor.remove();
  });
});

describe('focusTrap', () => {
  function mount() {
    const container = document.createElement('div');
    const [a, b, disabled, c] = ['a', 'b', 'disabled', 'c'].map((name) => {
      const button = document.createElement('button');
      button.textContent = name;
      container.append(button);
      return button;
    });
    disabled.setAttribute('disabled', '');
    document.body.append(container);
    return { container, a, b, c, trap: focusTrap(container) };
  }

  it('Edge: Tab between the ends keeps the browser order and is not prevented', () => {
    const { container, a, trap } = mount();
    a.focus();

    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
    a.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    trap.release();
    container.remove();
  });

  it('Edge: Tab on the last enabled control wraps to the first, skipping a disabled one', () => {
    const { container, a, c, trap } = mount();
    c.focus();

    fireEvent.keyDown(c, { key: 'Tab' });

    expect(document.activeElement).toBe(a);
    trap.release();
    container.remove();
  });

  it('Edge: Shift+Tab on the first wraps to the last', () => {
    const { container, a, c, trap } = mount();
    a.focus();

    fireEvent.keyDown(a, { key: 'Tab', shiftKey: true });

    expect(document.activeElement).toBe(c);
    trap.release();
    container.remove();
  });

  it('Edge: an empty container never throws and prevents nothing', () => {
    const container = document.createElement('div');
    document.body.append(container);
    const trap = focusTrap(container);

    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
    container.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    trap.release();
    container.remove();
  });
});

describe('Dialog focus', () => {
  function SelfFocusing() {
    const input = useRef<HTMLInputElement>(null);
    useEffect(() => input.current?.focus(), []);
    return <input ref={input} aria-label="inner" />;
  }

  it('Edge: a child that focuses itself on mount keeps the focus', () => {
    render(
      <Dialog open title="T" onClose={() => {}}>
        <SelfFocusing />
      </Dialog>,
    );

    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'inner' }));
  });

  it('Happy path: with no self-focusing child the panel takes the focus', () => {
    render(<Dialog open title="T" onClose={() => {}} />);

    expect(document.activeElement).toBe(screen.getByRole('dialog', { name: 'T' }));
  });
});

describe('sheet dice: editor dialog and Roll panel', () => {
  it('Edge: opening the editor dismisses an open Roll panel', async () => {
    const { user } = await open();
    await user.click(rollButton('One'));
    expect(panels().length).toBe(1);

    await user.click(editButton('One'));

    expect(panels().length).toBe(0);
    expect(
      screen.getByRole('dialog', { name: messageWith('dice.edit', { label: 'One' }) }),
    ).toBeTruthy();
  });

  it('Edge: rolling the same Row twice draws a fresh panel, so it is announced again', async () => {
    const { user } = await open();
    await user.click(rollButton('One'));
    const first = panels()[0];

    await user.click(rollButton('One'));

    expect(panels().length).toBe(1);
    expect(panels()[0]).not.toBe(first);
  });

  it('Edge: Keep current dice closes the dialog without a request and returns focus', async () => {
    const { user } = await open();
    await user.click(editButton('One'));
    const dialog = screen.getByRole('dialog', { name: messageWith('dice.edit', { label: 'One' }) });

    await user.click(within(dialog).getByRole('button', { name: message('dice.cancel') }));

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(document.activeElement).toBe(editButton('One'));
  });

  it('Happy path: the editor is a modal dialog with the expression field focused', async () => {
    const { user } = await open();

    await user.click(editButton('One'));

    const dialog = screen.getByRole('dialog', { name: messageWith('dice.edit', { label: 'One' }) });
    expect(dialog.getAttribute('aria-modal')).toBe('true');
    const field = within(dialog).getByRole('textbox', { name: message('dice.expressionLabel') });
    await waitFor(() => expect(document.activeElement).toBe(field));
    expect((field as HTMLInputElement).value).toBe('1d20+alpha.two');
    expect(editButton('One').getAttribute('aria-haspopup')).toBe('dialog');
  });

  it('Happy path: the Section Dice button shares the title row and the editor never sits in a Row', async () => {
    const { user } = await open();
    const section = document.querySelector<HTMLElement>('[data-section-key="alpha"]');
    const head = section?.querySelector('.lg-dice-section');

    expect(head?.querySelector('.lg-section__title')).not.toBeNull();
    expect(
      within(head as HTMLElement).getByRole('button', {
        name: messageWith('dice.editSection', { label: 'Alpha' }),
      }),
    ).toBeTruthy();

    await user.click(editButton('One'));
    expect(document.querySelector('[data-row-key] .lg-dice-editor')).toBeNull();
  });

  it('Happy path: Roll and Edit sit at the trailing edge of the Row label', async () => {
    await open();

    const actions = rollButton('One').closest('.lg-dice-actions');
    expect(actions?.parentElement?.classList.contains('lg-field-row__label')).toBe(true);
    expect(actions?.contains(editButton('One'))).toBe(true);
  });
});
