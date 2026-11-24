import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { DECLARED_ROUTES, SRD_PATH } from '../app/routes';
import { message } from '../l10n/strings';
import { renderApp } from './render';

const ATTRIBUTION =
  'This work includes material from the System Reference Document 5.2.1 (“SRD 5.2.1”) by Wizards of the Coast LLC, available at https://www.dndbeyond.com/srd. The SRD 5.2.1 is licensed under the Creative Commons Attribution 4.0 International License, available at https://creativecommons.org/licenses/by/4.0/legalcode.';

const ADAPTATION =
  "Legnaris adapts SRD 5.2.1 rules into sheet fields, formulas and labels; rule text is shortened and the name of the publisher's game is removed.";

async function visit(path: string) {
  const view = renderApp(path, { session: 'unauthenticated' });
  await waitFor(() => expect(screen.queryByTestId('auth-restoring')).not.toBeInTheDocument());
  return view;
}

describe('srd route (LGN-420)', () => {
  it('Happy path: the srd route is declared public with a deep link', () => {
    const route = DECLARED_ROUTES.find((declared) => declared.path === SRD_PATH);

    expect(route).toMatchObject({ audience: 'public', deepLink: true, notFound: false });
  });

  it('Happy path: a visitor without a session reads both texts verbatim on a direct navigation', async () => {
    await visit(SRD_PATH);

    const attribution = screen.getByText(ATTRIBUTION);
    const adaptation = screen.getByText(ADAPTATION);

    expect(attribution.textContent).toBe(ATTRIBUTION);
    expect(adaptation.textContent).toBe(ADAPTATION);
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(message('srd.title'));
    expect(screen.queryByRole('navigation', { name: 'Primary' })).not.toBeInTheDocument();
  });

  it('Edge: the page carries only the two attribution paragraphs and no disclaimer', async () => {
    await visit(SRD_PATH);

    expect(screen.getByRole('main').querySelectorAll('article p')).toHaveLength(2);
    expect(screen.queryByText(/not affiliated/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/trademark/i)).not.toBeInTheDocument();
  });

  it('Edge: no public page links to the srd page', async () => {
    for (const path of ['/', '/demo', '/privacy']) {
      const view = await visit(path);
      await screen.findByRole('heading', { level: 1 });
      const hrefs = screen.getAllByRole('link').map((link) => link.getAttribute('href') ?? '');
      for (const href of hrefs) {
        expect(new URL(href, 'http://legnaris.test').pathname).not.toBe(SRD_PATH);
      }
      view.unmount();
    }
  });
});
