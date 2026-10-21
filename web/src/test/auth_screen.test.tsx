import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createAuthApi } from '../api/auth_api';
import type { Transport } from '../api/transport';
import { AppProviders } from '../app/providers';
import { AuthScreen } from '../auth/auth_screen';
import { createAuthSession } from '../auth/auth_session';
import { createMemorySheetStore } from '../library/sheet_store';
import { message } from '../l10n/strings';
import { createDeferredTransport, errorOutcome, successOutcome } from './fake_transport';

const BASE_URL = 'http://api.test';

const SESSION_BODY = { access_token: 'access-1', token_type: 'Bearer', expires_in: 900 };

/** Renders the auth surface over a session built on the injected transport. */
function renderAuth(
  transport: Transport,
  readCsrfToken: () => string | undefined = () => undefined,
) {
  const session = createAuthSession({
    authApi: createAuthApi({ baseUrl: BASE_URL, transport, readCsrfToken }),
  });

  const view = render(
    <AppProviders session={session} sheetStore={createMemorySheetStore()}>
      <AuthScreen />
    </AppProviders>,
  );

  return { session, ...view };
}

describe('auth surface (WAS-07, WAS-09, WAS-10, WAS-31)', () => {
  it('Happy path: the login mode renders email and password, and the register mode adds a username (WAS-09)', async () => {
    const user = userEvent.setup();
    const deferred = createDeferredTransport();
    renderAuth(deferred.transport);

    expect(screen.getByLabelText(message('auth.emailLabel'))).toBeInTheDocument();
    expect(screen.getByLabelText(message('auth.passwordLabel'))).toBeInTheDocument();
    expect(screen.queryByLabelText(message('auth.usernameLabel'))).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('auth.modeRegister') }));

    expect(screen.getByLabelText(message('auth.usernameLabel'))).toBeInTheDocument();
  });

  it('Happy path: the heading and supporting line follow the selected mode (WAS-09)', async () => {
    const user = userEvent.setup();
    renderAuth(createDeferredTransport().transport);

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(message('auth.titleLogin'));
    expect(screen.getByText(message('auth.subtitleLogin'))).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('auth.modeRegister') }));

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      message('auth.titleRegister'),
    );
    expect(screen.queryByText(message('auth.subtitleLogin'))).not.toBeInTheDocument();
    expect(screen.getByText(message('auth.subtitleRegister'))).toBeInTheDocument();
  });

  it('Happy path: the mode switch names its group and marks exactly one mode as pressed (WAS-09)', () => {
    renderAuth(createDeferredTransport().transport);

    expect(screen.getByRole('group', { name: message('auth.modeGroupLabel') })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('auth.modeLogin') })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: message('auth.modeRegister') })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('Edge: the password is a hidden-content control (WAS-09)', () => {
    const deferred = createDeferredTransport();
    renderAuth(deferred.transport);

    expect(screen.getByLabelText(message('auth.passwordLabel'))).toHaveAttribute(
      'type',
      'password',
    );
  });

  it('Happy path: the primary action disables and shows progress while a submit is in flight, and a repeat activation issues no second request (WAS-09)', async () => {
    const user = userEvent.setup();
    const deferred = createDeferredTransport();
    renderAuth(deferred.transport);

    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');

    const submit = screen.getByRole('button', { name: message('auth.submitLogin') });
    await user.click(submit);

    expect(submit).toBeDisabled();
    expect(submit).toHaveAttribute('aria-busy', 'true');

    await user.click(submit);
    expect(deferred.attempts).toBe(1);

    deferred.resolve(successOutcome(SESSION_BODY));
  });

  it('Edge: a failed submit keeps the typed values and renders the mapped copy (WAS-07)', async () => {
    const user = userEvent.setup();
    const fake = createDeferredTransport();
    renderAuth(fake.transport);

    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));

    fake.resolve(errorOutcome('UNAUTHORIZED', 'bad credentials', 401));

    await waitFor(() => expect(screen.getByTestId('auth-error')).toBeInTheDocument());
    expect(screen.getByTestId('auth-error')).toHaveTextContent(message('api.error.unauthorized'));
    expect(screen.getByLabelText(message('auth.emailLabel'))).toHaveValue('a@b.test');
    expect(screen.getByLabelText(message('auth.passwordLabel'))).toHaveValue('secret');
  });

  it('Edge: a duplicate address names the conflict, and any other unmapped registration failure keeps the generic copy (WAS-10, WCS-11)', async () => {
    const user = userEvent.setup();
    const duplicate = createDeferredTransport();
    const { unmount } = renderAuth(duplicate.transport);

    await user.click(screen.getByRole('button', { name: message('auth.modeRegister') }));
    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.usernameLabel')), 'aragorn');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitRegister') }));
    duplicate.resolve(errorOutcome('CONFLICT', 'email already exists', 409));

    await waitFor(() => expect(screen.getByTestId('auth-error')).toBeInTheDocument());
    expect(screen.getByTestId('auth-error').textContent).toBe(message('api.error.conflict'));
    expect(screen.getByTestId('auth-error').textContent).not.toContain('already exists');
    expect(screen.getByLabelText(message('auth.emailLabel'))).toHaveValue('a@b.test');

    unmount();

    const other = createDeferredTransport();
    renderAuth(other.transport);
    await user.click(screen.getByRole('button', { name: message('auth.modeRegister') }));
    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.usernameLabel')), 'aragorn');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitRegister') }));
    other.resolve(errorOutcome('SOMETHING_NEW', 'email already exists', 409));

    await waitFor(() => expect(screen.getByTestId('auth-error')).toBeInTheDocument());
    expect(screen.getByTestId('auth-error').textContent).toBe(message('api.error.unknown'));
  });

  it('Edge: a server message carrying markup never becomes DOM (WAS-10, WAS-31)', async () => {
    const user = userEvent.setup();
    const deferred = createDeferredTransport();
    const { container } = renderAuth(deferred.transport);

    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));
    deferred.resolve(errorOutcome('INTERNAL_ERROR', '<script>alert(1)</script>', 500));

    await waitFor(() => expect(screen.getByTestId('auth-error')).toBeInTheDocument());
    expect(container.querySelector('script')).toBeNull();
    expect(screen.getByTestId('auth-error')).toHaveTextContent(message('api.error.serverError'));
  });

  it('Edge: switching mode clears a stale error but keeps the typed values (WAS-07)', async () => {
    const user = userEvent.setup();
    const fake = createDeferredTransport();
    renderAuth(fake.transport);

    await user.type(screen.getByLabelText(message('auth.emailLabel')), 'a@b.test');
    await user.type(screen.getByLabelText(message('auth.passwordLabel')), 'secret');
    await user.click(screen.getByRole('button', { name: message('auth.submitLogin') }));
    fake.resolve(errorOutcome('UNAUTHORIZED', 'bad credentials', 401));
    await waitFor(() => expect(screen.getByTestId('auth-error')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: message('auth.modeRegister') }));

    expect(screen.queryByTestId('auth-error')).not.toBeInTheDocument();
    expect(screen.getByLabelText(message('auth.emailLabel'))).toHaveValue('a@b.test');
    expect(screen.getByLabelText(message('auth.passwordLabel'))).toHaveValue('secret');
  });
});
