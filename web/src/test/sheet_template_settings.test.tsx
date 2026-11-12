import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import {
  createViewStore,
  initialView,
  mainSectionKeys,
  parseViewRecord,
  TEMPLATE_SETTINGS_KEYS,
  viewStorageKey,
} from '../sheet/view_store';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, fieldTree, sheetPayload, templateDetailPayload } from './sheet_fixtures';
import { createMemoryViewStorage } from './view_storage';

afterEach(cleanup);

const KEYS = ['identity', 'stats', 'rules', 'config'];

function settingsTree(): Record<string, unknown> {
  return {
    ...fieldTree(),
    rules: {
      label: 'Rules',
      children: {
        modifierStep: {
          schema: { type: 'number', label: 'Ability modifier step', is_editable: true, default: 2 },
        },
      },
    },
    config: {
      label: 'Config',
      children: {
        proficient: {
          schema: { type: 'number', label: 'Proficient', is_editable: true, default: 0 },
        },
      },
    },
  };
}

async function open(options: { fields?: Record<string, unknown>; storage?: object } = {}) {
  const fake = createFakeTransport(
    successOutcome(sheetPayload()),
    successOutcome(templateDetailPayload({ fields: options.fields ?? settingsTree() })),
  );
  const viewStorage = (options.storage ?? createMemoryViewStorage()) as ReturnType<
    typeof createMemoryViewStorage
  >;
  renderApp(`/sheets/${SHEET_ID}`, {
    client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
    viewStorage,
  });
  await screen.findByTestId('sheet-name');
  return { user: userEvent.setup(), viewStorage };
}

const toggle = () => screen.getByRole('button', { name: /Template settings/ });

describe('view store: template settings', () => {
  it('Edge: the settings keys are not part of the main keys', () => {
    expect(TEMPLATE_SETTINGS_KEYS).toEqual(['rules', 'config']);
    expect(mainSectionKeys(KEYS)).toEqual(['identity', 'stats']);
    expect(mainSectionKeys([])).toEqual([]);
  });

  it('Edge: nothing remembered starts with the area closed and no Section collapsed', () => {
    expect(initialView().settingsOpen).toBe(false);
    expect(initialView().collapsed).toEqual([]);
  });

  it.each([
    ['absent', { v: 1 }, false],
    ['not a boolean', { v: 1, settingsOpen: 'yes' }, false],
    ['true', { v: 1, settingsOpen: true }, true],
  ])('Edge: a stored settingsOpen that is %s reads as %s', (_n, record, expected) => {
    const state = parseViewRecord(JSON.stringify(record), new Set(['identity']));

    expect(state.settingsOpen).toBe(expected);
  });

  it('Edge: a settings Section is not collapsible and not remembered as collapsed', () => {
    const store = createViewStore({ sheetId: 's', sectionKeys: KEYS, storage: undefined });

    store.toggle('rules');
    store.collapseAll();

    expect(store.getState().collapsed).toEqual(['identity', 'stats']);
    expect(store.getState().settingsOpen).toBe(false);
  });

  it('Edge: revealing a settings Section opens the area and turns the filter off', () => {
    const store = createViewStore({ sheetId: 's', sectionKeys: KEYS, storage: undefined });
    store.setOnlyPinned(true);

    store.reveal('config');

    expect(store.getState().settingsOpen).toBe(true);
    expect(store.getState().onlyPinned).toBe(false);
  });

  it('Edge: a store whose storage throws still toggles', () => {
    const storage = createMemoryViewStorage({}, { throws: true });
    const store = createViewStore({ sheetId: 's', sectionKeys: KEYS, storage });

    store.toggleSettings();

    expect(store.getState().settingsOpen).toBe(true);
  });

  it('Happy path: toggling twice returns to closed', () => {
    const store = createViewStore({ sheetId: 's', sectionKeys: KEYS, storage: undefined });

    store.toggleSettings();
    store.toggleSettings();

    expect(store.getState().settingsOpen).toBe(false);
  });
});

describe('the Template settings area', () => {
  it('Edge: a template with no settings Sections draws no area', async () => {
    await open({ fields: fieldTree() });

    expect(screen.queryByTestId('template-settings')).toBeNull();
  });

  it('Edge: it is collapsed by default with the right aria', async () => {
    await open();

    const button = toggle();

    expect(button).toHaveAttribute('aria-expanded', 'false');
    const body = document.getElementById(button.getAttribute('aria-controls') ?? '');
    expect(body).not.toBeNull();
    expect(body).toHaveAttribute('hidden');
  });

  it('Edge: the settings Sections are not tabs of the section strip', async () => {
    await open();

    const nav = screen.getByRole('navigation', { name: message('sheet.sections') });
    const labels = within(nav)
      .getAllByRole('button')
      .map((link) => link.textContent);

    expect(labels).toEqual(['Identity', 'Derived', 'Ability']);
  });

  it('Edge: a sheet with only settings Sections draws the area and no strip links', async () => {
    const tree = settingsTree();
    const fields = { rules: tree.rules, config: tree.config };
    await open({ fields });

    expect(toggle()).toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: message('sheet.sections') })).toBeNull();
  });

  it('Happy path: it opens, shows the Rows, closes again and remembers', async () => {
    const { user, viewStorage } = await open();

    await user.click(toggle());

    expect(toggle()).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('textbox', { name: 'Ability modifier step' })).toBeVisible();
    expect(viewStorage.records.get(viewStorageKey(SHEET_ID))).toContain('"settingsOpen":true');

    await user.click(toggle());

    expect(toggle()).toHaveAttribute('aria-expanded', 'false');
    expect(viewStorage.records.get(viewStorageKey(SHEET_ID))).toContain('"settingsOpen":false');
  });

  it('Edge: a remembered open area starts open', async () => {
    const storage = createMemoryViewStorage({
      [viewStorageKey(SHEET_ID)]: JSON.stringify({ v: 1, collapsed: [], settingsOpen: true }),
    });
    await open({ storage });

    expect(toggle()).toHaveAttribute('aria-expanded', 'true');
  });

  it('Edge: storage that throws does not break the sheet and the area still toggles', async () => {
    const storage = createMemoryViewStorage({}, { throws: true });
    const { user } = await open({ storage });

    expect(toggle()).toHaveAttribute('aria-expanded', 'false');
    await user.click(toggle());

    expect(toggle()).toHaveAttribute('aria-expanded', 'true');
  });

  it('Happy path: a value inside is editable and counts as a change', async () => {
    const { user } = await open();
    await user.click(toggle());

    const step = screen.getByRole('textbox', { name: 'Ability modifier step' });
    await user.clear(step);
    await user.type(step, '3');

    expect(step).toHaveValue('3');
    expect(screen.getByTestId('sheet-change-count')).toBeInTheDocument();
  });

  it('Edge: Arrange mode hides the area and leaves the other Sections arrangeable', async () => {
    const { user } = await open();
    await user.click(screen.getByRole('button', { name: message('arrange.start') }));

    expect(screen.queryByTestId('template-settings')).toBeNull();
    expect(screen.getByRole('heading', { name: 'Identity' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Rules' })).toBeNull();

    await user.click(screen.getByRole('button', { name: message('arrange.cancel') }));

    expect(toggle()).toBeInTheDocument();
  });
});
