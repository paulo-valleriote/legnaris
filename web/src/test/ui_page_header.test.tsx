import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { PageHeader } from '../ui/page_header';
import { messages } from '../l10n/strings';

function renderHeader(node: ReactNode) {
  return render(<MemoryRouter>{node}</MemoryRouter>);
}

describe('page header primitive', () => {
  it('Happy path: the title is the page h1 and the meta row follows it', () => {
    renderHeader(<PageHeader title="Your account" meta={<span>Free plan</span>} />);
    expect(screen.getByRole('heading', { level: 1, name: 'Your account' })).toBeInTheDocument();
    expect(screen.getByText('Free plan')).toBeInTheDocument();
  });

  it('Happy path: a back link is a named ghost icon target', () => {
    renderHeader(
      <PageHeader title="Ember Coast" backTo="/campaigns" backLabel="Back to campaigns" />,
    );
    const back = screen.getByRole('link', { name: 'Back to campaigns' });
    expect(back).toHaveAttribute('href', '/campaigns');
    expect(back).toHaveAttribute('title', 'Back to campaigns');
    expect(back.querySelector('svg')).not.toBeNull();
  });

  it('Edge: without a back label the back link falls back to the shared wording', () => {
    renderHeader(<PageHeader title="Ember Coast" backTo="/campaigns" />);
    expect(screen.getByRole('link', { name: messages['common.back'] })).toBeInTheDocument();
  });

  it('Edge: no back destination renders no back link', () => {
    renderHeader(<PageHeader title="Catalog" />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('Happy path: actions render at the trailing edge of the header', () => {
    const { container } = renderHeader(
      <PageHeader title="Ember Coast" actions={<button type="button">Invite players</button>} />,
    );
    const actions = container.querySelector('.lg-page-header__actions');
    expect(actions).not.toBeNull();
    expect(screen.getByRole('button', { name: 'Invite players' })).toBeInTheDocument();
  });

  it('Happy path: an id, an extra class and a title id are placed where children need them', () => {
    const { container } = renderHeader(
      <PageHeader
        title="Ember Coast"
        id="page-top"
        className="lg-test__header"
        titleId="page-title"
      />,
    );
    const header = document.getElementById('page-top');
    const title = screen.getByRole('heading', { level: 1, name: 'Ember Coast' });

    expect(header).toBe(container.querySelector('.lg-page-header'));
    expect(header).toHaveClass('lg-test__header');
    expect(title).toHaveAttribute('id', 'page-title');
  });
});
