import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PrivacyScreen } from '../legal/privacy_screen';

const EN_HEADING = 'Privacy policy';
const PT_HEADING = 'Política de privacidade';

describe('privacy screen (LGN-234)', () => {
  it('Happy path: the English locale renders its heading as the page h1', () => {
    render(<PrivacyScreen locale="en" />);

    expect(screen.getByRole('heading', { level: 1, name: EN_HEADING })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: PT_HEADING })).not.toBeInTheDocument();
  });

  it('Happy path: the pt-BR locale renders its heading and sets the document language', () => {
    const { container } = render(<PrivacyScreen locale="pt-BR" />);

    expect(screen.getByRole('heading', { level: 1, name: PT_HEADING })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: EN_HEADING })).not.toBeInTheDocument();
    expect(container.querySelector('main')).toHaveAttribute('lang', 'pt-BR');
  });

  it('Edge: the authoring HTML comment is not rendered in either locale', () => {
    const { container, unmount } = render(<PrivacyScreen locale="en" />);

    expect(container.innerHTML).not.toContain('<!--');
    expect(screen.queryByText(/Privacy policy update/)).not.toBeInTheDocument();
    unmount();

    const pt = render(<PrivacyScreen locale="pt-BR" />);
    expect(pt.container.innerHTML).not.toContain('<!--');
  });

  it('Edge: bold runs render as strong elements and no literal asterisks remain', () => {
    const { container } = render(<PrivacyScreen locale="en" />);

    const strong = container.querySelectorAll('strong');
    expect(strong.length).toBeGreaterThan(0);
    expect(container.textContent).not.toContain('**');
  });

  it('Edge: bullet items render as list elements inside the body', () => {
    const { container } = render(<PrivacyScreen locale="en" />);

    const items = container.querySelectorAll('article ul > li');
    expect(items.length).toBeGreaterThan(0);
    expect(container.textContent).not.toMatch(/^- /m);
  });

  it('Happy path: bare URLs become links to their address', () => {
    render(<PrivacyScreen locale="en" />);

    const link = screen.getByRole('link', { name: 'https://stripe.com/privacy' });
    expect(link).toHaveAttribute('href', 'https://stripe.com/privacy');
  });

  it('Edge: no link is produced for a script URL', () => {
    const { container } = render(<PrivacyScreen locale="en" />);

    expect(container.querySelectorAll('a[href^="javascript:"]')).toHaveLength(0);
  });

  it('Edge: without a locale the screen renders a page in one of the two supported languages', () => {
    const { container } = render(<PrivacyScreen />);

    expect(['en', 'pt-BR']).toContain(container.querySelector('main')?.getAttribute('lang'));
  });
});
