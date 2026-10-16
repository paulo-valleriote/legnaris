import { describe, expect, it } from 'vitest';
import { act, screen } from '@testing-library/react';
import { renderApp } from './render';
import { setViewportWidth } from './match_media';

describe('application shell responsiveness', () => {
  it('Happy path: all three regions render at desktop width (FND-03)', () => {
    setViewportWidth(1280);
    renderApp('/');

    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.getByRole('complementary')).toBeInTheDocument();
  });

  it('Edge: the contextual panel collapses first as the viewport narrows (FND-03)', () => {
    setViewportWidth(600);
    renderApp('/');

    // The panel is gone; navigation and content remain.
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
  });

  it('Edge: narrowing after render hides only the panel (FND-03)', () => {
    setViewportWidth(1280);
    renderApp('/');
    expect(screen.getByRole('complementary')).toBeInTheDocument();

    act(() => {
      setViewportWidth(600);
    });

    expect(screen.queryByRole('complementary')).not.toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
  });
});
