import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { footerStatus, recalculatedCount, recalculatedSummary } from '../sheet/preview_summary';
import { firstRecalculatedRowKey, revealRow } from '../sheet/reveal_row';
import type { PreviewMarks } from '../sheet/sheet_edit';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, fieldTree, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const SHEET_URL = `/sheets/${SHEET_ID}`;

function derivedField(label: string): Record<string, unknown> {
  return {
    schema: { type: 'number', label, formula: 'level + 1', is_editable: false, default: 0 },
  };
}

function manyDerivedTemplate(): Record<string, unknown> {
  const fields = fieldTree() as Record<string, { children: Record<string, unknown> }>;
  fields.derived.children = {
    total: derivedField('Total'),
    bonus: derivedField('Bonus'),
    reach: derivedField('Reach'),
  };
  return templateDetailPayload({ fields });
}

function marks(modified: string[], recalculated: string[]): PreviewMarks {
  return { modified: new Set(modified), recalculated: new Set(recalculated) };
}

function liveRegions(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>('.lg-sheet__footer [aria-live]'));
}

async function openAndPreview(
  cascade: Record<string, unknown>,
  template = templateDetailPayload(),
) {
  const user = userEvent.setup();
  const fake = createFakeTransport(
    successOutcome(sheetPayload()),
    successOutcome(template),
    successOutcome(cascade),
  );
  renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  const level = await screen.findByRole('textbox', { name: 'Level' });
  await user.clear(level);
  await user.type(level, '4');
  await user.click(screen.getByRole('button', { name: message('sheet.preview') }));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled(),
  );
  return { user, fake, level };
}

describe('the preview summary copy', () => {
  it('Edge: no recalculated value reads as none', () => {
    expect(recalculatedSummary(0)).toBe('No values recalculated');
  });

  it('Edge: exactly one value is singular', () => {
    expect(recalculatedSummary(1)).toBe('1 value recalculated');
  });

  it('Happy path: several values are plural', () => {
    expect(recalculatedSummary(4)).toBe('4 values recalculated');
  });
});

describe('the footer status phrase', () => {
  const tests: { name: string; pending: boolean; count: number | undefined; want: string }[] = [
    {
      name: 'Edge: nothing pending and no preview says nothing',
      pending: false,
      count: undefined,
      want: '',
    },
    {
      name: 'Edge: pending without a preview is the plain unsaved phrase',
      pending: true,
      count: undefined,
      want: 'Unsaved changes',
    },
    {
      name: 'Edge: pending with a zero cascade says none recalculated',
      pending: true,
      count: 0,
      want: 'Unsaved, none recalculated',
    },
    {
      name: 'Edge: pending with one value is merged, not pluralised',
      pending: true,
      count: 1,
      want: 'Unsaved, 1 recalculated',
    },
    {
      name: 'Edge: a large count is carried whole',
      pending: true,
      count: 13,
      want: 'Unsaved, 13 recalculated',
    },
    {
      name: 'Edge: a preview that no longer has pending edits drops the unsaved word',
      pending: false,
      count: 4,
      want: '4 values recalculated',
    },
    {
      name: 'Edge: a clean zero cascade reads as none',
      pending: false,
      count: 0,
      want: 'No values recalculated',
    },
  ];

  for (const tc of tests) {
    it(tc.name, () => {
      expect(footerStatus(tc.pending, tc.count)).toBe(tc.want);
    });
  }
});

describe('the recalculated count', () => {
  it('Edge: an empty cascade counts zero', () => {
    expect(recalculatedCount(marks(['identity.level'], []))).toBe(0);
  });

  it('Edge: a path the person also edited is not counted as recalculated', () => {
    expect(recalculatedCount(marks(['a.x'], ['a.x', 'b.y']))).toBe(1);
  });

  it('Happy path: every cascade path counts once', () => {
    expect(recalculatedCount(marks(['a.x'], ['b.y', 'b.z', 'c.w']))).toBe(3);
  });
});

describe('revealRow', () => {
  let scrollIntoView: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView =
      scrollIntoView as unknown as typeof Element.prototype.scrollIntoView;
  });

  afterEach(() => {
    Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
  });

  function renderRows() {
    return render(
      <div>
        <div data-row-key="a">
          <span>a</span>
        </div>
        <div data-row-key="b">
          <span data-mark="recalculated">b</span>
        </div>
        <div data-row-key="c">
          <span data-mark="recalculated">c</span>
        </div>
      </div>,
    );
  }

  it('Edge: an unknown key returns false and moves nothing', () => {
    renderRows();

    expect(revealRow('missing')).toBe(false);
    expect(scrollIntoView).not.toHaveBeenCalled();
    expect(document.querySelector('[data-revealed]')).toBeNull();
  });

  it('Edge: the empty key returns false', () => {
    renderRows();

    expect(revealRow('')).toBe(false);
  });

  it('Edge: the first recalculated row is the first drawn, and none gives undefined', () => {
    const { unmount } = renderRows();
    expect(firstRecalculatedRowKey()).toBe('b');
    unmount();

    render(<div data-row-key="a" />);
    expect(firstRecalculatedRowKey()).toBeUndefined();
  });

  it('Happy path: scrolls the row to the centre, highlights it and focuses it', () => {
    renderRows();

    expect(revealRow('b')).toBe(true);

    const row = document.querySelector<HTMLElement>('[data-row-key="b"]');
    expect(scrollIntoView).toHaveBeenCalledTimes(1);
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'center', behavior: 'smooth' });
    expect(row).toHaveAttribute('data-revealed');
    expect(row).toHaveFocus();
  });

  it('Edge: reduced motion scrolls instantly', () => {
    renderRows();
    const original = window.matchMedia;
    window.matchMedia = ((media: string) => ({
      ...original(media),
      matches: media.includes('prefers-reduced-motion'),
    })) as typeof window.matchMedia;

    revealRow('b');

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'center', behavior: 'auto' });
    window.matchMedia = original;
  });

  it('Edge: revealing a second row moves the highlight and a blur clears it', async () => {
    const user = userEvent.setup();
    renderRows();

    revealRow('b');
    revealRow('c');

    expect(document.querySelector('[data-row-key="b"]')).not.toHaveAttribute('data-revealed');
    expect(document.querySelector('[data-row-key="c"]')).toHaveAttribute('data-revealed');

    await user.click(document.body);
    document.querySelector<HTMLElement>('[data-row-key="c"]')?.blur();

    expect(document.querySelector('[data-revealed]')).toBeNull();
    expect(document.querySelector('[data-row-key="c"]')).not.toHaveAttribute('tabindex');
  });

  it('Edge: a row that already has a tabindex keeps it after the blur', () => {
    const { container } = render(<div data-row-key="a" />);
    container.querySelector('[data-row-key]')?.setAttribute('tabindex', '0');

    revealRow('a');
    document.querySelector<HTMLElement>('[data-row-key="a"]')?.blur();

    expect(document.querySelector('[data-row-key="a"]')).toHaveAttribute('tabindex', '0');
  });

  it('Edge: revealing the same row twice scrolls twice and leaves one highlight', () => {
    renderRows();

    revealRow('b');
    revealRow('b');

    expect(scrollIntoView).toHaveBeenCalledTimes(2);
    expect(document.querySelectorAll('[data-revealed]')).toHaveLength(1);
  });
});

describe('the preview summary in the sheet footer', () => {
  let scrollIntoView: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView =
      scrollIntoView as unknown as typeof Element.prototype.scrollIntoView;
  });

  afterEach(() => {
    Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
  });

  it('Edge: before any preview the footer shows no summary and no jump control', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    await screen.findByRole('textbox', { name: 'Level' });

    expect(screen.queryByTestId('sheet-status')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: message('sheet.jumpToFirstName') })).toBeNull();
    expect(liveRegions()).toHaveLength(1);
    expect(liveRegions()[0]).toHaveTextContent('');
  });

  it('Edge: a cascade of zero values says so and offers no jump', async () => {
    await openAndPreview({});

    expect(screen.getByTestId('sheet-status')).toHaveTextContent('Unsaved, none recalculated');
    expect(screen.queryByRole('button', { name: message('sheet.jumpToFirstName') })).toBeNull();
    expect(liveRegions()[0]).toHaveTextContent('No values recalculated');
  });

  it('Edge: one recalculated value is singular, announced once by the single live region', async () => {
    await openAndPreview({ derived: { total: 5 } });

    expect(screen.getByTestId('sheet-status')).toHaveTextContent('Unsaved, 1 recalculated');
    expect(liveRegions()).toHaveLength(1);
    expect(liveRegions()[0]).toHaveAttribute('aria-live', 'polite');
    expect(liveRegions()[0]).toHaveTextContent('1 value recalculated');
    expect(screen.getByTestId('sheet-status')).toHaveAttribute('aria-hidden', 'true');
  });

  it('Happy path: several recalculated values are counted and plural', async () => {
    await openAndPreview({ derived: { total: 5, bonus: 6, reach: 7 } }, manyDerivedTemplate());

    expect(screen.getByTestId('sheet-status')).toHaveTextContent('Unsaved, 3 recalculated');
    expect(screen.getByRole('button', { name: message('sheet.jumpToFirstName') })).toBeEnabled();
  });

  it('Edge: an edit after the preview clears the summary and the announcement', async () => {
    const { user, level } = await openAndPreview({ derived: { total: 5 } });

    await user.clear(level);
    await user.type(level, '6');

    expect(screen.getByTestId('sheet-status')).toHaveTextContent(message('sheet.pendingEdits'));
    expect(screen.getByTestId('sheet-status')).not.toHaveAttribute('aria-hidden');
    expect(screen.queryByRole('button', { name: message('sheet.jumpToFirstName') })).toBeNull();
    expect(liveRegions()[0]).toHaveTextContent('');
  });

  it('Edge: a failed preview shows the error and no summary', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
    );
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));

    expect(await screen.findByTestId('sheet-preview-error')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-status')).toHaveTextContent(message('sheet.pendingEdits'));
    expect(document.querySelectorAll('.lg-sheet__status-line')).toHaveLength(1);
    expect(liveRegions()[0]).toHaveTextContent('');
  });

  it('Edge: Preview stays disabled after a preview, so a second press sends nothing', async () => {
    const { user, fake } = await openAndPreview({ derived: { total: 5 } });
    const sent = fake.requests.length;

    await user.click(screen.getByRole('button', { name: message('sheet.preview') }));

    expect(fake.requests).toHaveLength(sent);
    expect(screen.getByTestId('sheet-status')).toHaveTextContent('Unsaved, 1 recalculated');
  });

  it('Edge: the jump control lands on the first recalculated row in the order drawn', async () => {
    const { user } = await openAndPreview(
      { derived: { total: 5, bonus: 6, reach: 7 } },
      manyDerivedTemplate(),
    );

    await user.click(screen.getByRole('button', { name: message('sheet.jumpToFirstName') }));

    const row = document.querySelector<HTMLElement>('[data-row-key="derived.total"]');
    expect(scrollIntoView).toHaveBeenCalledTimes(1);
    expect(row).toHaveAttribute('data-revealed');
    expect(row).toHaveFocus();
  });

  it('Edge: a cascade path with no drawn row makes the jump a no-op', async () => {
    const { user } = await openAndPreview({ derived: { ghost: 1 } });

    expect(screen.getByTestId('sheet-status')).toHaveTextContent('Unsaved, 1 recalculated');
    await user.click(screen.getByRole('button', { name: message('sheet.jumpToFirstName') }));

    expect(scrollIntoView).not.toHaveBeenCalled();
    expect(document.querySelector('[data-revealed]')).toBeNull();
  });

  it('Happy path: the status is one phrase with Jump inline on the same line, and the pending line is gone', async () => {
    await openAndPreview({ derived: { total: 5, bonus: 6 } }, manyDerivedTemplate());

    const line = document.querySelector('.lg-sheet__status-line');
    expect(line).not.toBeNull();
    expect(line).toContainElement(screen.getByTestId('sheet-status'));
    expect(line).toContainElement(
      screen.getByRole('button', { name: message('sheet.jumpToFirstName') }),
    );
    expect(document.querySelectorAll('.lg-sheet__footer p:not([aria-live])')).toHaveLength(1);
    expect(screen.queryByText(message('sheet.pendingEdits'))).toBeNull();
  });

  it('Edge: the visible phrase is hidden from assistive technology so the cascade is read once', async () => {
    await openAndPreview({ derived: { total: 5 } });

    expect(screen.getByTestId('sheet-status')).toHaveAttribute('aria-hidden', 'true');
    expect(liveRegions()).toHaveLength(1);
    expect(liveRegions()[0]).toHaveTextContent(/^1 value recalculated$/);
  });

  it('Edge: Jump is outside the hidden phrase, so it stays reachable while the phrase is hidden', async () => {
    await openAndPreview({ derived: { total: 5 } });

    const jump = screen.getByRole('button', { name: message('sheet.jumpToFirstName') });
    expect(screen.getByTestId('sheet-status')).not.toContainElement(jump);
    expect(jump.closest('[aria-hidden="true"]')).toBeNull();
  });

  it('Edge: the Preview and Save actions stay two distinct buttons while the status shows', async () => {
    await openAndPreview({ derived: { total: 5 } });

    expect(screen.getByRole('button', { name: message('sheet.preview') })).toHaveClass(
      'lg-button--secondary',
    );
    expect(screen.getByRole('button', { name: message('sheet.save') })).toHaveClass(
      'lg-button--primary',
    );
    expect(screen.getByRole('button', { name: message('sheet.preview') })).toBeDisabled();
  });

  it('Edge: with a save error the alert sits in the status block beside the status line', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
    );
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    const level = await screen.findByRole('textbox', { name: 'Level' });
    await user.clear(level);
    await user.type(level, '4');
    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    const alert = await screen.findByTestId('sheet-save-error');
    expect(alert.closest('.lg-sheet__status')).toContainElement(screen.getByTestId('sheet-status'));
    expect(screen.getByTestId('sheet-status')).toHaveTextContent(message('sheet.pendingEdits'));
  });

  it('Edge: with nothing to say the status block is empty and takes no room', async () => {
    const fake = createFakeTransport(
      successOutcome(sheetPayload()),
      successOutcome(templateDetailPayload()),
    );
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    await screen.findByRole('textbox', { name: 'Level' });

    expect(document.querySelector('.lg-sheet__status')).toBeEmptyDOMElement();
  });

  it('Edge: the jump control keeps its visible text inside its accessible name', async () => {
    await openAndPreview({ derived: { total: 5 } });

    const jump = screen.getByRole('button', { name: message('sheet.jumpToFirstName') });

    expect(jump).toHaveTextContent(message('sheet.jumpToFirst'));
    expect(message('sheet.jumpToFirstName')).toContain(message('sheet.jumpToFirst'));
  });
});
