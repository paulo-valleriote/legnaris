import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { viewStorageKey } from '../sheet/view_store';
import { createArrangeStub, arrangeTemplatePayload } from './arrange_stub';
import type { ArrangeStubOptions } from './arrange_stub';
import { renderApp } from './render';
import { SHEET_ID } from './sheet_fixtures';
import { createMemoryViewStorage } from './view_storage';

const BASE_URL = 'http://api.test';
const SHEET_URL = `/sheets/${SHEET_ID}`;

function withCalculated() {
  const base = arrangeTemplatePayload();
  return {
    ...base,
    fields: {
      ...base.fields,
      gamma: {
        label: 'Gamma',
        children: {
          solo: { schema: { type: 'number', label: 'Solo', is_editable: true, default: 0 } },
          soloMod: {
            schema: {
              type: 'number',
              label: 'Solo modifier',
              is_editable: false,
              default: 0,
              formula: 'f',
              depends_on: [['gamma', 'solo']],
            },
          },
        },
      },
    },
  };
}

async function open(
  options: ArrangeStubOptions = {},
  extra: { viewStorage?: ReturnType<typeof createMemoryViewStorage> } = {},
) {
  const stub = createArrangeStub(options);
  const user = userEvent.setup();
  const view = renderApp(SHEET_URL, {
    client: createApiClient({
      baseUrl: BASE_URL,
      transport: stub.transport,
    }),
    viewStorage: extra.viewStorage,
  });
  await screen.findByTestId('sheet-name');
  return { stub, user, ...view };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('the sheet help', () => {
  it('Edge: a Template with no calculated value has no help control and no panel', async () => {
    await open();

    expect(screen.queryByRole('button', { name: message('sheet.help') })).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-help')).not.toBeInTheDocument();
  });

  it('Happy path: it opens once by itself, names both things, and Got it closes it for good', async () => {
    const storage = createMemoryViewStorage();
    const { user, unmount } = await open({ template: withCalculated() }, { viewStorage: storage });
    const toggle = screen.getByRole('button', { name: message('sheet.help') });

    const panel = screen.getByTestId('sheet-help');
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(toggle).toHaveAttribute('aria-controls', panel.id);
    expect(panel).toHaveTextContent(message('sheet.helpDerived'));
    expect(panel).toHaveTextContent(message('sheet.helpPreview'));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    await user.click(within(panel).getByRole('button', { name: message('sheet.helpDismiss') }));

    expect(screen.queryByTestId('sheet-help')).not.toBeInTheDocument();
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(toggle).toHaveFocus();
    expect(JSON.parse(storage.records.get(viewStorageKey(SHEET_ID)) ?? '{}').explainerSeen).toBe(
      true,
    );
    unmount();
    await open({ template: withCalculated() }, { viewStorage: storage });
    expect(screen.queryByTestId('sheet-help')).not.toBeInTheDocument();
  });

  it('Edge: after dismissal the control reopens it on demand and closing it again keeps it seen', async () => {
    const storage = createMemoryViewStorage({
      [viewStorageKey(SHEET_ID)]: JSON.stringify({ v: 1, collapsed: [], explainerSeen: true }),
    });
    const { user } = await open({ template: withCalculated() }, { viewStorage: storage });
    const toggle = screen.getByRole('button', { name: message('sheet.help') });
    expect(screen.queryByTestId('sheet-help')).not.toBeInTheDocument();

    await user.click(toggle);
    expect(screen.getByTestId('sheet-help')).toBeInTheDocument();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');

    await user.click(toggle);
    expect(screen.queryByTestId('sheet-help')).not.toBeInTheDocument();
  });

  it('Edge: it never blocks, the sheet stays editable while it is open', async () => {
    const { user } = await open({ template: withCalculated() });

    expect(screen.getByTestId('sheet-help')).toBeInTheDocument();
    await user.clear(screen.getByRole('textbox', { name: 'One' }));
    await user.type(screen.getByRole('textbox', { name: 'One' }), '4');

    expect(screen.getByRole('textbox', { name: 'One' })).toHaveValue('4');
  });
});
