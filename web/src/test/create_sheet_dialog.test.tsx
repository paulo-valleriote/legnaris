import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { UserEvent } from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import type { TemplateSummary } from '../api/models/template';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import { message } from '../l10n/strings';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  FREE_PLAN_USAGE,
  PAID_PLAN_USAGE,
  successOutcome,
  withPlanUsage,
} from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

const CATALOG: readonly TemplateSummary[] = [
  {
    id: 'dnd5e',
    name: 'Dungeons & Dragons 5e',
    default: true,
    kinds: ['character', 'npc', 'enemy'],
    fields: {},
  },
  {
    id: 'pbta',
    name: 'Powered by the Apocalypse',
    default: false,
    kinds: ['character', 'npc', 'enemy'],
    fields: {},
  },
];

const ENTRY_NAME = /Dungeons & Dragons 5e/;

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function detailEnvelope(name: string, id = 'dnd5e') {
  return successOutcome({
    id,
    name,
    default: true,
    kinds: ['character', 'npc', 'enemy'],
    fields: {},
  });
}

async function openSurface(user: UserEvent, plan: unknown = FREE_PLAN_USAGE) {
  const fake = createFakeTransport(
    successOutcome(CATALOG),
    detailEnvelope('Dungeons & Dragons 5e'),
  );
  renderApp('/', { client: clientWith(withPlanUsage(fake.transport, successOutcome(plan))) });
  await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
  await screen.findByTestId('detail-name');
  return fake;
}

describe('creation surface detail', () => {
  it('Happy path: activating an entry opens the surface and issues exactly one detail request for that identifier', async () => {
    const user = userEvent.setup();
    const fake = await openSurface(user);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByTestId('detail-name')).toHaveTextContent('Dungeons & Dragons 5e');
    expect(fake.attempts).toBe(2);
    expect(fake.lastRequest?.url).toContain('/templates/dnd5e');
  });

  it('Edge: while the detail is in flight it shows the loading copy and no previously opened detail', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(deferred.transport)) });

    deferred.resolve(successOutcome(CATALOG));
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));

    expect(screen.getByText(message('sheet.loading'))).toBeInTheDocument();
    expect(screen.queryByTestId('detail-name')).not.toBeInTheDocument();

    deferred.resolve(detailEnvelope('Dungeons & Dragons 5e'));
    await screen.findByTestId('detail-name');
  });

  it('Edge: a previously opened template is not shown when a new detail is in flight', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(deferred.transport)) });

    deferred.resolve(successOutcome(CATALOG));
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    deferred.resolve(detailEnvelope('Dungeons & Dragons 5e'));
    await screen.findByTestId('detail-name');

    await user.click(screen.getByRole('button', { name: message('sheet.close') }));
    await user.click(screen.getByRole('button', { name: /Powered by the Apocalypse/ }));

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText(message('sheet.loading'))).toBeInTheDocument();
    expect(within(dialog).queryByText('Dungeons & Dragons 5e')).not.toBeInTheDocument();
  });

  it('Edge: an empty template name renders as empty text without throwing', async () => {
    const fake = createFakeTransport(successOutcome(CATALOG), detailEnvelope(''));
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(fake.transport)) });

    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));

    await waitFor(() => expect(screen.getByTestId('detail-name')).toBeInTheDocument());
    expect(screen.getByTestId('detail-name').textContent).toBe('');
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('Edge: a failure shows the mapped copy inline and keeps the surface open', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(deferred.transport)) });

    deferred.resolve(successOutcome(CATALOG));
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    deferred.resolve(errorOutcome('INTERNAL_ERROR', 'boom', 500));

    await waitFor(() =>
      expect(screen.getByText(failureMessage('serverError'))).toBeInTheDocument(),
    );
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('common.retry') })).toBeInTheDocument();
  });

  it('Edge: a missing template shows the missing-template copy inline without closing or crashing', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(deferred.transport)) });

    deferred.resolve(successOutcome(CATALOG));
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    deferred.resolve(errorOutcome('NOT_FOUND', 'gone', 404));

    await waitFor(() => expect(screen.getByText(failureMessage('notFound'))).toBeInTheDocument());
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('Edge: a malformed payload shows the parse-failure copy inline and never a half-filled detail', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(deferred.transport)) });

    deferred.resolve(successOutcome(CATALOG));
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    deferred.resolve({ status: 200, body: JSON.stringify({ data: { id: 'dnd5e' } }) });

    await waitFor(() => expect(screen.getByText(parseFailureMessage())).toBeInTheDocument());
    expect(screen.queryByTestId('detail-name')).not.toBeInTheDocument();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('Edge: malformed kinds show the parse-failure copy and no half-filled detail', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(deferred.transport)) });

    deferred.resolve(successOutcome(CATALOG));
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    deferred.resolve(
      successOutcome({
        id: 'dnd5e',
        name: 'Dungeons & Dragons 5e',
        default: true,
        kinds: ['dragon'],
        fields: {},
      }),
    );

    await waitFor(() => expect(screen.getByText(parseFailureMessage())).toBeInTheDocument());
    expect(screen.queryByTestId('detail-name')).not.toBeInTheDocument();
    expect(screen.queryByLabelText(message('catalog.kindLabel'))).not.toBeInTheDocument();
  });

  it('Happy path: the surface Retry re-requests the detail exactly once', async () => {
    const fake = createFakeTransport(
      successOutcome(CATALOG),
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
      detailEnvelope('Dungeons & Dragons 5e'),
    );
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(fake.transport)) });

    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('common.retry') })).toBeInTheDocument(),
    );
    expect(fake.attempts).toBe(2);

    await user.click(screen.getByRole('button', { name: message('common.retry') }));

    await screen.findByTestId('detail-name');
    expect(fake.attempts).toBe(3);
  });

  it('Edge: reopening the same template issues exactly one further request and starts at loading', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(deferred.transport)) });

    deferred.resolve(successOutcome(CATALOG));
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    deferred.resolve(detailEnvelope('Dungeons & Dragons 5e'));
    await screen.findByTestId('detail-name');
    expect(deferred.attempts).toBe(2);

    await user.click(screen.getByRole('button', { name: message('sheet.close') }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: ENTRY_NAME }));

    expect(screen.getByText(message('sheet.loading'))).toBeInTheDocument();
    expect(deferred.attempts).toBe(3);

    deferred.resolve(detailEnvelope('Dungeons & Dragons 5e'));
    await screen.findByTestId('detail-name');
  });

  it("Happy path: opening two different entries loads each one's own detail", async () => {
    const fake = createFakeTransport(
      successOutcome(CATALOG),
      detailEnvelope('Dungeons & Dragons 5e', 'dnd5e'),
      detailEnvelope('Powered by the Apocalypse', 'pbta'),
    );
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(withPlanUsage(fake.transport)) });

    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    await screen.findByTestId('detail-name');
    expect(fake.lastRequest?.url).toContain('/templates/dnd5e');

    await user.click(screen.getByRole('button', { name: message('sheet.close') }));
    await user.click(screen.getByRole('button', { name: /Powered by the Apocalypse/ }));

    await waitFor(() =>
      expect(screen.getByTestId('detail-name')).toHaveTextContent('Powered by the Apocalypse'),
    );
    expect(fake.lastRequest?.url).toContain('/templates/pbta');
  });
});

describe('sheet-name field and validation', () => {
  it('Happy path: the field starts empty with a localized label and example placeholder', async () => {
    const user = userEvent.setup();
    await openSurface(user);

    const input = screen.getByLabelText(message('sheet.nameLabel'));
    expect(input).toHaveValue('');
    expect(input).toHaveAttribute('placeholder', message('sheet.namePlaceholder'));
  });

  it('Edge: the primary action is disabled for "" and "   " and exposes the required reason', async () => {
    const user = userEvent.setup();
    await openSurface(user);

    const input = screen.getByLabelText(message('sheet.nameLabel'));
    const create = screen.getByRole('button', { name: message('sheet.create') });

    expect(create).toHaveAttribute('aria-disabled', 'true');
    expect(create).toHaveAccessibleDescription(message('sheet.nameRequired'));
    expect(create).not.toHaveAttribute('title');

    await user.type(input, '   ');
    expect(create).toHaveAttribute('aria-disabled', 'true');
  });

  it('Edge: the primary action is enabled once the trimmed value is non-empty', async () => {
    const user = userEvent.setup();
    await openSurface(user);

    const input = screen.getByLabelText(message('sheet.nameLabel'));

    await user.type(input, '  Aragorn  ');
    expect(screen.getByRole('button', { name: message('sheet.create') })).not.toHaveAttribute(
      'aria-disabled',
      'true',
    );

    await user.clear(input);
    await user.type(input, 'Aragorn');
    expect(screen.getByRole('button', { name: message('sheet.create') })).not.toHaveAttribute(
      'aria-disabled',
      'true',
    );
  });

  it('Edge: typing a name issues no request by itself — creation is activated explicitly', async () => {
    const user = userEvent.setup();
    const fake = await openSurface(user);

    const input = screen.getByLabelText(message('sheet.nameLabel'));
    const before = fake.attempts;

    await user.type(input, 'Aragorn');

    expect(fake.attempts).toBe(before);
  });

  it('Happy path: on a paid plan the Generate with AI entry is a link carrying the template and the chosen kind', async () => {
    const user = userEvent.setup();
    await openSurface(user, PAID_PLAN_USAGE);

    const entry = screen.getByRole('link', { name: message('generate.entry') });
    expect(entry).toHaveAttribute('href', '/generate/dnd5e?kind=character');
    expect(entry).toHaveClass('lg-button', 'lg-button--secondary');
    expect(screen.queryByText(message('generate.upgrade.badge'))).toBeNull();

    await user.selectOptions(screen.getByLabelText(message('catalog.kindLabel')), 'npc');
    expect(screen.getByRole('link', { name: message('generate.entry') })).toHaveAttribute(
      'href',
      '/generate/dnd5e?kind=npc',
    );
  });
});
