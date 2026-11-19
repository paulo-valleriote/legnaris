import js from '@eslint/js';
import globals from 'globals';
import tseslint from 'typescript-eslint';
import react from 'eslint-plugin-react';
import reactHooks from 'eslint-plugin-react-hooks';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import noUnsanitized from 'eslint-plugin-no-unsanitized';
import security from 'eslint-plugin-security';
import prettier from 'eslint-config-prettier';

const STORAGE_MESSAGE =
  'Credentials are memory-only; storage is allowed only in the theme/view-state seams';
const FETCH_MESSAGE = 'Network calls go through the injected transport (src/api/transport.ts).';
const XHR_MESSAGE =
  'XMLHttpRequest is forbidden; use the injected transport (src/api/transport.ts).';
const IDB_MESSAGE = 'idb may be imported only by src/library/idb_sheet_store.ts.';

const storageGlobals = ['localStorage', 'sessionStorage'].map((name) => ({
  name,
  message: STORAGE_MESSAGE,
}));
const storageProperties = ['window', 'self', 'globalThis'].flatMap((object) =>
  ['localStorage', 'sessionStorage'].map((property) => ({
    object,
    property,
    message: STORAGE_MESSAGE,
  })),
);
const networkGlobals = [
  { name: 'fetch', message: FETCH_MESSAGE },
  { name: 'XMLHttpRequest', message: XHR_MESSAGE },
];

export default tseslint.config(
  {
    ignores: [
      'dist',
      'coverage',
      'node_modules',
      'playwright-report',
      'test-results',
      'blob-report',
      // Compiled by vite-plugin-pwa (Workbox injectManifest), not by tsc.
      'src/pwa/sw.ts',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  react.configs.flat.recommended,
  react.configs.flat['jsx-runtime'],
  reactHooks.configs.flat.recommended,
  noUnsanitized.configs.recommended,
  jsxA11y.flatConfigs.recommended,
  {
    plugins: { security },
    languageOptions: {
      ecmaVersion: 2023,
      globals: { ...globals.browser, ...globals.node },
    },
    settings: { react: { version: 'detect' } },
    rules: {
      // Anti-XSS sinks (SEC-02, FND-10, contracts/ci-gates.contract.md).
      'react/no-danger': 'error',
      'no-eval': 'error',
      'no-new-func': 'error',
      'no-script-url': 'error',
      'no-unsanitized/property': 'error',
      'no-unsanitized/method': 'error',
      'no-restricted-syntax': [
        'error',
        {
          selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']",
          message: 'dangerouslySetInnerHTML is forbidden (FND-10).',
        },
      ],
      'no-empty': ['error', { allowEmptyCatch: true }],
      '@typescript-eslint/consistent-type-imports': 'error',
      'security/detect-eval-with-expression': 'error',
      'security/detect-non-literal-fs-filename': 'off',
    },
  },
  {
    files: ['**/*.test.{ts,tsx}', 'src/test/**/*.{ts,tsx}', 'e2e/**/*.{ts,tsx}'],
    languageOptions: { globals: { ...globals.node, ...globals.browser } },
  },
  {
    files: ['src/**/*.{ts,tsx}', 'public/**/*.js'],
    rules: {
      'no-restricted-globals': ['error', ...storageGlobals, ...networkGlobals],
      'no-restricted-properties': ['error', ...storageProperties],
      'no-restricted-imports': [
        'error',
        {
          paths: [{ name: 'idb', message: IDB_MESSAGE }],
        },
      ],
    },
  },
  {
    files: ['src/theme/use_theme.ts', 'src/sheet/view_store.ts', 'public/theme-init.js'],
    rules: {
      'no-restricted-globals': ['error', ...networkGlobals],
      'no-restricted-properties': 'off',
    },
  },
  {
    files: ['src/api/transport.ts'],
    rules: {
      'no-restricted-globals': ['error', ...storageGlobals],
    },
  },
  {
    files: ['src/library/idb_sheet_store.ts'],
    rules: { 'no-restricted-imports': 'off' },
  },
  prettier,
);
