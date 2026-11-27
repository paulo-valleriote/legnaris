import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import {
  CAMPAIGN_ID,
  SHEET_ID,
  TEMPLATE_ID,
  baseRoutes,
  capturedSheet,
  capturedTemplates,
  clientFor,
  ok,
  scripted,
} from './campaign_helpers';
import { fantasySheetPayload, fantasyTemplatePayload } from './fantasy_fixtures';
import { fifthCharacterSheetPayload, fifthCharacterTemplatePayload } from './fifth_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import {
  GENERATION_ID,
  VALID_CONCEPT,
  conceptField,
  generateButton,
  planWith,
  renderGenerate,
} from './generate_fixtures';
import { renderApp } from './render';
import { sheetPayload, templateDetailPayload } from './sheet_fixtures';
import { GAME_SYSTEM_CATALOG_ENTRY } from './template_game_system_fixture';

const here = path.dirname(fileURLToPath(import.meta.url));
const BASE_URL = 'http://api.test';
const NOTICE_LABEL = 'SRD 5.2.1';
const NOTICE_PATH = '/srd';
const GAME_SYSTEM = GAME_SYSTEM_CATALOG_ENTRY.game_system;
const GM_PATH = `/campaigns/${CAMPAIGN_ID}/sheets/${SHEET_ID}`;
const NPC_PATH = `/campaigns/${CAMPAIGN_ID}/npcs/${SHEET_ID}`;

function withNotice(payload: Record<string, unknown>): Record<string, unknown> {
  return { ...payload, name: '5E Character', game_system: GAME_SYSTEM };
}

function templateLine(name: string): string {
  return `${message('gm.templatePrefix')}${name}`;
}

async function openSheet(
  template: Record<string, unknown>,
  sheet: Record<string, unknown> = sheetPayload(),
) {
  const fake = createFakeTransport(successOutcome(sheet), successOutcome(template));
  const user = userEvent.setup();
  renderApp(`/sheets/${String(sheet.id ?? SHEET_ID)}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function noticeLink() {
  return screen.getByRole('link', { name: NOTICE_LABEL });
}

describe('owner sheet template notice', () => {
  it('Happy path: a generic sheet draws the template name with its notice link', async () => {
    await openSheet(withNotice(fantasyTemplatePayload()), fantasySheetPayload());

    const line = screen.getByTestId('sheet-eyebrow');
    expect(line).toHaveTextContent(`5E Character · ${NOTICE_LABEL}`);
    const link = noticeLink();
    expect(link).toHaveAttribute('href', NOTICE_PATH);
    expect(link.tagName).toBe('A');
    link.focus();
    expect(link).toHaveFocus();
  });

  it('Happy path: a preset sheet keeps the template name and its notice link', async () => {
    await openSheet(withNotice(fifthCharacterTemplatePayload()), fifthCharacterSheetPayload());

    expect(screen.getByTestId('sheet-eyebrow')).toHaveTextContent(`5E Character · ${NOTICE_LABEL}`);
    expect(noticeLink()).toHaveAttribute('href', NOTICE_PATH);
  });

  it('Happy path: a template without a Game system draws the current name and no link', async () => {
    await openSheet(templateDetailPayload({ name: 'Fantasy Template' }));

    expect(screen.getByTestId('sheet-eyebrow')).toHaveTextContent('Fantasy Template');
    expect(screen.queryByRole('link', { name: NOTICE_LABEL })).not.toBeInTheDocument();
  });

  it('Edge: the fantasy_v1 generic view draws its name and no notice link', async () => {
    await openSheet(fantasyTemplatePayload(), fantasySheetPayload());

    expect(screen.getByTestId('sheet-eyebrow')).toHaveTextContent('Fantasy Template');
    expect(screen.queryByRole('link', { name: NOTICE_LABEL })).not.toBeInTheDocument();
  });

  it('Edge: leaving the Sheet through the notice with unsaved work keeps the guard', async () => {
    const { user } = await openSheet(withNotice(templateDetailPayload()));

    const level = screen.getByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    await user.click(noticeLink());

    const dialog = await screen.findByRole('dialog', { name: message('sheet.leaveTitle') });
    await user.click(within(dialog).getByRole('button', { name: message('sheet.leaveConfirm') }));

    await waitFor(() => expect(screen.getByTestId('location-pathname')).toHaveTextContent('/srd'));
  });

  it('Edge: cancelling the guard keeps the Sheet open', async () => {
    const { user } = await openSheet(withNotice(templateDetailPayload()));

    const level = screen.getByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    await user.click(noticeLink());

    const dialog = await screen.findByRole('dialog', { name: message('sheet.leaveTitle') });
    await user.click(within(dialog).getByRole('button', { name: message('sheet.cancel') }));

    expect(screen.getByTestId('sheet-name')).toHaveTextContent('Aragorn');
    expect(screen.queryByTestId('not-found')).not.toBeInTheDocument();
  });
});

describe('campaign read-only template notice', () => {
  function sheetRoutes(path: string, notice: boolean) {
    const template = notice
      ? withNotice(capturedTemplates()[0] as Record<string, unknown>)
      : (capturedTemplates()[0] as Record<string, unknown>);
    return scripted([
      ...baseRoutes(),
      { method: 'GET', path: `/templates/${TEMPLATE_ID}`, respond: () => ok(template) },
      { method: 'GET', path, respond: () => campaignSheetResponse() },
    ]);
  }

  it('Happy path: the GM view of a Player Sheet links the notice beside the template name', async () => {
    const script = sheetRoutes(GM_PATH, true);
    renderApp(GM_PATH, { client: clientFor(script.transport) });

    expect(await screen.findByRole('link', { name: NOTICE_LABEL })).toHaveAttribute(
      'href',
      NOTICE_PATH,
    );
    const meta = noticeLink().closest('.lg-page-header__meta');
    expect(meta).toHaveTextContent(templateLine('5E Character'));
    expect(meta).toHaveTextContent(NOTICE_LABEL);
  });

  it('Happy path: the GM view without a notice keeps the current template line and no link', async () => {
    const script = sheetRoutes(GM_PATH, false);
    renderApp(GM_PATH, { client: clientFor(script.transport) });

    expect(await screen.findByText(templateLine('Fantasy Template'))).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: NOTICE_LABEL })).not.toBeInTheDocument();
  });

  it('Happy path: the NPC view a GM opens links the notice', async () => {
    const script = sheetRoutes(NPC_PATH, true);
    renderApp(NPC_PATH, { client: clientFor(script.transport) });

    expect(await screen.findByRole('link', { name: NOTICE_LABEL })).toHaveAttribute(
      'href',
      NOTICE_PATH,
    );
    expect(screen.getByRole('heading', { name: 'Brenna', level: 1 })).toBeInTheDocument();
  });

  it('Happy path: the NPC view revealed to Players links the notice', async () => {
    const script = sheetRoutes(NPC_PATH, true);
    renderApp(NPC_PATH, { client: clientFor(script.transport) });

    expect(await screen.findByRole('link', { name: NOTICE_LABEL })).toHaveAttribute(
      'href',
      NOTICE_PATH,
    );
  });

  it('Happy path: the NPC view revealed to Players without a notice links nothing', async () => {
    const script = sheetRoutes(NPC_PATH, false);
    renderApp(NPC_PATH, { client: clientFor(script.transport) });

    expect(await screen.findByText(templateLine('Fantasy Template'))).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: NOTICE_LABEL })).not.toBeInTheDocument();
  });
});

function campaignSheetResponse() {
  return {
    status: 200,
    body: JSON.stringify({ data: { ...capturedSheet(), name: 'Brenna' } }),
    etag: '"1.gm"',
  };
}

describe('generation review template notice', () => {
  const SUCCEEDED = {
    id: GENERATION_ID,
    status: 'succeeded',
    kind: 'character',
    template_id: 'plain_v1',
    values: { identity: { characterName: 'Aragorn', level: 3 } },
    suggested_name: 'Aragorn',
    created_at: '2026-11-14T10:00:00Z',
  };

  async function reachReview(template: Record<string, unknown>) {
    const user = userEvent.setup();
    renderGenerate({
      'GET /templates/plain_v1': ok(template),
      'POST /generations': ok({ id: GENERATION_ID }, 202),
      [`GET /generations/${GENERATION_ID}`]: ok(SUCCEEDED),
      'GET /me/plan': ok(planWith()),
    });
    await screen.findByLabelText('Describe your character');
    await user.type(conceptField(), VALID_CONCEPT);
    await user.click(generateButton());
    return screen.findByRole('heading', { name: 'Review your character' });
  }

  it('Happy path: the review links the template notice beside the template name', async () => {
    await reachReview(withNotice(templateDetailPayload()));

    const link = noticeLink();
    expect(link).toHaveAttribute('href', NOTICE_PATH);
    const review = screen.getByRole('region', { name: 'Review your character' });
    expect(review).toHaveTextContent(`5E Character · ${NOTICE_LABEL}`);
  });

  it('Happy path: a review without a notice keeps the current copy and links nothing', async () => {
    await reachReview(templateDetailPayload());

    expect(screen.queryByRole('link', { name: NOTICE_LABEL })).not.toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Review your character' })).not.toHaveTextContent(
      'Fantasy Template',
    );
  });
});

describe('template notice layout', () => {
  const uiCss = readFileSync(path.resolve(here, '../catalog/template_name.css'), 'utf8');

  function rule(selector: string): string {
    const start = uiCss.indexOf(`${selector} {`);
    expect(start).toBeGreaterThanOrEqual(0);
    return uiCss.slice(start, uiCss.indexOf('}', start));
  }

  it('Edge: the notice link wraps anywhere instead of overflowing a 390px viewport', () => {
    const link = rule('.lg-template-name__notice');
    expect(link).toMatch(/overflow-wrap:\s*anywhere/);
    expect(link).not.toMatch(/white-space:\s*nowrap/);
    expect(link).not.toMatch(/\bwidth:/);
    expect(link).toMatch(/color:\s*var\(--lg-color-link\)/);
  });
});
