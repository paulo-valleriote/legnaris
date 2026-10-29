import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { message } from '../l10n/strings';
import { renderApp } from './render';

const here = path.dirname(fileURLToPath(import.meta.url));
const publicDir = path.resolve(here, '../../public');
const stageSource = readFileSync(path.resolve(here, '../auth/auth_stage.tsx'), 'utf8');

async function visit(path: string) {
  renderApp(path, { session: 'unauthenticated' });
  await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());
}

describe('auth stage', () => {
  it('Happy path: the sign-in and sign-up surfaces open with the photograph and a brand link to the landing', async () => {
    for (const route of ['/sign-in', '/sign-up']) {
      const view = renderApp(route, { session: 'unauthenticated' });
      await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());

      expect(screen.getByRole('link', { name: message('app.name') })).toHaveAttribute('href', '/');
      expect(view.container.querySelector('.lg-auth__photo')).toHaveAttribute('alt', '');
      view.unmount();
    }
  });

  it('Edge: the photograph is decorative and never reaches the accessibility tree', async () => {
    await visit('/sign-in');

    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });

  it('Edge: the form is still the surface heading, the lockup adds none', async () => {
    await visit('/sign-up');

    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      message('auth.titleRegister'),
    );
  });

  it('Edge: every size the stage names exists in public/ on the app origin, with no third-party address', () => {
    const images = [...stageSource.matchAll(/'(\/images\/hero-lava-\d+\.webp)(?: \d+w)?'/g)].map(
      (match) => match[1],
    );

    expect(new Set(images).size).toBeGreaterThanOrEqual(2);
    for (const image of images) {
      expect(image.startsWith('//')).toBe(false);
      expect(existsSync(path.join(publicDir, image))).toBe(true);
    }
    expect(stageSource).not.toMatch(/https?:\/\//);
  });
});
