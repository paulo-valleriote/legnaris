import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup, configure } from '@testing-library/react';
import { installMatchMedia } from './match_media';

configure({ asyncUtilTimeout: 5_000 });

installMatchMedia();

afterEach(() => {
  cleanup();
  installMatchMedia();
});
