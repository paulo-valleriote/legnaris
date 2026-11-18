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
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';

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

  it('Happy path: the password toggle reveals and hides with aria-pressed and a fixed name', async () => {
    const user = userEvent.setup();
    renderAuth();
    const field = screen.getByLabelText(message('auth.passwordLabel'));
    const toggle = screen.getByRole('button', { name: message('auth.showPassword') });

    expect(toggle).toHaveAttribute('aria-pressed', 'false');
    await user.click(toggle);
    expect(field).toHaveAttribute('type', 'text');
    expect(toggle).toHaveAttribute('aria-pressed', 'true');
    expect(toggle).toHaveAccessibleName(message('auth.showPassword'));
    await user.click(toggle);
    expect(field).toHaveAttribute('type', 'password');
    expect(toggle).toHaveAttribute('aria-pressed', 'false');
  });

  it('Edge: a refusal is announced once, and the credential fields carry aria-invalid and its description', async () => {
    const user = userEvent.setup();
    const deferred = createDeferredTransport();
    const session = createAuthSession({
      authApi: createAuthApi({
        baseUrl: 'http://api.test',
        transport: deferred.transport,
        readCsrfToken: () => undefined,
      }),
    });
    render(
      <AppProviders session={session} sheetStore={createMemorySheetStore()}>
        <MemoryRouter>
          <AuthScreen />
        </MemoryRouter>
      </AppProviders>,
    );

    const email = screen.getByLabelText(message('auth.emailLabel'));
    const password = screen.getByLabelText(message('auth.passwordLabel'));
    expect(email).toHaveAttribute('aria-invalid', 'false');
    expect(email).not.toHaveAttribute('aria-describedby');

    await user.type(email, 'a@b.test');
    await user.type(password, 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));
    deferred.resolve(errorOutcome('UNAUTHORIZED', 'bad credentials', 401));

    const error = await screen.findByTestId('auth-error');
    expect(error).toHaveAttribute('role', 'alert');
    expect(error).not.toHaveAttribute('aria-live');
    expect(error).toHaveAttribute('id', 'auth-error');
    expect(email).toHaveAttribute('aria-invalid', 'true');
    expect(password).toHaveAttribute('aria-invalid', 'true');
    expect(email.getAttribute('aria-describedby')).toBe('auth-error');
    expect(password.getAttribute('aria-describedby')).toBe('auth-error');
    expect(email).toHaveAccessibleDescription(error.textContent ?? '');
  });

  it('Happy path: a sheet destination names the sheet in the subtitle; other paths do not', () => {
    const { unmount } = renderAuth('/sheets/abc');
    expect(screen.getByText(message('auth.subtitleLoginSheet'))).toBeInTheDocument();
    unmount();
    renderAuth('/your-sheet');
    expect(screen.getByText(message('auth.subtitleLogin'))).toBeInTheDocument();
  });

  it('Edge: the deferred-delivery notice is informational, never the refusal tone', async () => {
    const fake = createAuthSession({
      authApi: createAuthApi({
        baseUrl: 'http://api.test',
        transport: createFakeTransport(successOutcome({ delivery: 'deferred' }, 201)).transport,
        readCsrfToken: () => undefined,
      }),
    });
    await fake.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });

    render(
      <AppProviders session={fake} sheetStore={createMemorySheetStore()}>
        <MemoryRouter>
          <AuthScreen restore={fake.getSnapshot().restored} initialMode="register" />
        </MemoryRouter>
      </AppProviders>,
    );

    const notice = screen.getByText(message('auth.deferredNotice')).closest('.lg-notice');
    expect(notice).toHaveAttribute('role', 'status');
    expect(notice).toHaveClass('lg-notice--info', 'lg-notice--stage');
    expect(notice).not.toHaveClass('lg-notice--danger');
  });
});
