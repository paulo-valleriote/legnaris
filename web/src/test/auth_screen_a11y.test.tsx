import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { createAuthApi } from '../api/auth_api';
import { AppProviders } from '../app/providers';
import { AuthScreen } from '../auth/auth_screen';
import { createAuthSession } from '../auth/auth_session';
import { createMemorySheetStore } from '../library/sheet_store';
import { message } from '../l10n/strings';
import { createDeferredTransport } from './fake_transport';

function renderAuth(next: string | null = null) {
  const session = createAuthSession({
    authApi: createAuthApi({
      baseUrl: 'http://api.test',
      transport: createDeferredTransport().transport,
      readCsrfToken: () => undefined,
    }),
  });
  return render(
    <AppProviders session={session} sheetStore={createMemorySheetStore()}>
      <MemoryRouter>
        <AuthScreen next={next} />
      </MemoryRouter>
    </AppProviders>,
  );
}

describe('auth surface accessibility', () => {
  it('Happy path: modes are tabs wired to one tabpanel', () => {
    renderAuth();
    const login = screen.getByRole('tab', { name: message('auth.modeLogin') });
    const register = screen.getByRole('tab', { name: message('auth.modeRegister') });

    expect(screen.getByRole('tablist')).toBeInTheDocument();
    expect(login).toHaveAttribute('aria-selected', 'true');
    expect(register).toHaveAttribute('aria-selected', 'false');
    expect(register).toHaveAttribute('tabindex', '-1');
    expect(login).toHaveAttribute('aria-controls', screen.getByRole('tabpanel').id);
  });

  it('Edge: arrow, Home and End keys move selection and focus with wrap-around', async () => {
    const user = userEvent.setup();
    renderAuth();
    const login = screen.getByRole('tab', { name: message('auth.modeLogin') });
    const register = screen.getByRole('tab', { name: message('auth.modeRegister') });

    login.focus();
    await user.keyboard('{ArrowRight}');
    expect(register).toHaveAttribute('aria-selected', 'true');
    expect(register).toHaveFocus();

    await user.keyboard('{ArrowRight}');
    expect(login).toHaveAttribute('aria-selected', 'true');
    expect(login).toHaveFocus();

    await user.keyboard('{End}');
    expect(register).toHaveFocus();
    await user.keyboard('{Home}');
    expect(login).toHaveFocus();
    await user.keyboard('{ArrowLeft}');
    expect(register).toHaveAttribute('aria-selected', 'true');
  });

  it('Happy path: the password toggle reveals and hides with aria-pressed', async () => {
    const user = userEvent.setup();
    renderAuth();
    const field = screen.getByLabelText(message('auth.passwordLabel'));
    const toggle = screen.getByRole('button', { name: message('auth.showPassword') });

    expect(toggle).toHaveAttribute('aria-pressed', 'false');
    await user.click(toggle);
    expect(field).toHaveAttribute('type', 'text');
    const hide = screen.getByRole('button', { name: message('auth.hidePassword') });
    expect(hide).toHaveAttribute('aria-pressed', 'true');
    await user.click(hide);
    expect(field).toHaveAttribute('type', 'password');
  });

  it('Happy path: a sheet destination names the sheet in the subtitle; other paths do not', () => {
    const { unmount } = renderAuth('/sheets/abc');
    expect(screen.getByText(message('auth.subtitleLoginSheet'))).toBeInTheDocument();
    unmount();
    renderAuth('/your-sheet');
    expect(screen.getByText(message('auth.subtitleLogin'))).toBeInTheDocument();
  });
});
