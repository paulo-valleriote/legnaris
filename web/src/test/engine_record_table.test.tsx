import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';
import type { FieldNode, ListSchema } from '../api/models/template';
import type { SheetExclusiveGroup, SheetField } from '../engine/field_grouping';
import type { RecordListActions } from '../engine/sheet_form';
import { SheetForm } from '../engine/sheet_form';
import { message, messageWith } from '../l10n/strings';
import {
  LIST_CAPACITY,
  LIST_PATH,
  LIST_RECORD_A,
  LIST_RECORD_B,
  listData,
  listFields,
} from './list_fixture';

const FIELDS = listFields();
const DATA = listData();

function listNode(fields: Record<string, FieldNode>): ListSchema {
  const inventory = fields.inventory;
  if (inventory?.kind !== 'group') {
    throw new Error('the fixture lost its inventory group');
  }
  const items = inventory.children.items;
  if (items?.kind !== 'list') {
    throw new Error('the fixture lost its items list');
  }
  return items.list;
}

function recordRows(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>('[data-row-kind="record"]'));
}

function actions(overrides: Partial<RecordListActions> = {}): RecordListActions {
  return {
    add: vi.fn(() => undefined),
    remove: vi.fn(),
    move: vi.fn(),
    ...overrides,
  };
}

const emptyData = { level: 2, inventory: { items: {} } };

const addName = () => messageWith('record.add', { list: 'Items' });

describe('the record table in the generic form', () => {
  it('Happy path: a list draws a header and one editable row per record under its own keys', () => {
    const { container } = render(
      <SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} recordActions={actions()} />,
    );

    expect(screen.getByRole('heading', { name: 'Inventory' })).toBeInTheDocument();
    expect(screen.queryByText(message('record.empty'))).not.toBeInTheDocument();
    const rows = recordRows(container);
    expect(rows.map((row) => row.dataset.rowKey)).toEqual([
      `${LIST_PATH}.${LIST_RECORD_A}`,
      `${LIST_PATH}.${LIST_RECORD_B}`,
    ]);

    const first = rows[0] as HTMLElement;
    expect(within(first).getByRole('textbox', { name: 'Name' })).toHaveValue('Sword');
    expect(within(first).getByRole('textbox', { name: 'Quantity' })).toHaveValue('2');
    expect(within(first).getByRole('checkbox', { name: 'Active' })).toBeChecked();
    expect(within(first).getByRole('combobox', { name: 'Pick' })).toHaveValue(
      `${LIST_PATH}.${LIST_RECORD_A}.optionA`,
    );
    expect(within(first).getByRole('group', { name: /^Derived total(:| |$)/ })).toHaveTextContent(
      '6',
    );
    expect(within(first).getAllByText('Name')).not.toHaveLength(0);
    expect(within(first).getAllByText('Derived total')).not.toHaveLength(0);
  });

  it('Happy path: the exclusive members of a shape take one choice cell, not one cell each', () => {
    const { container } = render(
      <SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} recordActions={actions()} />,
    );

    const first = recordRows(container)[0] as HTMLElement;
    expect(within(first).getAllByRole('combobox')).toHaveLength(1);
    const options = within(first)
      .getByRole('combobox', { name: 'Pick' })
      .querySelectorAll('option');
    expect(Array.from(options).map((option) => option.textContent)).toEqual(['None', 'A', 'B']);
  });

  it('Happy path: Add calls the seam with the list path and stays enabled below capacity', async () => {
    const user = userEvent.setup();
    const recordActions = actions({ add: vi.fn(() => LIST_RECORD_A) });
    render(
      <SheetForm
        fields={FIELDS}
        data={emptyData}
        onChanged={vi.fn()}
        recordActions={recordActions}
      />,
    );

    const add = screen.getByRole('button', { name: addName() });
    expect(add).toBeEnabled();
    await user.click(add);

    expect(recordActions.add).toHaveBeenCalledWith(LIST_PATH);
  });

  it('Edge: at capacity Add is disabled and carries the reason', () => {
    render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} recordActions={actions()} />);

    const add = screen.getByRole('button', { name: addName() });
    expect(add).toHaveAttribute('aria-disabled', 'true');
    expect(
      screen.getByText(
        messageWith('record.atCapacity', {
          count: LIST_CAPACITY,
          capacity: LIST_CAPACITY,
        }),
      ),
    ).toBeInTheDocument();
  });

  it('Edge: an empty list keeps its section, says so and offers Add', () => {
    const { container } = render(
      <SheetForm fields={FIELDS} data={emptyData} onChanged={vi.fn()} recordActions={actions()} />,
    );

    expect(recordRows(container)).toEqual([]);
    expect(screen.getByText(message('record.empty'))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: addName() })).toBeEnabled();
  });

  it('Happy path: the first record cannot move up and the last cannot move down', () => {
    const { container } = render(
      <SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} recordActions={actions()} />,
    );
    const rows = recordRows(container);

    expect(
      within(rows[0] as HTMLElement).getByRole('button', {
        name: messageWith('record.moveUp', { label: 'Items 1' }),
      }),
    ).toBeDisabled();
    expect(
      within(rows[0] as HTMLElement).getByRole('button', {
        name: messageWith('record.moveDown', { label: 'Items 1' }),
      }),
    ).toBeEnabled();
    expect(
      within(rows[1] as HTMLElement).getByRole('button', {
        name: messageWith('record.moveDown', { label: 'Items 2' }),
      }),
    ).toBeDisabled();
  });

  it('Happy path: remove and move report the record through the seam', async () => {
    const user = userEvent.setup();
    const recordActions = actions();
    const { container } = render(
      <SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} recordActions={recordActions} />,
    );
    const first = recordRows(container)[0] as HTMLElement;

    await user.click(
      within(first).getByRole('button', {
        name: messageWith('record.moveDown', { label: 'Items 1' }),
      }),
    );
    await user.click(
      within(first).getByRole('button', {
        name: messageWith('record.remove', { label: 'Items 1' }),
      }),
    );

    expect(recordActions.move).toHaveBeenCalledWith(LIST_PATH, LIST_RECORD_A, 1);
    expect(recordActions.remove).toHaveBeenCalledWith(LIST_PATH, LIST_RECORD_A);
  });

  it('Edge: without the record seam the same table is drawn read-only, with no add, move or remove', () => {
    const { container } = render(<SheetForm fields={FIELDS} data={DATA} onChanged={vi.fn()} />);

    expect(recordRows(container)).toHaveLength(2);
    expect(screen.queryByRole('button', { name: addName() })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Remove / })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Move / })).not.toBeInTheDocument();
  });

  it('Happy path: builder mode draws the cells it is given and asks for the full record paths', () => {
    const controlBuilder = vi.fn((field: SheetField): ReactNode => (
      <span data-testid={`cell-${field.path}`}>{field.schema.label}</span>
    ));
    const exclusiveControlBuilder = vi.fn((group: SheetExclusiveGroup): ReactNode => (
      <span data-testid={`choice-${group.id}`}>{group.options.length}</span>
    ));
    const { container } = render(
      <SheetForm
        fields={FIELDS}
        controlBuilder={controlBuilder}
        exclusiveControlBuilder={exclusiveControlBuilder}
        records={{ [LIST_PATH]: [LIST_RECORD_A] }}
        recordActions={actions()}
      />,
    );

    expect(recordRows(container)).toHaveLength(1);
    expect(screen.getByTestId(`cell-${LIST_PATH}.${LIST_RECORD_A}.name`)).toBeInTheDocument();
    expect(screen.getByTestId(`cell-${LIST_PATH}.${LIST_RECORD_A}.quantity`)).toBeInTheDocument();
    expect(screen.getByTestId('choice-pick')).toBeInTheDocument();
    expect(controlBuilder).toHaveBeenCalledWith(
      expect.objectContaining({ path: `${LIST_PATH}.${LIST_RECORD_A}.name` }),
      undefined,
      undefined,
    );
  });

  it('Edge: a record row is not a derived row even when every cell is calculated', () => {
    const fields = listFields();
    const list = listNode(fields);
    const shaped: FieldNode = {
      kind: 'list',
      list: {
        ...list,
        shape: { total: list.shape.derivedTotal },
        columns: ['total'],
      },
    };
    const { container } = render(
      <SheetForm
        fields={{ inventory: { kind: 'group', children: { items: shaped } } }}
        data={{ inventory: { items: { [LIST_RECORD_A]: { position: 1, total: 3 } } } }}
        onChanged={vi.fn()}
      />,
    );

    expect(recordRows(container)[0]).not.toHaveClass('lg-row--derived');
  });
});
