import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import { installMatchMedia } from './match_media';

installMatchMedia();

afterEach(() => {
  cleanup();
  installMatchMedia();
});
