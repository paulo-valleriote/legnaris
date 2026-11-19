import { beforeAll, describe, expect, it } from 'vitest';
import { messagesAt, restricted, ruleIds, warmUpEslint } from './eslint_harness';

beforeAll(warmUpEslint, 60000);

describe('ESLint network seam (LGN-291)', () => {
  it('Error: fetch and XMLHttpRequest are refused outside the transport', async () => {
    const fetchMessages = await messagesAt(
      'src/sheet/probe.ts',
      "export const pending = fetch('/health');",
    );
    expect(ruleIds(fetchMessages)).toContain('no-restricted-globals');

    const xhrMessages = await messagesAt(
      'src/sheet/probe.ts',
      'export const request = new XMLHttpRequest();',
    );
    expect(ruleIds(xhrMessages)).toContain('no-restricted-globals');
  });

  it('Happy path: the transport module is the one network seam', async () => {
    const code = [
      "export const pending = fetch('/health');",
      'export const request = new XMLHttpRequest();',
    ].join('\n');

    expect(restricted(await messagesAt('src/api/transport.ts', code))).toEqual([]);
  });
});
