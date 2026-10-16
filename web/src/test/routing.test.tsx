import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { renderApp } from './render';
import { DECLARED_ROUTES, NOT_FOUND_PATH } from '../app/routes';

describe('declared routes (FND-08)', () => {
  it('Happy path: the route table declares the root and exactly one fallback', () => {
    const fallbacks = DECLARED_ROUTES.filter((route) => route.notFound);
    expect(DECLARED_ROUTES.length).toBeGreaterThan(0);
    expect(fallbacks).toHaveLength(1);
    expect(fallbacks[0].path).toBe(NOT_FOUND_PATH);
  });

  it('Edge: an unknown path renders the designed not-found state inside the shell', () => {
    renderApp('/does-not-exist');

    expect(screen.getByTestId('not-found')).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
  });

  it('Happy path: a deep link to the root renders the same screen as in-app navigation', () => {
    renderApp('/');

    expect(screen.getByTestId('catalog')).toBeInTheDocument();
    expect(screen.queryByTestId('not-found')).not.toBeInTheDocument();
  });
});
