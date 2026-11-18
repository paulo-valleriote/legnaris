import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { message } from '../l10n/strings';
import { PrivacyScreen } from '../legal/privacy_screen';

const EN_HEADING = 'Privacy policy';
const PT_HEADING = 'Política de privacidade';

function renderScreen(locale?: 'en' | 'pt-BR') {
  return render(
    <MemoryRouter initialEntries={['/privacy']}>
      <PrivacyScreen locale={locale} />
    </MemoryRouter>,
  );
}

describe('privacy screen (LGN-234)', () => {
  it('Happy path: the English locale renders its heading as the page h1', () => {
    renderScreen('en');

    expect(screen.getByRole('heading', { level: 1, name: EN_HEADING })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: PT_HEADING })).not.toBeInTheDocument();
  });

  it('Happy path: the pt-BR locale renders its heading and sets the document language', () => {
    const { container } = renderScreen('pt-BR');

    expect(screen.getByRole('heading', { level: 1, name: PT_HEADING })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: EN_HEADING })).not.toBeInTheDocument();
    expect(container.querySelector('main')).toHaveAttribute('lang', 'pt-BR');
  });

  it('Happy path: the page offers one way back to the landing', () => {
    renderScreen('en');

    expect(screen.getByRole('link', { name: message('privacy.back') })).toHaveAttribute(
      'href',
      '/',
    );
  });

  it('Edge: the title is drawn once, never repeated by the article body', () => {
    renderScreen('en');

    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
  });

  it('Edge: the authoring HTML comment is not rendered in either locale', () => {
    const { container, unmount } = renderScreen('en');

    expect(container.innerHTML).not.toContain('<!--');
    expect(screen.queryByText(/Privacy policy update/)).not.toBeInTheDocument();
    unmount();

    const pt = renderScreen('pt-BR');
    expect(pt.container.innerHTML).not.toContain('<!--');
  });

  it('Edge: bold runs render as strong elements and no literal asterisks remain', () => {
    const { container } = renderScreen('en');

    const strong = container.querySelectorAll('strong');
    expect(strong.length).toBeGreaterThan(0);
    expect(container.textContent).not.toContain('**');
  });

  it('Edge: bullet items render as list elements inside the body', () => {
    const { container } = renderScreen('en');

    const items = container.querySelectorAll('article ul > li');
    expect(items.length).toBeGreaterThan(0);
    expect(container.textContent).not.toMatch(/^- /m);
  });

  it('Happy path: bare URLs become links to their address', () => {
    renderScreen('en');

    const link = screen.getByRole('link', { name: 'https://stripe.com/privacy' });
    expect(link).toHaveAttribute('href', 'https://stripe.com/privacy');
  });

  it('Edge: no link is produced for a script URL', () => {
    const { container } = renderScreen('en');

    expect(container.querySelectorAll('a[href^="javascript:"]')).toHaveLength(0);
  });

  it('Edge: without a locale the screen renders a page in one of the two supported languages', () => {
    const { container } = renderScreen();

    expect(['en', 'pt-BR']).toContain(container.querySelector('main')?.getAttribute('lang'));
  });
});
