import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { act, screen } from '@testing-library/react';
import { renderApp } from './render';
import { setViewportWidth } from './match_media';

const here = path.dirname(fileURLToPath(import.meta.url));

describe('application shell responsiveness', () => {
  it('Happy path: both regions render at desktop width (UIF-24)', () => {
    setViewportWidth(1280);
    renderApp('/');

    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument();
  });

  it('Edge: at a narrow viewport the navigation stays visible and both regions remain (UIF-25)', () => {
    setViewportWidth(600);
    renderApp('/');

    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument();
  });

  it('Edge: narrowing after render keeps the same two regions, with no third (UIF-25, UIF-32)', () => {
    setViewportWidth(1280);
    renderApp('/');
    expect(screen.getByRole('main')).toBeInTheDocument();

    act(() => {
      setViewportWidth(600);
    });

    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument();
  });

  it('Edge: the grid keeps a tokenised content minimum, so the regions never overlap (UIF-25)', () => {
    const css = readFileSync(path.resolve(here, '../shell/shell.css'), 'utf8');
    const shell = css.match(/\.lg-shell\s*\{([\s\S]*?)\}/)?.[1] ?? '';

    expect(shell).toContain('minmax(var(--lg-size-content-min), 1fr)');
  });
});
