import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createAuthApi } from '../api/auth_api';
import { AppProviders } from '../app/providers';
import { AuthCodeScreen } from '../auth/auth_code_screen';
import { createAuthSession } from '../auth/auth_session';
import { createMemorySheetStore } from '../library/sheet_store';
import { message } from '../l10n/strings';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import type { FakeOutcome } from './fake_transport';

const here = path.dirname(fileURLToPath(import.meta.url));
const BASE_URL = 'http://api.test';

const CHALLENGE_BODY = {
  challenge_id: 'challenge-1',
  expires_in: 600,
  resend_after: 60,
};

const SESSION_BODY = { access_token: 'access-1', token_type: 'Bearer', expires_in: 900 };

/**
 * Renders the code surface over a session that already holds a pending step.
 *
 * The session is driven into the pending state through the real credential
 * operation, so the surface reads a step the server actually answered.
 */
async function renderCodeSurface(
  outcomes: FakeOutcome[] = [successOutcome(CHALLENGE_BODY)],
  email = 'a@b.test',
) {
  const fake = createFakeTransport(...outcomes);
  const session = createAuthSession({
    authApi: createAuthApi({
      baseUrl: BASE_URL,
      transport: fake.transport,
      readCsrfToken: () => undefined,
    }),
  });

  await session.login({ email, password: 'secret' });

  const view = render(
    <AppProviders session={session} sheetStore={createMemorySheetStore()}>
      <AuthCodeScreen />
    </AppProviders>,
  );

  return { session, fake, ...view };
}

describe('the code surface (WCS-03, WCS-19, WCS-20, WCS-22, WCS-24)', () => {
  it('Happy path: the surface names the address, states the lifetime and takes the focus (WCS-19, WCS-20, WCS-22)', async () => {
    await renderCodeSurface();

    expect(screen.getByTestId('code-instruction')).toHaveTextContent('a@b.test');
    expect(screen.getByTestId('code-instruction')).toHaveTextContent('10 minutes');
    expect(screen.getByLabelText(message('auth.codeLabel'))).toHaveFocus();
  });

  it('Happy path: the origin line names the account mode the step started with (WCS-25)', async () => {
    await renderCodeSurface();

    expect(screen.getByTestId('code-origin')).toHaveTextContent(
      message('auth.accountModeExisting'),
    );
  });

  it('Edge: the field is one editable input with a numeric mode and the one-time-code autofill (WCS-24)', async () => {
    await renderCodeSurface();

    const field = screen.getByLabelText(message('auth.codeLabel'));

    expect(field).toHaveAttribute('inputmode', 'numeric');
    expect(field).toHaveAttribute('autocomplete', 'one-time-code');
    expect(field).toHaveAccessibleName(message('auth.codeLabel'));
  });

  it('Happy path: the six positions follow the field value, so a pasted code lands whole (WCS-24)', async () => {
    const user = userEvent.setup();
    await renderCodeSurface();

    const field = screen.getByLabelText(message('auth.codeLabel'));
    await user.click(field);
    await user.paste('042917');

    expect(field).toHaveValue('042917');
    expect(screen.getByTestId('code-slot-0')).toHaveTextContent('0');
    expect(screen.getByTestId('code-slot-1')).toHaveTextContent('4');
    expect(screen.getByTestId('code-slot-2')).toHaveTextContent('2');
    expect(screen.getByTestId('code-slot-3')).toHaveTextContent('9');
    expect(screen.getByTestId('code-slot-4')).toHaveTextContent('1');
    expect(screen.getByTestId('code-slot-5')).toHaveTextContent('7');
  });

  it('Happy path: the submitted code reaches the server exactly as typed, leading zeros included (WCS-03)', async () => {
    const user = userEvent.setup();
    const { fake } = await renderCodeSurface([
      successOutcome(CHALLENGE_BODY),
      successOutcome(SESSION_BODY),
    ]);

    const field = screen.getByLabelText(message('auth.codeLabel'));
    await user.click(field);
    await user.paste('042917');
    await user.click(screen.getByRole('button', { name: message('auth.codeSubmit') }));

    await waitFor(() => expect(fake.attempts).toBe(2));
    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual({
      challenge_id: 'challenge-1',
      code: '042917',
    });
  });

  it('Edge: a refused code keeps the entered code, keeps the focus on the field and shows the mapped copy (WCS-04, WCS-22)', async () => {
    const user = userEvent.setup();
    await renderCodeSurface([
      successOutcome(CHALLENGE_BODY),
      errorOutcome('OTP_INVALID', 'invalid or expired code', 401),
    ]);

    const field = screen.getByLabelText(message('auth.codeLabel'));
    await user.click(field);
    await user.paste('042917');
    await user.click(screen.getByRole('button', { name: message('auth.codeSubmit') }));

    await waitFor(() => expect(screen.getByTestId('code-refusal')).toBeInTheDocument());
    expect(screen.getByTestId('code-refusal')).toHaveTextContent(message('api.error.invalidCode'));
    expect(field).toHaveValue('042917');
    expect(field).toHaveFocus();
    expect(field).toHaveAttribute('aria-invalid', 'true');
    expect(field.getAttribute('aria-describedby')).toContain('auth-code-refusal');
  });

  it('Edge: the submit is disabled and shows progress while a verification is in flight, and a repeat activation issues one request (WCS-03)', async () => {
    const user = userEvent.setup();
    const deferred = createDeferredTransport();
    const session = createAuthSession({
      authApi: createAuthApi({
        baseUrl: BASE_URL,
        transport: deferred.transport,
        readCsrfToken: () => undefined,
      }),
    });

    const pending = session.login({ email: 'a@b.test', password: 'secret' });
    await waitFor(() => expect(deferred.attempts).toBe(1));
    deferred.resolve(successOutcome(CHALLENGE_BODY));
    await pending;

    render(
      <AppProviders session={session} sheetStore={createMemorySheetStore()}>
        <AuthCodeScreen />
      </AppProviders>,
    );

    await user.click(screen.getByLabelText(message('auth.codeLabel')));
    await user.paste('042917');

    const submit = screen.getByRole('button', { name: message('auth.codeSubmit') });
    await user.click(submit);

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.codeVerifying') })).toBeDisabled(),
    );
    await user.click(screen.getByRole('button', { name: message('auth.codeVerifying') }));

    expect(deferred.attempts).toBe(2);
  });

  it('Edge: starting over clears the step, issues no request and returns to the credential form (WCS-06, WCS-21)', async () => {
    const user = userEvent.setup();
    const { session, fake } = await renderCodeSurface();
    const before = fake.attempts;

    await user.click(screen.getByTestId('code-back'));

    expect(session.getSnapshot()).toMatchObject({ state: 'unauthenticated', pending: null });
    expect(fake.attempts).toBe(before);
  });

  it('Security: the entered code is never persisted and never becomes the field name (WCS-05)', async () => {
    const user = userEvent.setup();
    await renderCodeSurface();

    const field = screen.getByLabelText(message('auth.codeLabel'));
    await user.click(field);
    await user.paste('042917');

    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBe(0);
    expect(field).toHaveAccessibleName(message('auth.codeLabel'));
    expect(field.getAttribute('aria-label')).toBeNull();
  });
});

describe('the code surface composition (WCS-25)', () => {
  it('Happy path: the blocks appear in the declared order and the mode switch is gone (WCS-25)', async () => {
    await renderCodeSurface();

    expect(screen.queryByRole('group', { name: message('auth.modeGroupLabel') })).toBeNull();

    const blocks = [
      screen.getByTestId('code-origin'),
      screen.getByRole('heading', { level: 1 }),
      screen.getByTestId('code-instruction'),
      screen.getByLabelText(message('auth.codeLabel')),
      screen.getByRole('button', { name: message('auth.codeSubmit') }),
      screen.getByTestId('code-back'),
    ];

    for (let index = 0; index < blocks.length - 1; index += 1) {
      const position = blocks[index].compareDocumentPosition(blocks[index + 1]);
      expect(position & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    }
  });

  it('Edge: the swap animates opacity and transform only, and no longer than the longest motion duration (WCS-25)', () => {
    const styles = readFileSync(path.resolve(here, '../auth/auth.css'), 'utf8');
    const keyframes = styles.match(/@keyframes lg-auth-swap \{[\s\S]*?\n\}/)?.[0] ?? '';
    const properties = [...keyframes.matchAll(/^\s*([a-z-]+):/gm)].map((match) => match[1]);

    expect([...new Set(properties)]).toEqual(['opacity', 'transform']);
    expect(styles).toContain('animation: lg-auth-swap var(--lg-motion-240)');
  });

  it('Edge: the swap is zeroed when the person asks for reduced motion (WCS-25)', () => {
    const styles = readFileSync(path.resolve(here, '../auth/auth.css'), 'utf8');
    const reduced =
      styles.match(/@media \(prefers-reduced-motion: reduce\) \{[\s\S]*?\n\}/)?.[0] ?? '';

    expect(reduced).toContain('animation: none');
  });
});
