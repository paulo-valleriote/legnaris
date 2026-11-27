import { fireEvent, screen } from '@testing-library/react';
import { message } from '../l10n/strings';

export async function startEditing(): Promise<void> {
  fireEvent.click(await screen.findByRole('button', { name: message('sheet.edit') }));
}
