import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { leaveIsGuarded } from '../shell/leave_guard';
import { viewStorageKey } from '../sheet/view_store';
import type { MemoryViewStorage } from './view_storage';
import { createMemoryViewStorage } from './view_storage';
import { createArrangeStub } from './arrange_stub';
import type { ArrangeStubOptions } from './arrange_stub';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const SHEET_URL = `/sheets/${SHEET_ID}`;
const BASE_URL = 'http://api.test';

const PINS = { section_order: [], pinned: ['alpha.two', 'beta.note', 'alpha.one'] };

async function open(
  options: ArrangeStubOptions = {},
  extra: { viewStorage?: MemoryViewStorage } = {},
) {
  const stub = createArrangeStub(options);
  const user = userEvent.setup();
  const viewStorage = extra.viewStorage ?? createMemoryViewStorage();
  const view = renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
    viewStorage,
  });
  await screen.findByTestId('sheet-name');
  return { stub, user, ...view };
}

const button = (name: string) => screen.getByRole('button', { name });
const toggle = (label: string | RegExp) => {
  const found = screen
    .getAllByRole('button', { name: label })
    .find((candidate) => candidate.classList.contains('lg-section__toggle'));
  if (found === undefined) {
    throw new Error(`no Section title control named ${String(label)}`);
  }
  return found;
};
const filter = () => screen.getByRole('button', { name: message('sheet.pinnedOnly') });
const sectionKeys = () =>
  Array.from(document.querySelectorAll<HTMLElement>('[data-section-key]')).map(
    (element) => element.dataset.sectionKey,
  );
const rowKeys = (scope: ParentNode = document) =>
  Array.from(scope.querySelectorAll<HTMLElement>('[data-row-key]')).map(
    (element) => element.dataset.rowKey,
  );
const chipLabels = () =>
  within(screen.getByRole('navigation', { name: message('sheet.sections') }))
    .getAllByRole('button')
    .map((button) => button.textContent);

let scrollIntoView: ReturnType<typeof vi.fn>;

beforeEach(() => {
  scrollIntoView = vi.fn();
  Element.prototype.scrollIntoView =
    scrollIntoView as unknown as typeof Element.prototype.scrollIntoView;
});

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
});

describe('the strip is drawn only when it has something to say', () => {
  it('Edge: a template with no Section draws no strip', async () => {
    await open({ template: { id: 'fantasy_v1', name: 'Empty', default: true, fields: {} } });

    expect(screen.queryByTestId('section-strip')).not.toBeInTheDocument();
  });

  it('Edge: one Section draws the filter and no links', async () => {
    await open({
      template: {
        id: 'fantasy_v1',
        name: 'One',
        default: true,
        fields: {
          alpha: {
            label: 'Alpha',
            children: {
              one: { schema: { type: 'number', label: 'One', is_editable: true, default: 0 } },
            },
          },
        },
      },
    });

    expect(screen.getByTestId('section-strip')).toBeInTheDocument();
    expect(filter()).toBeInTheDocument();
    expect(
      screen.queryByRole('navigation', { name: message('sheet.sections') }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.collapseAll') }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: several Sections draw the filter and one link per Section in drawn order', async () => {
    await open({ layout: { section_order: ['gamma'], pinned: [] } });

    expect(chipLabels()).toEqual(['Gamma', 'Alpha', 'Beta']);
    expect(filter()).toHaveAttribute('aria-pressed', 'false');
  });

  it('Edge: the strip stays during Arrange mode and follows the staged order', async () => {
    const { user } = await open();

    await user.click(screen.getByRole('button', { name: message('arrange.start') }));

    expect(screen.getByTestId('section-strip')).toBeInTheDocument();
    expect(chipLabels()).toEqual(['Alpha', 'Beta', 'Gamma']);
    expect(filter()).toBeDisabled();
  });

  it('Edge: a jump while arranging scrolls and focuses the handle and leaves the view state alone', async () => {
    const { user, viewStorage } = await open();
    await user.click(toggle('Beta'));
    await user.click(screen.getByRole('button', { name: message('arrange.start') }));
    const before = viewStorage.records.get(viewStorageKey(SHEET_ID));

    await user.click(
      within(screen.getByRole('navigation', { name: message('sheet.sections') })).getByRole(
        'button',
        {
          name: messageWith('sheet.sectionLink', { label: 'Gamma' }),
        },
      ),
    );

    expect(scrollIntoView).toHaveBeenCalled();
    expect(
      screen.getByRole('button', { name: messageWith('arrange.handle', { label: 'Gamma' }) }),
    ).toHaveFocus();
    expect(viewStorage.records.get(viewStorageKey(SHEET_ID))).toBe(before);
  });
});

describe('collapsing Sections', () => {
  it('Happy path: collapse hides the rows without unmounting them and persists across a remount', async () => {
    const storage = createMemoryViewStorage();
    const first = await open({}, { viewStorage: storage });

    await first.user.click(toggle('Alpha'));

    expect(toggle('Alpha')).toHaveAttribute('aria-expanded', 'false');
    expect(document.querySelector('[data-section-key="alpha"] .lg-section__rows')).toHaveAttribute(
      'hidden',
    );
    expect(rowKeys()).toContain('alpha.one');
    expect(JSON.parse(storage.records.get(viewStorageKey(SHEET_ID)) ?? 'null')).toEqual({
      v: 1,
      collapsed: ['alpha'],
      onlyPinned: false,
      explainerSeen: false,
    });
    first.unmount();

    await open({}, { viewStorage: storage });

    expect(toggle('Alpha')).toHaveAttribute('aria-expanded', 'false');
    expect(toggle('Beta')).toHaveAttribute('aria-expanded', 'true');
  });

  it('Edge: collapse and filter changes send no request, never count as unsaved work and never guard a leave', async () => {
    const { stub, user } = await open();
    const before = stub.requests.length;

    await user.click(toggle('Alpha'));
    await user.click(button(message('sheet.collapseAll')));
    await user.click(button(message('sheet.expandAll')));
    await user.click(filter());
    await user.click(filter());

    expect(stub.requests).toHaveLength(before);
    expect(screen.queryByTestId('sheet-status')).not.toBeInTheDocument();
    expect(leaveIsGuarded()).toBe(false);
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
  });

  it('Edge: a stored key the Template no longer has is ignored', async () => {
    const storage = createMemoryViewStorage({
      [viewStorageKey(SHEET_ID)]: JSON.stringify({ v: 1, collapsed: ['ghost', 'beta', 'beta'] }),
    });

    await open({}, { viewStorage: storage });

    expect(toggle('Beta')).toHaveAttribute('aria-expanded', 'false');
    expect(toggle('Alpha')).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getAllByRole('button', { expanded: false }).length).toBeGreaterThanOrEqual(1);
  });

  it('Edge: corrupt stored JSON falls back to everything expanded', async () => {
    await open(
      {},
      { viewStorage: createMemoryViewStorage({ [viewStorageKey(SHEET_ID)]: '{nope' }) },
    );

    for (const label of ['Alpha', 'Beta', 'Gamma']) {
      expect(toggle(label)).toHaveAttribute('aria-expanded', 'true');
    }
  });

  it('Edge: a storage that always throws still lets the Sections collapse in memory', async () => {
    const { user } = await open({}, { viewStorage: createMemoryViewStorage({}, { throws: true }) });

    await user.click(toggle('Gamma'));

    expect(toggle('Gamma')).toHaveAttribute('aria-expanded', 'false');
  });

  it('Edge: another sheet remembers its own collapsed Sections', async () => {
    const storage = createMemoryViewStorage({
      [viewStorageKey('another-sheet')]: JSON.stringify({ v: 1, collapsed: ['alpha'] }),
    });

    await open({}, { viewStorage: storage });

    expect(toggle('Alpha')).toHaveAttribute('aria-expanded', 'true');
  });

  it('Edge: Collapse all collapses every Section and the same button then reads Expand all', async () => {
    const { user } = await open();

    await user.click(button(message('sheet.collapseAll')));

    for (const label of ['Alpha', 'Beta', 'Gamma']) {
      expect(toggle(label)).toHaveAttribute('aria-expanded', 'false');
    }
    expect(
      screen.queryByRole('button', { name: message('sheet.collapseAll') }),
    ).not.toBeInTheDocument();

    await user.click(button(message('sheet.expandAll')));

    for (const label of ['Alpha', 'Beta', 'Gamma']) {
      expect(toggle(label)).toHaveAttribute('aria-expanded', 'true');
    }
  });

  it('Edge: collapse is unavailable while arranging and restored on leaving Arrange mode', async () => {
    const { user } = await open();
    await user.click(toggle('Beta'));

    await user.click(screen.getByRole('button', { name: message('arrange.start') }));

    expect(screen.queryByRole('button', { name: 'Beta', expanded: false })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('sheet.collapseAll') }),
    ).not.toBeInTheDocument();
    expect(document.querySelector('.lg-section__rows[hidden]')).toBeNull();

    await user.click(screen.getByRole('button', { name: message('arrange.cancel') }));

    expect(toggle('Beta')).toHaveAttribute('aria-expanded', 'false');
  });

  it('Edge: a typed value survives a collapse and an expand and still saves as one edit', async () => {
    const { user } = await open();
    const field = screen.getByRole('textbox', { name: 'One' });
    await user.clear(field);
    await user.type(field, '12');

    await user.click(toggle('Alpha'));
    await user.click(toggle('Alpha'));

    expect(screen.getByRole('textbox', { name: 'One' })).toBe(field);
    expect(field).toHaveValue('12');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();
  });
});

describe('jumping to a Section', () => {
  it('Edge: a Section chip and its title toggle never share an accessible name', async () => {
    await open();

    const chipName = messageWith('sheet.sectionLink', { label: 'Gamma' });

    expect(screen.getAllByRole('button', { name: chipName })).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: 'Gamma' })).toHaveLength(1);
    expect(toggle('Gamma')).toHaveAttribute('aria-expanded');
  });

  it('Happy path: a link scrolls the Section and focuses its title control', async () => {
    const { user } = await open();

    await user.click(
      within(screen.getByRole('navigation', { name: message('sheet.sections') })).getByRole(
        'button',
        {
          name: messageWith('sheet.sectionLink', { label: 'Gamma' }),
        },
      ),
    );

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start' });
    expect(scrollIntoView.mock.instances[0]).toBe(
      document.querySelector('[data-section-key="gamma"]'),
    );
    expect(toggle('Gamma')).toHaveFocus();
  });

  it('Edge: jumping to a collapsed Section expands it first', async () => {
    const { user } = await open();
    await user.click(toggle('Gamma'));
    expect(toggle('Gamma')).toHaveAttribute('aria-expanded', 'false');

    await user.click(
      within(screen.getByRole('navigation', { name: message('sheet.sections') })).getByRole(
        'button',
        {
          name: messageWith('sheet.sectionLink', { label: 'Gamma' }),
        },
      ),
    );

    expect(toggle('Gamma')).toHaveAttribute('aria-expanded', 'true');
    expect(toggle('Gamma')).toHaveFocus();
  });
});

describe('the pinned filter', () => {
  it('Edge: it starts off when nothing is remembered for this sheet', async () => {
    await open({ layout: PINS });

    expect(filter()).toHaveAttribute('aria-pressed', 'false');
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('Edge: with nothing pinned it explains itself and offers Edit pins', async () => {
    const { user } = await open();

    await user.click(filter());

    expect(screen.getByText(message('sheet.pinnedEmpty'))).toBeInTheDocument();
    expect(rowKeys()).toEqual([]);
    expect(screen.getAllByRole('button', { name: message('sheet.editPins') })).toHaveLength(1);
  });

  it('Edge: Edit pins enters pin mode and Done returns to the empty filter', async () => {
    const { user, stub } = await open();
    await user.click(filter());

    await user.click(screen.getByRole('button', { name: message('sheet.editPins') }));

    expect(screen.getByRole('button', { name: message('arrange.done') })).toBeInTheDocument();
    expect(filter()).toBeDisabled();
    expect(filter()).toHaveAttribute('aria-pressed', 'false');
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(screen.getAllByRole('button', { name: /^Pin (?!rows$)/ })).toHaveLength(7);

    await user.click(screen.getByRole('button', { name: message('arrange.done') }));

    expect(filter()).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText(message('sheet.pinnedEmpty'))).toBeInTheDocument();
    expect(stub.puts()).toHaveLength(0);
  });

  it('Happy path: with pins it draws only pinned Rows, in pin order, in their own Sections', async () => {
    const { user } = await open({ layout: PINS });

    await user.click(filter());

    expect(rowKeys()).toEqual(['alpha.two', 'alpha.one', 'beta.note']);
    expect(sectionKeys()).toEqual(['alpha', 'beta']);
    expect(
      within(screen.getByRole('navigation', { name: message('sheet.sections') })).getAllByRole(
        'button',
      ),
    ).toHaveLength(2);
    expect(document.querySelector('.lg-section__toggle')).toBeNull();
    expect(
      screen.queryByRole('button', { name: message('sheet.collapseAll') }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.editPins') })).toBeInTheDocument();
  });

  it('Edge: the toggle is announced once through the shared polite region, both ways', async () => {
    const { user } = await open({ layout: PINS });
    const live = screen.getByTestId('arrange-live');

    await user.click(filter());
    expect(live).toHaveTextContent(messageWith('sheet.pinnedShowing', { count: 3 }));

    await user.click(filter());
    expect(live).toHaveTextContent(message('sheet.pinnedShowingAll'));
    expect(document.querySelectorAll('[aria-live]').length).toBeLessThanOrEqual(2);
  });

  it('Edge: collapsed Sections are untouched while the filter is on and come back when it is off', async () => {
    const { user } = await open({ layout: PINS });
    await user.click(toggle('Beta'));

    await user.click(filter());
    expect(rowKeys()).toContain('beta.note');
    await user.click(filter());

    expect(toggle('Beta')).toHaveAttribute('aria-expanded', 'false');
  });

  it('Happy path: Edit pins with pins enters pin mode and Done leaves it back on the filter', async () => {
    const { user, stub } = await open({ layout: PINS });
    await user.click(filter());

    await user.click(screen.getByRole('button', { name: message('sheet.editPins') }));
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    await user.click(screen.getByRole('button', { name: message('arrange.done') }));

    expect(filter()).toBeEnabled();
    expect(filter()).toHaveAttribute('aria-pressed', 'true');
    expect(sectionKeys()).toEqual(['alpha', 'beta']);
    expect(stub.puts()).toHaveLength(0);
  });

  it('Edge: unpinning every Row in Arrange mode and finishing leaves the filter on with its empty state', async () => {
    const { user, stub } = await open({ layout: { section_order: [], pinned: ['alpha.one'] } });
    await user.click(filter());
    await user.click(screen.getByRole('button', { name: message('sheet.editPins') }));

    await user.click(screen.getByRole('button', { name: 'Pin One' }));
    await user.click(screen.getByRole('button', { name: message('arrange.done') }));
    await waitFor(() => expect(stub.puts()).toHaveLength(1));

    expect(screen.getByText(message('sheet.pinnedEmpty'))).toBeInTheDocument();
    expect(filter()).toHaveAttribute('aria-pressed', 'true');
    expect(JSON.parse(stub.puts()[0].body ?? '')).toEqual({ layout: null });
  });
});

describe('the view and the value cycle stay apart', () => {
  async function openWithPreview(layout?: unknown) {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(sheetPayload(layout === undefined ? {} : { layout })),
      successOutcome(templateDetailPayload()),
      successOutcome({ derived: { total: 5 } }),
    );
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    return { user, fake };
  }

  it('Happy path: a collapsed Section holding preview marks says how many Rows changed', async () => {
    const { user } = await openWithPreview();
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));
    await screen.findByTestId('sheet-status');

    await user.click(toggle(/^Derived/));
    await user.click(toggle(/^Identity/));

    const derived = toggle(/^Derived/);
    expect(derived).toHaveTextContent(messageWith('engine.sectionChanged', { count: 1 }));
    expect(toggle(/^Identity/)).toHaveTextContent(
      messageWith('engine.sectionChanged', { count: 1 }),
    );
    expect(toggle(/^Ability/)).not.toHaveTextContent('changed');

    await user.click(derived);
    expect(toggle('Derived')).not.toHaveTextContent('changed');
  });

  it('Edge: Jump to first expands the collapsed Section holding the first recalculated Row', async () => {
    const { user } = await openWithPreview();
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));
    await screen.findByTestId('sheet-status');
    await user.click(toggle(/^Derived/));
    expect(toggle(/^Derived/)).toHaveAttribute('aria-expanded', 'false');

    await user.click(screen.getByRole('button', { name: message('sheet.jumpToFirstName') }));

    expect(toggle(/^Derived/)).toHaveAttribute('aria-expanded', 'true');
    expect(document.querySelector('[data-row-key="derived.total"]')).toHaveFocus();
  });

  it('Edge: Jump to first turns the pinned filter off when the Row is not drawn under it', async () => {
    const { user } = await openWithPreview({
      section_order: [],
      pinned: ['identity.characterName'],
    });
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));
    await screen.findByTestId('sheet-status');
    await user.click(filter());
    expect(document.querySelector('[data-row-key="derived.total"]')).toBeNull();

    await user.click(screen.getByRole('button', { name: message('sheet.jumpToFirstName') }));

    expect(filter()).toHaveAttribute('aria-pressed', 'false');
    expect(document.querySelector('[data-row-key="derived.total"]')).toHaveFocus();
  });

  it('Edge: Ctrl+S still saves the values, with the filter on and Sections collapsed', async () => {
    const { user, fake } = await openWithPreview();
    await user.click(toggle(/^Derived/));
    await user.click(filter());

    await act(async () => {
      await user.keyboard('{Control>}s{/Control}');
    });

    await waitFor(() =>
      expect(fake.requests.some((request) => request.method === 'PATCH')).toBe(true),
    );
  });
});
