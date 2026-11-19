import { execFileSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterEach, describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const scriptPath = path.resolve(here, '../../docker-entrypoint.d/40-runtime-config.sh');
const temporaryDirectories: string[] = [];

interface RunResult {
  readonly config: string;
  readonly adsTxt: string | null;
}

function run(env: Record<string, string>, seedAdsTxt?: string): RunResult {
  const directory = mkdtempSync(path.join(tmpdir(), 'lg-runtime-config-'));
  temporaryDirectories.push(directory);

  if (seedAdsTxt !== undefined) {
    writeFileSync(path.join(directory, 'ads.txt'), seedAdsTxt);
  }

  execFileSync('sh', [scriptPath], {
    env: { PATH: process.env.PATH, RUNTIME_CONFIG_DIR: directory, ...env },
    stdio: 'pipe',
  });

  const adsTxtPath = path.join(directory, 'ads.txt');
  return {
    config: readFileSync(path.join(directory, 'config.json'), 'utf8'),
    adsTxt: existsSync(adsTxtPath) ? readFileSync(adsTxtPath, 'utf8') : null,
  };
}

function parseConfig(result: RunResult): unknown {
  return JSON.parse(result.config) as unknown;
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe('runtime config entrypoint script', () => {
  it('Happy path: no environment writes an empty document and no ads.txt', () => {
    const result = run({});

    expect(result.config).toBe('{}\n');
    expect(parseConfig(result)).toEqual({});
    expect(result.adsTxt).toBeNull();
  });

  it('Happy path: a valid client and slots publish the AdSense block and ads.txt', () => {
    const result = run({
      API_BASE_URL: 'https://api.example.test',
      ADSENSE_CLIENT: 'ca-pub-1234567890',
      ADSENSE_SLOT_CATALOG: '111',
      ADSENSE_SLOT_CAMPAIGNS: '222',
      ADSENSE_SLOT_LANDING: '333',
    });

    expect(parseConfig(result)).toEqual({
      apiBaseUrl: 'https://api.example.test',
      adsense: {
        client: 'ca-pub-1234567890',
        slots: { catalog: '111', campaigns: '222', landing: '333' },
      },
    });
    expect(result.adsTxt).toBe('google.com, pub-1234567890, DIRECT, f08c47fec0942fa0\n');
  });

  it('Edge: an invalid client omits the AdSense block and ads.txt', () => {
    const result = run({
      API_BASE_URL: 'https://api.example.test',
      ADSENSE_CLIENT: 'ca-pub-123',
    });

    expect(parseConfig(result)).toEqual({ apiBaseUrl: 'https://api.example.test' });
    expect(result.adsTxt).toBeNull();
  });

  it('Edge: invalid or blank slots are omitted and the valid ones survive', () => {
    const result = run({
      ADSENSE_CLIENT: 'ca-pub-1234567890',
      ADSENSE_SLOT_CATALOG: '111',
      ADSENSE_SLOT_CAMPAIGNS: '12x',
      ADSENSE_SLOT_LANDING: '',
    });

    expect(parseConfig(result)).toEqual({
      adsense: { client: 'ca-pub-1234567890', slots: { catalog: '111' } },
    });
  });

  it('Edge: a valid client with no valid slot keeps the client without a slots object', () => {
    const result = run({
      ADSENSE_CLIENT: 'ca-pub-1234567890',
      ADSENSE_SLOT_CATALOG: 'abc',
      ADSENSE_SLOT_CAMPAIGNS: '1'.repeat(21),
    });

    expect(parseConfig(result)).toEqual({ adsense: { client: 'ca-pub-1234567890' } });
  });

  it('Happy path: quotes and backslashes in apiBaseUrl stay escaped in valid JSON', () => {
    const value = 'https://api.example.test/"quoted"\\path';

    const result = run({ API_BASE_URL: value });

    expect(parseConfig(result)).toEqual({ apiBaseUrl: value });
  });

  it('Edge: a stale ads.txt is removed when the client is invalid', () => {
    const result = run({ ADSENSE_CLIENT: 'not-a-client' }, 'stale\n');

    expect(result.adsTxt).toBeNull();
  });

  it('Edge: a valid client replaces a stale ads.txt', () => {
    const result = run({ ADSENSE_CLIENT: 'ca-pub-1234567890' }, 'stale\n');

    expect(result.adsTxt).toBe('google.com, pub-1234567890, DIRECT, f08c47fec0942fa0\n');
  });
});
