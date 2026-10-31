import { describe, expect, it } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { failureMessage } from '../api/error_mapper';
import type { Transport } from '../api/transport';
import { message, messages } from '../l10n/strings';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { renderApp } from './render';
import {
  SHEET_ID,
  emptyTemplateDetailPayload,
  sheetPayload,
  templateDetailPayload,
} from './sheet_fixtures';

const BASE_URL = 'http://api.test';

const SHEET_URL = `/sheets/${SHEET_ID}`;

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

describe('sheet screen states (WSC-08, WSC-10, SC-001)', () => {
  it('Happy path: the loaded screen shows the sheet name and renders the template field tree through the engine (WSC-08)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    renderApp(SHEET_URL, { client: clientWith(fake.transport) });

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
    expect(screen.getByTestId('sheet-eyebrow')).toHaveTextContent('Fantasy');
    expect(screen.getByRole('heading', { name: 'Identity' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('Aragorn');
    expect(screen.getByRole('textbox', { name: 'Level' })).toHaveValue('3');
    expect(screen.getByRole('group', { name: 'Total' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Ability' })).toBeInTheDocument();
  });

  it('Edge: the open-loading and template-loading lines resolve through the catalog copy (WSC-05, WSC-09)', async () => {
    const deferred = createDeferredTransport();
    renderApp(SHEET_URL, { client: clientWith(deferred.transport) });

    expect(screen.getByText(message('sheet.openLoading'))).toBeInTheDocument();
    expect(screen.getByText(messages['sheet.openLoading'])).toBeInTheDocument();

    deferred.resolve(successOutcome(sheetPayload()));

    expect(await screen.findByText(message('sheet.templateLoading'))).toBeInTheDocument();
    expect(screen.getByText(messages['sheet.templateLoading'])).toBeInTheDocument();

    deferred.resolve(successOutcome(templateDetailPayload()));
    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
  });

  it('Edge: a template with no fields shows the empty-fields line instead of an empty form (WSC-08)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(emptyTemplateDetailPayload()),
    );
    renderApp(SHEET_URL, { client: clientWith(fake.transport) });

    expect(await screen.findByTestId('sheet-empty-fields')).toHaveTextContent(
      messages['sheet.emptyFields'],
    );
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('Happy path: a direct navigation renders the same sheet screen (WSC-10)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    renderApp(SHEET_URL, { client: clientWith(fake.transport) });

    expect(await screen.findByTestId('sheet-screen')).toBeInTheDocument();
    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
  });
});

describe('sheet footer, save and Ctrl+S (WSC-12, WSC-14, WSC-16, WSC-17, WSC-19, WSC-20)', () => {
  async function openLoadedScreen(transport: Transport) {
    const user = userEvent.setup();
    renderApp(SHEET_URL, { client: clientWith(transport) });
    const level = await screen.findByRole('textbox', { name: 'Level' });
    return { user, level };
  }

  it('Edge: Save is disabled while the sheet is clean and no pending indicator shows (WSC-19, WSC-12)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    await openLoadedScreen(fake.transport);

    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(screen.queryByText(message('sheet.pendingEdits'))).not.toBeInTheDocument();
  });

  it('Happy path: editing shows the pending indicator and Save sends the delta, showing saving then saved (WSC-12, WSC-14, WSC-16)', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp(SHEET_URL, { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome(sheetPayload()));
    expect(await screen.findByText(message('sheet.templateLoading'))).toBeInTheDocument();
    deferred.resolve(successOutcome(templateDetailPayload()));
    const level = await screen.findByRole('textbox', { name: 'Level' });

    const save = screen.getByRole('button', { name: message('sheet.save') });
    expect(save).toBeDisabled();

    await user.clear(level);
    await user.type(level, '4');

    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();

    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    expect(screen.getByRole('button', { name: message('sheet.saving') })).toBeDisabled();

    deferred.resolve(successOutcome({ derived: { total: 5 } }));

    expect(await screen.findByRole('button', { name: message('sheet.saved') })).toBeInTheDocument();
    expect(screen.queryByText(message('sheet.pendingEdits'))).not.toBeInTheDocument();
  });

  it('Edge: a failed save renders the mapped message inline and keeps the edits (WSC-17)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
    );
    const { user, level } = await openLoadedScreen(fake.transport);

    await user.clear(level);
    await user.type(level, '4');
    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    expect(await screen.findByTestId('sheet-save-error')).toHaveTextContent(
      failureMessage('serverError'),
    );
    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();
  });

  it('Happy path: Ctrl+S runs the same save as the footer action (WSC-20)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      successOutcome({}),
    );
    const { user, level } = await openLoadedScreen(fake.transport);

    await user.clear(level);
    await user.type(level, '4');

    await user.keyboard('{Control>}s{/Control}');

    await waitFor(() => {
      expect(fake.requests.filter((request) => request.method === 'PATCH')).toHaveLength(1);
    });
    expect(screen.queryByText(message('sheet.pendingEdits'))).not.toBeInTheDocument();
  });
});

const here = path.dirname(fileURLToPath(import.meta.url));

/** The feature's source directories whose files must own no DOM-injection sink (WSC-35). */
function featureSources(): string[] {
  return ['../sheet', '../library'].flatMap((relative) => {
    const dir = path.resolve(here, relative);
    return readdirSync(dir)
      .filter((name) => name.endsWith('.ts') || name.endsWith('.tsx'))
      .map((name) => path.join(dir, name));
  });
}

describe('markup safety (WSC-35, SC-007)', () => {
  it('Edge: a sheet name of "<script>" renders as literal text and produces no script node (WSC-35)', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload({ name: '<script>alert(1)</script>' })),
      successOutcome(templateDetailPayload()),
    );
    renderApp(SHEET_URL, { client: clientWith(fake.transport) });

    const name = await screen.findByTestId('sheet-name');

    expect(name).toHaveTextContent('<script>alert(1)</script>');
    expect(document.querySelectorAll('script')).toHaveLength(0);
  });

  it('Edge: a field value of "<script>" renders as literal text and produces no script node (WSC-35)', async () => {
    const fake = createFakeTransport(
      successOutcome(
        sheetPayload({
          data: {
            identity: { characterName: '<script>x</script>', level: 3 },
            derived: { total: 4 },
          },
        }),
      ),
      successOutcome(templateDetailPayload()),
    );
    renderApp(SHEET_URL, { client: clientWith(fake.transport) });

    const name = await screen.findByRole('textbox', { name: 'Name' });

    expect(name).toHaveValue('<script>x</script>');
    expect(document.querySelectorAll('script')).toHaveLength(0);
  });

  it('Edge: the sheet-cycle sources own no DOM-injection sink (WSC-35)', () => {
    const sources = featureSources();
    expect(sources.length).toBeGreaterThan(0);

    for (const file of sources) {
      const text = readFileSync(file, 'utf8');
      expect(text).not.toMatch(
        /dangerouslySetInnerHTML|\binnerHTML\b|\beval\(|new Function|document\.write/,
      );
    }
  });
});
