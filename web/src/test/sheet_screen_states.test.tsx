import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import { message, messages } from '../l10n/strings';
import { createDeferredTransport, createFakeTransport, successOutcome } from './fake_transport';
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
    expect(screen.getByText(message('sheet.headerTitle'))).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Identity' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('Aragorn');
    expect(screen.getByRole('textbox', { name: 'Level' })).toHaveValue('3');
    expect(screen.getByRole('status', { name: 'Total' })).toBeInTheDocument();
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
