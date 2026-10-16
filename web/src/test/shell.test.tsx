import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import { message } from '../l10n/strings';
import { renderApp } from './render';

describe('application shell', () => {
  it('Happy path: renders navigation, content and the contextual panel in order (FND-01)', () => {
    renderApp('/');

    const regions = [
      screen.getByRole('navigation', { name: 'Primary' }),
      screen.getByRole('main'),
      screen.getByRole('complementary'),
    ];

    for (let i = 1; i < regions.length; i += 1) {
      const follows =
        regions[i - 1].compareDocumentPosition(regions[i]) & Node.DOCUMENT_POSITION_FOLLOWING;
      expect(follows).toBeTruthy();
    }
  });

  it('Happy path: the content region shows the template catalog in place of the empty workspace (WCT-01)', async () => {
    renderApp('/');

    const main = screen.getByRole('main');
    expect(within(main).getByTestId('catalog')).toBeInTheDocument();
    await waitFor(() =>
      expect(within(main).getByText(message('catalog.empty'))).toBeInTheDocument(),
    );
  });

  it('Edge: the content region renders no placeholder rows or demo data (FND-01)', async () => {
    renderApp('/');

    await waitFor(() => expect(screen.getByText(message('catalog.empty'))).toBeInTheDocument());

    expect(screen.queryAllByRole('row')).toHaveLength(0);
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
    expect(screen.queryAllByRole('table')).toHaveLength(0);
  });
});
