import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messages } from '../l10n/strings';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const here = path.dirname(fileURLToPath(import.meta.url));
const BASE_URL = 'http://api.test';
const SHEET_URL = `/sheets/${SHEET_ID}`;

async function openSheet(templateOverrides: Record<string, unknown> = {}) {
  const fake = createFakeTransport(
    successOutcome(sheetPayload()),
    successOutcome(templateDetailPayload(templateOverrides)),
  );
  renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
}

describe('sheet page template name', () => {
  it('Happy path: the template display name is shown with the sheet name', async () => {
    await openSheet({ name: 'Fantasy Template' });

    expect(screen.getByTestId('sheet-eyebrow')).toHaveTextContent('Fantasy Template');
    expect(screen.getByTestId('sheet-name')).toHaveTextContent('Aragorn');
  });

  it.each([
    ['Edge: an empty template name', ''],
    ['Edge: a whitespace-only template name', '   \n\t '],
  ])('%s falls back to the generic "Sheet" copy', async (_name, name) => {
    await openSheet({ name });

    expect(screen.getByTestId('sheet-eyebrow')).toHaveTextContent(
      new RegExp(`^${message('sheet.headerTitle')}$`),
    );
  });

  it('Edge: a very long template name is shown in full and stays out of the headings', async () => {
    const long = 'The Extraordinarily Long Role-Playing Template Name '.repeat(8).trim();
    await openSheet({ name: long });

    expect(screen.getByTestId('sheet-eyebrow')).toHaveTextContent(long);
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Aragorn');
  });

  it('Edge: a name carrying markup is rendered as literal text', async () => {
    await openSheet({ name: '<img src=x onerror=alert(1)> Template' });

    const eyebrow = screen.getByTestId('sheet-eyebrow');
    expect(eyebrow).toHaveTextContent('<img src=x onerror=alert(1)> Template');
    expect(eyebrow.querySelector('img')).toBeNull();
  });

  it('Edge: the sheet landmark keeps the generic accessible name, not the template name', async () => {
    await openSheet({ name: 'Fantasy Template' });

    expect(screen.getByTestId('sheet-screen')).toHaveAttribute(
      'aria-label',
      message('sheet.headerTitle'),
    );
  });
});

describe('sheet footer and Arrange hierarchy', () => {
  it('Happy path: Save is the one primary action and Preview its secondary, at the same size', async () => {
    await openSheet();

    const save = screen.getByRole('button', { name: message('sheet.save') });
    const preview = screen.getByRole('button', { name: message('sheet.preview') });
    expect(save).toHaveClass('lg-button--primary', 'lg-button--md');
    expect(preview).toHaveClass('lg-button--secondary', 'lg-button--md');
    expect(document.querySelectorAll('.lg-sheet__footer .lg-button--primary')).toHaveLength(1);
  });

  it('Edge: Arrange stays a small ghost entry in the heading group, below the footer actions in weight', async () => {
    await openSheet();

    const arrange = screen.getByRole('button', { name: message('arrange.start') });
    expect(arrange).toHaveClass('lg-button--ghost', 'lg-button--sm');
    expect(arrange.closest('.lg-sheet__header-actions')).not.toBeNull();
    expect(arrange.closest('.lg-sheet__footer')).toBeNull();
  });

  it('Happy path: the heading actions are one group: Help and Arrange, with Collapse all in the section strip', async () => {
    await openSheet();

    const group = document.querySelector('.lg-sheet__header-actions') as HTMLElement;
    expect(
      within(group)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual([message('sheet.help'), message('arrange.start')]);
    const strip = screen.getByTestId('section-strip');
    expect(
      within(strip).getByRole('button', { name: message('sheet.collapseAll') }),
    ).toBeInTheDocument();
    expect(group.closest('.lg-sheet__footer')).toBeNull();
  });

  it('Edge: the template name sits under the sheet name, never above it', async () => {
    await openSheet();

    const name = screen.getByTestId('sheet-name');
    const template = screen.getByTestId('sheet-eyebrow');
    expect(name.compareDocumentPosition(template) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('Happy path: the Arrange hint is short and still carries all three instructions', async () => {
    const user = userEvent.setup();
    await openSheet();
    await user.click(screen.getByRole('button', { name: message('arrange.start') }));

    const hint = screen.getByTestId('arrange-hint');
    expect(hint).toHaveTextContent(messages['arrange.hint']);
    expect(messages['arrange.hint'].length).toBeLessThanOrEqual(90);
    expect(messages['arrange.hint']).toMatch(/drag/i);
    expect(messages['arrange.hint']).toMatch(/star/i);
    expect(messages['arrange.hint']).toMatch(/done/i);
  });
});

describe('sheet stylesheet contract', () => {
  const engineCss = readFileSync(path.join(here, '..', 'engine', 'engine.css'), 'utf8');

  function rule(selector: string): string {
    const start = engineCss.indexOf(`${selector} {`);
    expect(start).toBeGreaterThanOrEqual(0);
    return engineCss.slice(start, engineCss.indexOf('}', start));
  }

  it('Edge: the preview mark leaves the label flow so a mark never changes a row height', () => {
    const mark = rule('.lg-row .lg-field-row__mark');
    expect(mark).toMatch(/position:\s*absolute/);
    expect(mark).toMatch(/white-space:\s*nowrap/);
    expect(mark).toMatch(/background-color:\s*var\(--lg-color-surface\)/);
    expect(rule('.lg-row')).toMatch(/position:\s*relative/);
  });

  it('Edge: the placeholder is token-coloured, fully opaque and passes through the muted role', () => {
    const placeholder = rule('.lg-field::placeholder');
    expect(placeholder).toMatch(/color:\s*var\(--lg-color-text-muted\)/);
    expect(placeholder).toMatch(/opacity:\s*1\b/);
  });
});
