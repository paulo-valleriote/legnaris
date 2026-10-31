import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

async function saveWith(failure: Parameters<typeof createFakeTransport>[0]) {
  const user = userEvent.setup();
  const fake = createFakeTransport(
    successOutcome(sheetPayload()),
    successOutcome(templateDetailPayload()),
    failure,
  );
  renderApp(`/sheets/${SHEET_ID}`, {
    client: createApiClient({ baseUrl: 'http://api.test', transport: fake.transport }),
  });
  const level = await screen.findByRole('textbox', { name: 'Level' });
  await user.clear(level);
  await user.type(level, '4');
  await user.click(screen.getByRole('button', { name: message('sheet.save') }));
  return screen.findByTestId('sheet-save-error');
}

describe('offline failure slot', () => {
  it('Edge: a server failure keeps the danger tone', async () => {
    const alert = await saveWith(errorOutcome('INTERNAL_ERROR', 'boom', 500));

    expect(alert).toHaveClass('lg-sheet__error');
    expect(alert).not.toHaveClass('lg-sheet__error--offline');
  });

  it('Happy path: an offline save says the work is kept and uses the muted tone', async () => {
    const alert = await saveWith({ failure: new Error('down') });

    expect(alert).toHaveTextContent(message('sheet.error.saveOffline'));
    expect(alert).toHaveClass('lg-sheet__error--offline');
    expect(alert).toHaveAttribute('role', 'alert');
  });
});
