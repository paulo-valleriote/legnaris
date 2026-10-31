import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { createApiClient } from '../api/client';
import { sheetKey } from '../catalog/queries';
import type { Sheet } from '../api/models/sheet';
import { errorOutcome, successOutcome } from './fake_transport';
import { message, messageWith } from '../l10n/strings';
import { SCHEMA_VERSION, createMemorySheetStore } from '../library/sheet_store';
import { renderApp } from './render';
import type { ArrangeStubOptions } from './arrange_stub';
import { arrangeTemplatePayload, createArrangeStub } from './arrange_stub';
import { SHEET_ID } from './sheet_fixtures';

const SHEET_URL = `/sheets/${SHEET_ID}`;
const BASE_URL = 'http://api.test';

async function open(
  options: ArrangeStubOptions = {},
  extra: { sheetStore?: ReturnType<typeof createMemorySheetStore>; queryClient?: QueryClient } = {},
) {
  const stub = createArrangeStub(options);
  const user = userEvent.setup();
  const view = renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
    ...extra,
  });
  await screen.findByTestId('sheet-name');
  return { stub, user, ...view };
}

async function arrange(options: ArrangeStubOptions = {}, extra = {}) {
  const opened = await open(options, extra);
  await opened.user.click(screen.getByRole('button', { name: message('arrange.start') }));
  return opened;
}

const sectionKeys = () =>
  Array.from(document.querySelectorAll<HTMLElement>('[data-section-key]')).map(
    (e) => e.dataset.sectionKey,
  );

const rowKeys = (scope: ParentNode = document) =>
  Array.from(scope.querySelectorAll<HTMLElement>('[data-row-key]')).map((e) => e.dataset.rowKey);

const row = (key: string) => document.querySelector(`[data-row-key="${key}"]`) as HTMLElement;
const section = (key: string) =>
  document.querySelector(`[data-section-key="${key}"]`) as HTMLElement;
const live = () => screen.getByTestId('arrange-live');
const star = (key: string) => within(row(key)).getByRole('button', { name: /^Pin (?!rows$)/ });
const done = () => screen.getByRole('button', { name: message('arrange.done') });
const startButton = () => screen.getByRole('button', { name: message('arrange.start') });
const sectionHandle = (label: string) =>
  screen.getByRole('button', { name: messageWith('arrange.handle', { label }) });

afterEach(() => {
  vi.restoreAllMocks();
});

describe('Arrange mode is explicit and separate from play mode', () => {
  it('Edge: outside Arrange mode there are no handles, no stars and no drag wiring', async () => {
    await open();

    expect(screen.queryByRole('button', { name: /^Reorder /i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Pin (?!rows$)/i })).not.toBeInTheDocument();
    expect(document.querySelector('[data-row-key] [aria-pressed]')).toBeNull();
    expect(document.querySelector('[aria-roledescription="sortable"]')).toBeNull();
    expect(document.querySelector('[id^="DndDescribedBy"], [id^="DndLiveRegion"]')).toBeNull();
    expect(startButton()).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'One' })).toBeInTheDocument();
  });

  it('Edge: a template with no sections offers no Arrange mode', async () => {
    await open({ template: { id: 'fantasy_v1', name: 'Empty', default: true, fields: {} } });

    expect(
      screen.queryByRole('button', { name: message('arrange.start') }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: Arrange swaps the value footer for Reset, Cancel and Done; a handle per Section and a star per Row, nothing else', async () => {
    await arrange();

    expect(done()).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('arrange.cancel') })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('arrange.reset') })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('sheet.save') })).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: message('arrange.start') }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /^Reorder /i })).toHaveLength(3);
    for (const label of ['Alpha', 'Beta', 'Gamma']) {
      expect(sectionHandle(label)).toBeInTheDocument();
    }
    expect(screen.getAllByRole('button', { name: /^Pin (?!rows$)/i })).toHaveLength(7);
    for (const rowEl of document.querySelectorAll('[data-row-key]')) {
      expect(within(rowEl as HTMLElement).getAllByRole('button')).toHaveLength(1);
    }
    expect(screen.getByTestId('arrange-hint')).toHaveFocus();
    expect(screen.queryByRole('textbox', { name: 'One' })).not.toBeInTheDocument();
  });

  it('Edge: there is no Move up or Move down control and no Pinned zone anywhere', async () => {
    const { user } = await arrange();
    await user.click(star('alpha.two'));

    expect(screen.queryByRole('button', { name: /move (up|down)/i })).not.toBeInTheDocument();
    expect(document.querySelector('[data-zone]')).toBeNull();
    expect(screen.queryByText(/^Pinned$/)).not.toBeInTheDocument();
  });

  it('Edge: only Section handles carry the sortable semantics and the keyboard instructions', async () => {
    await arrange();

    const handle = sectionHandle('Alpha');
    expect(handle.tagName).toBe('BUTTON');
    expect(handle).toHaveAttribute('tabindex', '0');
    expect(handle).toHaveAttribute('aria-roledescription', 'sortable');
    const describedBy = handle.getAttribute('aria-describedby') ?? '';
    expect(document.getElementById(describedBy)).toHaveTextContent(message('arrange.instructions'));
    expect(document.querySelectorAll('[aria-roledescription="sortable"]')).toHaveLength(3);
    expect(row('alpha.one').querySelector('[aria-roledescription]')).toBeNull();
  });

  it('Edge: Rows are not draggable: a drag gesture on a Row changes nothing', async () => {
    const { user } = await arrange();

    await user.pointer([
      { keys: '[MouseLeft>]', target: row('alpha.one') },
      { target: row('alpha.three') },
      { keys: '[/MouseLeft]' },
    ]);

    expect(rowKeys(section('alpha'))).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
    expect(screen.queryByTestId('arrange-pending')).not.toBeInTheDocument();
  });
});

describe('the pin star', () => {
  it('Happy path: it is a toggle button with one fixed name, aria-pressed following the state', async () => {
    const { user } = await arrange();
    const button = star('alpha.two');

    expect(button.tagName).toBe('BUTTON');
    expect(button).toHaveAccessibleName(messageWith('arrange.pinLabel', { label: 'Two' }));
    expect(button).toHaveAttribute('aria-pressed', 'false');

    await user.click(button);

    expect(star('alpha.two')).toHaveAccessibleName(
      messageWith('arrange.pinLabel', { label: 'Two' }),
    );
    expect(star('alpha.two')).toHaveAttribute('aria-pressed', 'true');
    expect(live()).toHaveTextContent(messageWith('arrange.pinned', { label: 'Two' }));
  });

  it('Edge: the star is the Row only control and carries no text, only an accessible name', async () => {
    await arrange();

    const button = star('alpha.one');
    expect(button).toHaveTextContent('');
    expect(button.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    expect(button.querySelector('[href], image, use')).toBeNull();
  });

  it('Edge: focus stays on the star after pinning and after unpinning, even when the browser drops focus from a moved node (React restores it)', async () => {
    const { user } = await arrange();
    const insertBefore = Node.prototype.insertBefore;
    const appendChild = Node.prototype.appendChild;
    const blurIfMovedWhileFocused = (node: Node, move: () => Node) => {
      const focused = document.activeElement;
      const holdsFocus = focused instanceof HTMLElement && node.contains(focused);
      const result = move();
      process.stdout.write(`MOVE ${(node as HTMLElement).tagName} ${holdsFocus}\n`);
      if (holdsFocus) {
        focused.blur();
      }
      return result;
    };
    vi.spyOn(Node.prototype, 'insertBefore').mockImplementation(function (
      this: Node,
      node: Node,
      ref: Node | null,
    ) {
      return blurIfMovedWhileFocused(node, () => insertBefore.call(this, node, ref)) as never;
    });
    vi.spyOn(Node.prototype, 'appendChild').mockImplementation(function (this: Node, node: Node) {
      return blurIfMovedWhileFocused(node, () => appendChild.call(this, node)) as never;
    });

    await user.click(star('alpha.three'));
    expect(star('alpha.three')).toHaveFocus();

    await user.click(star('alpha.three'));
    expect(star('alpha.three')).toHaveFocus();
    expect(live()).toHaveTextContent(messageWith('arrange.unpinned', { label: 'Three' }));
  });

  it('Edge: Space and Enter toggle the star from the keyboard', async () => {
    const { user } = await arrange();
    star('alpha.two').focus();

    await user.keyboard(' ');
    expect(star('alpha.two')).toHaveAttribute('aria-pressed', 'true');
    expect(star('alpha.two')).toHaveFocus();

    await user.keyboard('{Enter}');
    expect(star('alpha.two')).toHaveAttribute('aria-pressed', 'false');
  });

  it('Edge: every toggle replaces the announcement: pinned, then unpinned, then pinned again', async () => {
    const { user } = await arrange();

    await user.click(star('alpha.two'));
    expect(live()).toHaveTextContent(messageWith('arrange.pinned', { label: 'Two' }));
    await user.click(star('alpha.two'));
    expect(live()).toHaveTextContent(messageWith('arrange.unpinned', { label: 'Two' }));
    await user.click(star('alpha.two'));
    expect(live()).toHaveTextContent(messageWith('arrange.pinned', { label: 'Two' }));
  });
});

describe('pinning inside a Section', () => {
  it('Happy path: a pinned Row rises to the top of its own Section, the rest keep Template order', async () => {
    const { user } = await arrange();

    await user.click(star('alpha.three'));

    expect(rowKeys(section('alpha'))).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
  });

  it('Edge: pinned Rows are ordered by pin time and unpinning returns the Row to its Template position', async () => {
    const { user } = await arrange();
    await user.click(star('alpha.three'));
    await user.click(star('alpha.two'));
    expect(rowKeys(section('alpha'))).toEqual(['alpha.three', 'alpha.two', 'alpha.one']);

    await user.click(star('alpha.three'));
    expect(rowKeys(section('alpha'))).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);

    await user.click(star('alpha.two'));
    expect(rowKeys(section('alpha'))).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
    expect(screen.queryByTestId('arrange-pending')).not.toBeInTheDocument();
  });

  it('Edge: pins in several Sections act independently and never cross Sections', async () => {
    const { user } = await arrange();

    await user.click(star('beta#pick'));
    await user.click(star('alpha.two'));
    await user.click(star('beta.stats'));

    expect(rowKeys(section('alpha'))).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);
    expect(rowKeys(section('beta'))).toEqual(['beta#pick', 'beta.stats', 'beta.note']);
    expect(rowKeys(section('gamma'))).toEqual(['gamma.solo']);
  });

  it('Edge: a Section whose every Row is pinned still renders with all its Rows', async () => {
    const { user } = await arrange();

    await user.click(star('gamma.solo'));

    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(rowKeys(section('gamma'))).toEqual(['gamma.solo']);
    expect(star('gamma.solo')).toHaveAttribute('aria-pressed', 'true');
  });

  it('Edge: pinning every Row of a Section keeps pin order and Section placement', async () => {
    const { user } = await arrange();

    for (const key of ['alpha.three', 'alpha.one', 'alpha.two']) {
      await user.click(star(key));
    }

    expect(rowKeys(section('alpha'))).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('Edge: a Row never shows twice, and the pinned state is mirrored by exactly the pinned stars', async () => {
    const { user } = await arrange();
    await user.click(star('alpha.two'));
    await user.click(star('beta.stats'));

    expect(document.querySelectorAll('[data-row-key]')).toHaveLength(7);
    const pressed = Array.from(document.querySelectorAll('[aria-pressed="true"]')).map((button) =>
      button.closest('[data-row-key]')?.getAttribute('data-row-key'),
    );
    expect(pressed.sort()).toEqual(['alpha.two', 'beta.stats']);
  });
});

describe('committing', () => {
  it('Happy path: Done sends exactly one PUT with only the pins, in pin order, leaves Arrange mode and keeps the order', async () => {
    const { stub, user } = await arrange();
    await user.click(star('gamma.solo'));
    await user.click(star('alpha.two'));
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
    expect(stub.puts()).toHaveLength(0);

    await user.click(done());

    await waitFor(() => expect(stub.puts()).toHaveLength(1));
    const put = stub.puts()[0];
    expect(put.url).toBe(`${BASE_URL}/sheets/${SHEET_ID}/layout`);
    expect(JSON.parse(put.body ?? '')).toEqual({
      layout: { pinned: ['gamma.solo', 'alpha.two'] },
    });
    expect(put.body).not.toContain('row_order');
    expect(put.body).not.toContain('section_order');
    await waitFor(() => expect(startButton()).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /^Reorder /i })).not.toBeInTheDocument();
    expect(rowKeys(section('gamma'))).toEqual(['gamma.solo']);
    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(rowKeys(section('alpha'))).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);
    expect(startButton()).toHaveFocus();
    expect(stub.patches()).toHaveLength(0);
  });

  it('Edge: the committed layout reaches the sheet cache and the local record, so reopening shows it with no fetch', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const sheetStore = createMemorySheetStore();
    const { stub, user } = await arrange({}, { queryClient, sheetStore });
    await user.click(star('alpha.three'));

    await user.click(done());

    await waitFor(() => {
      const cached = queryClient.getQueryData<Sheet>(sheetKey(SHEET_ID));
      expect(cached?.layout?.pinned).toEqual(['alpha.three']);
    });
    await waitFor(async () => {
      expect((await sheetStore.readSheet(SHEET_ID))?.layout?.pinned).toEqual(['alpha.three']);
    });
    expect(
      stub.requests.filter((r) => r.method === 'GET' && r.url.endsWith(SHEET_ID)),
    ).toHaveLength(1);
  });

  it('Edge: Done with nothing changed sends no request', async () => {
    const { stub, user } = await arrange();

    await user.click(done());

    expect(stub.puts()).toHaveLength(0);
    expect(startButton()).toBeInTheDocument();
  });

  it('Edge: Done twice quickly sends one PUT', async () => {
    const { stub, user } = await arrange();
    await user.click(star('alpha.one'));

    const button = done();
    fireEvent.click(button);
    fireEvent.click(button);

    await waitFor(() => expect(startButton()).toBeInTheDocument());
    expect(stub.puts()).toHaveLength(1);
  });

  it('Happy path: Cancel discards every staged change without a request', async () => {
    const { stub, user } = await arrange();
    await user.click(star('alpha.three'));
    await user.click(star('gamma.solo'));

    await user.click(screen.getByRole('button', { name: message('arrange.cancel') }));

    expect(stub.puts()).toHaveLength(0);
    expect(rowKeys(section('alpha'))).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
    expect(document.querySelector('[data-row-key] [aria-pressed]')).toBeNull();
    expect(startButton()).toHaveFocus();
    await user.click(startButton());
    expect(screen.queryByTestId('arrange-pending')).not.toBeInTheDocument();
  });

  it('Edge: Reset to default stages the Template order and Done commits {"layout": null}', async () => {
    const { stub, user } = await arrange({
      layout: { section_order: ['gamma'], pinned: ['alpha.one'] },
    });
    expect(sectionKeys()).toEqual(['gamma', 'alpha', 'beta']);

    await user.click(screen.getByRole('button', { name: message('arrange.reset') }));

    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(rowKeys(section('alpha'))).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
    expect(document.querySelector('[data-row-key] [aria-pressed="true"]')).toBeNull();
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
    expect(stub.puts()).toHaveLength(0);

    await user.click(done());

    await waitFor(() => expect(stub.puts()).toHaveLength(1));
    expect(JSON.parse(stub.puts()[0].body ?? '')).toEqual({ layout: null });
  });

  it('Error: a failed PUT keeps the staging with the mapped copy and Done retries', async () => {
    const { stub, user } = await arrange({
      putOutcomes: [
        errorOutcome('INTERNAL_ERROR', 'boom', 500),
        successOutcome({ layout: { pinned: ['alpha.one'] } }),
      ],
    });
    await user.click(star('alpha.one'));

    await user.click(done());

    expect(await screen.findByTestId('arrange-error')).toHaveTextContent(
      message('api.error.serverError'),
    );
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
    expect(rowKeys(section('alpha'))[0]).toBe('alpha.one');
    expect(star('alpha.one')).toHaveAttribute('aria-pressed', 'true');
    expect(done()).toBeEnabled();

    await user.click(done());

    await waitFor(() => expect(startButton()).toBeInTheDocument());
    expect(stub.puts()).toHaveLength(2);
    expect(stub.puts()[1].body).toBe(stub.puts()[0].body);
  });

  it('Error: a 404 on the PUT shows the missing-sheet copy and keeps the staging', async () => {
    const { user } = await arrange({ putOutcomes: [errorOutcome('NOT_FOUND', 'gone', 404)] });
    await user.click(star('alpha.one'));

    await user.click(done());

    expect(await screen.findByTestId('arrange-error')).toHaveTextContent(
      message('api.error.notFound'),
    );
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
  });

  it('Error: a 400 shows the invalid-change copy', async () => {
    const { user } = await arrange({ putOutcomes: [errorOutcome('INVALID_REQUEST', 'bad', 400)] });
    await user.click(star('alpha.one'));

    await user.click(done());

    expect(await screen.findByTestId('arrange-error')).toHaveTextContent(
      message('api.error.invalidEdit'),
    );
  });
});

describe('a layout is not a value', () => {
  it('Edge: arranging never marks the sheet as having unsaved values, and Ctrl+S saves nothing while arranging', async () => {
    const { stub, user } = await arrange();
    await user.click(star('alpha.one'));

    await user.keyboard('{Control>}s{/Control}');

    expect(screen.queryByTestId('sheet-status')).not.toBeInTheDocument();
    expect(stub.patches()).toHaveLength(0);
    await user.click(screen.getByRole('button', { name: message('arrange.cancel') }));
    expect(screen.queryByTestId('sheet-status')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
  });

  it('Edge: pending value edits survive an Arrange session and the value save carries no layout', async () => {
    const { stub, user } = await open();
    const one = screen.getByRole('textbox', { name: 'One' });
    await user.clear(one);
    await user.type(one, '5');
    await user.click(startButton());
    await user.click(star('alpha.one'));
    await user.click(done());
    await waitFor(() => expect(stub.puts()).toHaveLength(1));
    await waitFor(() => expect(startButton()).toBeInTheDocument());

    expect(screen.getByTestId('sheet-status')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'One' })).toHaveValue('5');
    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    await waitFor(() => expect(stub.patches()).toHaveLength(1));
    expect(JSON.parse(stub.patches()[0].body ?? '')).toEqual({ data: { alpha: { one: 5 } } });
    expect(stub.puts()).toHaveLength(1);
  });

  it('Edge: the layout PUT never carries values', async () => {
    const { stub, user } = await arrange();
    await user.click(star('alpha.one'));
    await user.click(done());
    await waitFor(() => expect(stub.puts()).toHaveLength(1));

    expect(Object.keys(JSON.parse(stub.puts()[0].body ?? ''))).toEqual(['layout']);
  });
});

describe('opening a sheet that has a layout', () => {
  it('Happy path: the stored layout is drawn in play mode: Section order and pins, with no Pinned zone', async () => {
    await open({
      layout: { section_order: ['gamma', 'alpha'], pinned: ['alpha.three', 'beta#pick'] },
    });

    expect(sectionKeys()).toEqual(['gamma', 'alpha', 'beta']);
    expect(rowKeys(section('alpha'))).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
    expect(rowKeys(section('beta'))).toEqual(['beta#pick', 'beta.note', 'beta.stats']);
    expect(document.querySelector('[data-zone]')).toBeNull();
    expect(screen.getAllByText(/^Pinned$/)).toHaveLength(2);
    expect(within(row('alpha.three')).getByText(/^Pinned$/)).toBeInTheDocument();
    expect(within(row('beta#pick')).getByText(/^Pinned$/)).toBeInTheDocument();
    expect(within(row('alpha.one')).queryByText(/^Pinned$/)).not.toBeInTheDocument();
  });

  it('Edge: entering Arrange mode starts from the stored layout with the right stars on', async () => {
    const { user } = await open({ layout: { pinned: ['alpha.three'] } });

    await user.click(startButton());

    expect(rowKeys(section('alpha'))).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
    expect(star('alpha.three')).toHaveAttribute('aria-pressed', 'true');
    expect(star('alpha.one')).toHaveAttribute('aria-pressed', 'false');
    expect(screen.queryByTestId('arrange-pending')).not.toBeInTheDocument();
  });

  it('Edge: stale, duplicate and foreign keys in the stored layout are ignored', async () => {
    await open({
      layout: {
        section_order: ['ghost', 'beta', 'beta'],
        pinned: ['nope', 'alpha.ghost', 'gamma.solo', 'gamma.solo', 'alpha.two'],
      },
    });

    expect(sectionKeys()).toEqual(['beta', 'alpha', 'gamma']);
    expect(rowKeys(section('gamma'))).toEqual(['gamma.solo']);
    expect(rowKeys(section('alpha'))).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);
    expect(document.querySelectorAll('[data-row-key="gamma.solo"]')).toHaveLength(1);
  });

  it('Edge: a retired row_order in the stored layout is ignored', async () => {
    await open({ layout: { row_order: { alpha: ['alpha.three', 'alpha.one'] } } });

    expect(rowKeys(section('alpha'))).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
  });

  it('Edge: an empty stored layout is the Template order', async () => {
    await open({ layout: {} });

    expect(sectionKeys()).toEqual(['alpha', 'beta', 'gamma']);
    expect(rowKeys(section('alpha'))).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
  });

  it('Error: a malformed stored layout is a mapped parse failure, not a half-drawn sheet', async () => {
    const stub = createArrangeStub({ layout: { pinned: 'alpha.one' } });
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
    });

    expect(await screen.findByText(message('api.error.parse'))).toBeInTheDocument();
  });
});

describe('leaving with a pending arrangement', () => {
  const catalogLink = () => screen.getByRole('link', { name: message('nav.catalog') });

  it('Happy path: leaving asks through the one modal, Keep editing stays with the staging intact', async () => {
    const { user, stub } = await arrange();
    await user.click(star('alpha.one'));

    await user.click(catalogLink());

    expect(screen.getByTestId('sheet-confirm-leave')).toBeInTheDocument();
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
    await user.click(screen.getByRole('button', { name: message('sheet.cancel') }));
    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
    expect(star('alpha.one')).toHaveAttribute('aria-pressed', 'true');
    expect(stub.puts()).toHaveLength(0);
  });

  it('Happy path: confirming leaves without a PUT and drops the backup', async () => {
    const sheetStore = createMemorySheetStore();
    const { user, stub } = await arrange({}, { sheetStore });
    await user.click(star('alpha.one'));
    await waitFor(async () => expect(await sheetStore.readLayoutDraft(SHEET_ID)).not.toBeNull());

    await user.click(catalogLink());
    await user.click(screen.getByRole('button', { name: message('sheet.leaveConfirm') }));

    expect(await screen.findByTestId('catalog')).toBeInTheDocument();
    expect(stub.puts()).toHaveLength(0);
    await waitFor(async () => expect(await sheetStore.readLayoutDraft(SHEET_ID)).toBeNull());
  });

  it('Edge: arranging with no change does not guard leaving', async () => {
    const { user } = await arrange();

    await user.click(catalogLink());

    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(await screen.findByTestId('catalog')).toBeInTheDocument();
  });

  it('Edge: leaving while the PUT is in flight is a no-op', async () => {
    let release: (value: unknown) => void = () => undefined;
    const stub = createArrangeStub();
    const slow = (request: Parameters<typeof stub.transport>[0]) =>
      request.method === 'PUT'
        ? new Promise<{ status: number; body: string }>((resolve) => {
            release = () =>
              resolve({
                status: 200,
                body: JSON.stringify({ data: { layout: { pinned: ['alpha.one'] } } }),
              });
          })
        : stub.transport(request);
    const user = userEvent.setup();
    renderApp(SHEET_URL, { client: createApiClient({ baseUrl: BASE_URL, transport: slow }) });
    await screen.findByTestId('sheet-name');
    await user.click(startButton());
    await user.click(star('alpha.one'));
    await user.click(done());

    await user.click(catalogLink());

    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-screen')).toBeInTheDocument();
    await act(async () => {
      release(undefined);
    });
  });

  it('Edge: a refresh and a leave never stack two modals', async () => {
    const { user } = await arrange();
    await user.click(star('alpha.one'));

    await user.keyboard('{F5}');
    await user.click(catalogLink());

    expect(screen.getByTestId('sheet-confirm-refresh')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-confirm-leave')).not.toBeInTheDocument();
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
  });

  it('Happy path: confirming a refresh discards the staging', async () => {
    const { user, stub } = await arrange();
    await user.click(star('alpha.one'));

    await user.keyboard('{F5}');
    await user.click(screen.getByRole('button', { name: message('sheet.refreshConfirm') }));

    await waitFor(() => expect(startButton()).toBeInTheDocument());
    expect(rowKeys(section('alpha'))).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
    expect(stub.puts()).toHaveLength(0);
  });
});

describe('pending arrangement backup', () => {
  it('Happy path: a staged change is backed up and a reopened sheet resumes Arrange mode with it', async () => {
    const sheetStore = createMemorySheetStore();
    const first = await arrange({}, { sheetStore });
    await first.user.click(star('alpha.one'));
    await waitFor(async () =>
      expect(await sheetStore.readLayoutDraft(SHEET_ID)).toEqual({
        layout: { sectionOrder: [], pinned: ['alpha.one'] },
      }),
    );
    first.unmount();

    await open({}, { sheetStore });

    expect(
      await screen.findByRole('button', { name: message('arrange.done') }),
    ).toBeInTheDocument();
    expect(star('alpha.one')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('arrange-pending')).toBeInTheDocument();
    expect(JSON.stringify(sheetStore.records())).not.toMatch(
      /token|password|secret|authorization|cookie|credential/i,
    );
  });

  it('Edge: a corrupt backup is ignored', async () => {
    const sheetStore = createMemorySheetStore();
    sheetStore.putRawLayoutDraft(SHEET_ID, {
      schemaVersion: SCHEMA_VERSION + 1,
      id: SHEET_ID,
      layout: null,
    });

    await open({}, { sheetStore });

    expect(startButton()).toBeInTheDocument();
  });
});

describe('literal text and motion', () => {
  it('Edge: markup in Section and Row labels stays literal in the arrange chrome and the announcements', async () => {
    const markup = '<img src=x onerror=alert(1)>';
    const stub = createArrangeStub({
      template: arrangeTemplatePayload({ alpha: markup, one: markup }),
    });
    const user = userEvent.setup();
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
    });
    await screen.findByTestId('sheet-name');
    expect(document.querySelectorAll('img')).toHaveLength(0);

    await user.click(startButton());
    await user.click(
      within(row('alpha.one')).getByRole('button', {
        name: messageWith('arrange.pinLabel', { label: markup }),
      }),
    );

    expect(document.querySelectorAll('img')).toHaveLength(0);
    expect(sectionHandle(markup)).toBeInTheDocument();
    expect(row('alpha.one')).toHaveTextContent(markup);
    expect(live().textContent).toContain(markup);
  });
});
