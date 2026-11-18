import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { Transport, HttpRequest, HttpResponse } from '../api/transport';
import { message, messageWith } from '../l10n/strings';
import { errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';
const PLAN_PATH = '/me/plan';
const CONSENT_PATH = '/me/llm-consent';

const NO_CONSENT_PLAN = {
  plan: 'free',
  sheets: { character: { used: 0, limit: 1 } },
  llm_consent_at: null,
  llm_consent_version: null,
};

const CONSENTED_PLAN = {
  ...NO_CONSENT_PLAN,
  llm_consent_at: '2026-11-01T12:00:00Z',
  llm_consent_version: '2',
};

const NO_CONTENT: HttpResponse = { status: 204, body: '' };

function scriptedTransport(options: {
  plans: Array<HttpResponse | Promise<HttpResponse>>;
  revoke?: () => Promise<HttpResponse>;
}) {
  const requests: HttpRequest[] = [];
  let planReads = 0;

  const transport: Transport = (request) => {
    requests.push(request);
    if (request.url.endsWith(PLAN_PATH)) {
      const outcome = options.plans[Math.min(planReads, options.plans.length - 1)];
      planReads += 1;
      return Promise.resolve(outcome);
    }
    if (request.url.endsWith(CONSENT_PATH) && request.method === 'DELETE') {
      return (options.revoke ?? (() => Promise.resolve(NO_CONTENT)))();
    }
    return Promise.reject(new Error(`unexpected request ${request.method} ${request.url}`));
  };

  return {
    transport,
    requests,
    revokes: () => requests.filter((request) => request.method === 'DELETE'),
    planReads: () => planReads,
  };
}

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function deferredRevoke() {
  let settle: (response: HttpResponse) => void = () => undefined;
  const pending = new Promise<HttpResponse>((resolve) => {
    settle = resolve;
  });
  return { responder: () => pending, settle: (response: HttpResponse) => settle(response) };
}

function scrim(): HTMLElement {
  const dialog = screen.getByRole('dialog');
  const overlay = dialog.parentElement;
  if (overlay === null) {
    throw new Error('the dialog is not mounted');
  }
  return overlay;
}

describe('account screen plan and consent (LGN-195)', () => {
  it('Happy path: shows the plan and the recorded consent with its date and version', async () => {
    const scripted = scriptedTransport({ plans: [successOutcome(CONSENTED_PLAN)] });
    renderApp('/account', { client: clientWith(scripted.transport) });

    expect(await screen.findByText(message('account.planFree'))).toBeInTheDocument();
    const consent = screen.getByTestId('account-consent');
    expect(consent).toHaveTextContent(/2026/);
    expect(consent).toHaveTextContent(messageWith('account.consentVersion', { version: '2' }));
    expect(screen.getByRole('button', { name: message('account.revoke') })).toBeEnabled();
  });

  it('Edge: with no consent recorded the revoke action is hidden and the state says so', async () => {
    const scripted = scriptedTransport({ plans: [successOutcome(NO_CONSENT_PLAN)] });
    renderApp('/account', { client: clientWith(scripted.transport) });

    expect(await screen.findByTestId('account-consent-none')).toHaveTextContent(
      message('account.consentNone'),
    );
    expect(screen.queryByRole('button', { name: message('account.revoke') })).toBeNull();
    expect(scripted.revokes()).toHaveLength(0);
  });

  it('Edge: while the plan is loading the screen shows the loading line and no consent state', () => {
    const scripted = scriptedTransport({
      plans: [new Promise<HttpResponse>(() => undefined)],
    });
    renderApp('/account', { client: clientWith(scripted.transport) });

    expect(screen.getByText(message('account.loading'))).toBeInTheDocument();
    expect(screen.queryByTestId('account-consent')).toBeNull();
  });

  it('Error: a failed plan read shows the shared error state and retries with one request', async () => {
    const scripted = scriptedTransport({
      plans: [errorOutcome('INTERNAL_ERROR', 'boom', 500), successOutcome(CONSENTED_PLAN)],
    });
    const user = userEvent.setup();
    renderApp('/account', { client: clientWith(scripted.transport) });

    expect(await screen.findByRole('alert')).toHaveTextContent(message('plan.loadFailed'));
    await user.click(screen.getByRole('button', { name: message('common.retry') }));

    expect(await screen.findByTestId('account-consent')).toBeInTheDocument();
    expect(scripted.planReads()).toBe(2);
  });
});

describe('account sheet usage (LGN-234)', () => {
  it('Happy path: the plan row shows the sheet usage the API returned', async () => {
    const scripted = scriptedTransport({ plans: [successOutcome(NO_CONSENT_PLAN)] });
    renderApp('/account', { client: clientWith(scripted.transport) });

    const usage = await screen.findByTestId('account-usage');
    expect(within(usage).getByText(message('account.usageCharacter'))).toBeInTheDocument();
    expect(
      within(usage).getByText(messageWith('account.usageCount', { used: 0, limit: 1 })),
    ).toHaveClass('lg-account__usage-count');
    expect(within(usage).getByText('0 of 1')).toBeInTheDocument();
  });

  it('Happy path: the kinds render in SHEET_KINDS order whatever the JSON key order', async () => {
    const scripted = scriptedTransport({
      plans: [
        successOutcome({
          ...NO_CONSENT_PLAN,
          sheets: {
            enemy: { used: 1, limit: 2 },
            character: { used: 0, limit: 1 },
            npc: { used: 2, limit: 2 },
          },
        }),
      ],
    });
    renderApp('/account', { client: clientWith(scripted.transport) });

    const usage = await screen.findByTestId('account-usage');
    const labels = Array.from(usage.querySelectorAll('dt')).map((node) => node.textContent);
    expect(labels).toEqual([
      message('account.usageCharacter'),
      message('account.usageNpc'),
      message('account.usageEnemy'),
    ]);
    const counts = Array.from(usage.querySelectorAll('dd')).map((node) => node.textContent);
    expect(counts).toEqual([
      messageWith('account.usageCount', { used: 0, limit: 1 }),
      `${messageWith('account.usageCount', { used: 2, limit: 2 })} ${message('account.quotaFull')}`,
      messageWith('account.usageCount', { used: 1, limit: 2 }),
    ]);
  });

  it('Edge: an empty sheets object renders no usage list', async () => {
    const scripted = scriptedTransport({
      plans: [successOutcome({ ...NO_CONSENT_PLAN, sheets: {} })],
    });
    renderApp('/account', { client: clientWith(scripted.transport) });

    expect(await screen.findByTestId('account-plan')).toBeInTheDocument();
    expect(screen.queryByTestId('account-usage')).toBeNull();
  });

  it('Edge: usage at the limit carries the full modifier and a text cue in the warning tone', async () => {
    const scripted = scriptedTransport({
      plans: [
        successOutcome({
          ...NO_CONSENT_PLAN,
          sheets: { character: { used: 1, limit: 1 } },
        }),
      ],
    });
    renderApp('/account', { client: clientWith(scripted.transport) });

    const usage = await screen.findByTestId('account-usage');
    expect(
      within(usage).getByText(messageWith('account.usageCount', { used: 1, limit: 1 })),
    ).toHaveClass('lg-account__usage-count--full');
    expect(within(usage).getByText(message('account.quotaFull'))).toHaveClass(
      'lg-account__quota-full',
    );
  });

  it('Edge: a large count renders verbatim', async () => {
    const scripted = scriptedTransport({
      plans: [
        successOutcome({
          ...NO_CONSENT_PLAN,
          sheets: { enemy: { used: 120, limit: 500 } },
        }),
      ],
    });
    renderApp('/account', { client: clientWith(scripted.transport) });

    const usage = await screen.findByTestId('account-usage');
    expect(
      within(usage).getByText(messageWith('account.usageCount', { used: 120, limit: 500 })),
    ).toHaveTextContent('120 of 500');
  });
});

describe('revoke LLM consent (LGN-195)', () => {
  it('Happy path: confirming revokes once, then the state updates to no consent', async () => {
    const scripted = scriptedTransport({
      plans: [successOutcome(CONSENTED_PLAN), successOutcome(NO_CONSENT_PLAN)],
    });
    const user = userEvent.setup();
    renderApp('/account', { client: clientWith(scripted.transport) });

    await user.click(await screen.findByRole('button', { name: message('account.revoke') }));
    const dialog = screen.getByRole('dialog', { name: message('account.revokeTitle') });
    expect(dialog).toHaveTextContent(message('account.revokeMessage'));
    await user.click(
      within(dialog).getByRole('button', { name: message('account.revokeConfirm') }),
    );

    expect(await screen.findByTestId('account-consent-none')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent(message('account.revoked'));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(scripted.revokes()).toHaveLength(1);
    expect(scripted.planReads()).toBe(2);
  });

  it('Edge: the safe action carries focus, so Enter does not destroy consent', async () => {
    const scripted = scriptedTransport({ plans: [successOutcome(CONSENTED_PLAN)] });
    const user = userEvent.setup();
    renderApp('/account', { client: clientWith(scripted.transport) });

    await user.click(await screen.findByRole('button', { name: message('account.revoke') }));

    const dialog = screen.getByRole('dialog', { name: message('account.revokeTitle') });
    expect(
      within(dialog).getByRole('button', { name: message('account.revokeKeep') }),
    ).toHaveFocus();
  });

  it('Edge: closing the dialog returns focus to the action that opened it', async () => {
    const scripted = scriptedTransport({ plans: [successOutcome(CONSENTED_PLAN)] });
    const user = userEvent.setup();
    renderApp('/account', { client: clientWith(scripted.transport) });

    const opener = await screen.findByRole('button', { name: message('account.revoke') });
    await user.click(opener);
    await user.click(screen.getByRole('button', { name: message('account.revokeKeep') }));

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(opener).toHaveFocus();
  });

  it.each([
    {
      name: 'Escape',
      act: (user: ReturnType<typeof userEvent.setup>) => user.keyboard('{Escape}'),
    },
    {
      name: 'the Keep consent action',
      act: (user: ReturnType<typeof userEvent.setup>) =>
        user.click(screen.getByRole('button', { name: message('account.revokeKeep') })),
    },
    {
      name: 'a click on the scrim',
      act: (user: ReturnType<typeof userEvent.setup>) => user.click(scrim()),
    },
  ])(
    'Edge: cancelling with $name closes the dialog without any request',
    async ({ act: cancel }) => {
      const scripted = scriptedTransport({ plans: [successOutcome(CONSENTED_PLAN)] });
      const user = userEvent.setup();
      renderApp('/account', { client: clientWith(scripted.transport) });

      await user.click(await screen.findByRole('button', { name: message('account.revoke') }));
      expect(screen.getByRole('dialog')).toBeInTheDocument();
      await cancel(user);

      expect(screen.queryByRole('dialog')).toBeNull();
      expect(scripted.revokes()).toHaveLength(0);
      expect(screen.getByTestId('account-consent')).toBeInTheDocument();
    },
  );

  it('Edge: a double submit while the revoke is in flight sends exactly one DELETE', async () => {
    const deferred = deferredRevoke();
    const scripted = scriptedTransport({
      plans: [successOutcome(CONSENTED_PLAN), successOutcome(NO_CONSENT_PLAN)],
      revoke: deferred.responder,
    });
    const user = userEvent.setup();
    renderApp('/account', { client: clientWith(scripted.transport) });

    await user.click(await screen.findByRole('button', { name: message('account.revoke') }));
    const dialog = screen.getByRole('dialog', { name: message('account.revokeTitle') });
    const confirm = within(dialog).getByRole('button', { name: message('account.revokeConfirm') });
    await user.click(confirm);
    await user.click(confirm);
    await user.keyboard('{Enter}');

    expect(scripted.revokes()).toHaveLength(1);
    expect(confirm).toBeDisabled();

    await act(async () => {
      deferred.settle(NO_CONTENT);
    });
    expect(await screen.findByTestId('account-consent-none')).toBeInTheDocument();
    expect(scripted.revokes()).toHaveLength(1);
  });

  it('Error: a failed revoke keeps the dialog open with the mapped copy and sends no repeat', async () => {
    const scripted = scriptedTransport({
      plans: [successOutcome(CONSENTED_PLAN)],
      revoke: () => Promise.resolve(errorOutcome('INTERNAL_ERROR', 'boom', 500)),
    });
    const user = userEvent.setup();
    renderApp('/account', { client: clientWith(scripted.transport) });

    await user.click(await screen.findByRole('button', { name: message('account.revoke') }));
    const dialog = screen.getByRole('dialog', { name: message('account.revokeTitle') });
    await user.click(
      within(dialog).getByRole('button', { name: message('account.revokeConfirm') }),
    );

    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      message('api.error.serverError'),
    );
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.queryByRole('status')).toBeNull();
    expect(scripted.revokes()).toHaveLength(1);
    expect(scripted.planReads()).toBe(1);
  });
});

describe('account settings rows (DESIGN.md)', () => {
  it('Happy path: the page title is the shared page header', async () => {
    const scripted = scriptedTransport({ plans: [successOutcome(NO_CONSENT_PLAN)] });
    renderApp('/account', { client: clientWith(scripted.transport) });

    expect(
      await screen.findByRole('heading', { level: 1, name: message('account.title') }),
    ).toBeInTheDocument();
  });

  it('Edge: a settings row label is a term, not a page section heading', async () => {
    const scripted = scriptedTransport({ plans: [successOutcome(NO_CONSENT_PLAN)] });
    renderApp('/account', { client: clientWith(scripted.transport) });

    expect(await screen.findByTestId('account-plan')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: message('account.planHeading') })).toBeNull();
    expect(screen.getByText(message('account.planHeading')).tagName).toBe('DT');
  });
});

describe('account screen shared layer (UIP-01)', () => {
  it('Edge: the screen draws its dialog, title and styles from its own layer', () => {
    const source = readFileSync(
      path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../account/account_screen.tsx'),
      'utf8',
    );

    expect(source).not.toMatch(/catalog\/catalog\.css/);
    expect(source).not.toMatch(/lg-overlay/);
    expect(source).toContain("from '../ui/dialog'");
    expect(source).toContain("from '../ui/page_header'");
    expect(source).toContain("from '../ui/state'");
  });
});
