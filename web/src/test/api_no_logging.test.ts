import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { createFakeTransport, successOutcome } from './fake_transport';

const here = path.dirname(fileURLToPath(import.meta.url));
const apiDir = path.resolve(here, '../api');
const authDir = path.resolve(here, '../auth');

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = path.join(dir, entry);

    if (statSync(full).isDirectory()) {
      return entry === 'fixtures' ? [] : sourceFiles(full);
    }

    return entry.endsWith('.ts') ? [full] : [];
  });
}

/** The auth sources, `.ts` and `.tsx`, so a component is inspected too (WAS-32). */
function authSourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = path.join(dir, entry);

    if (statSync(full).isDirectory()) {
      return authSourceFiles(full);
    }

    return entry.endsWith('.ts') || entry.endsWith('.tsx') ? [full] : [];
  });
}

const templatePayload = {
  id: 'fantasy_v1',
  name: 'Fantasy RPG',
  default: true,
  fields: {
    stats: {
      children: {
        strength: { schema: { type: 'number', label: 'Strength', is_editable: true, default: 10 } },
      },
    },
  },
};

const sheetPayload = {
  id: '01a0f967-f42b-710e-851f-8f293a634921',
  template_id: 'fantasy_v1',
  name: 'Aragorn',
  data: {},
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};

describe('credential hygiene (API-29)', () => {
  it('Edge: the API module contains no logging call and imports no logger (API-29)', () => {
    const files = sourceFiles(apiDir);
    expect(files.length).toBeGreaterThan(0);

    for (const file of files) {
      const source = readFileSync(file, 'utf8');

      expect(source, file).not.toMatch(/\bconsole\s*\./);
      expect(source, file).not.toMatch(/(?:from|import)\s*['"][^'"]*logger[^'"]*['"]/);
    }
  });

  it('Edge: the auth module contains no logging call and imports no logger (WAS-32)', () => {
    const files = authSourceFiles(authDir);
    expect(files.length).toBeGreaterThan(0);

    for (const file of files) {
      const source = readFileSync(file, 'utf8');

      expect(source, file).not.toMatch(/\bconsole\s*\./);
      expect(source, file).not.toMatch(/(?:from|import)\s*['"][^'"]*logger[^'"]*['"]/);
    }
  });

  it('Happy path: every request the client issues omits ambient credentials (API-29)', async () => {
    const baseUrl = 'https://api.example.test';
    const fakes = [
      createFakeTransport(successOutcome({ status: 'healthy' })),
      createFakeTransport(successOutcome([templatePayload])),
      createFakeTransport(successOutcome(templatePayload)),
      createFakeTransport(successOutcome(sheetPayload, 201)),
      createFakeTransport(successOutcome(sheetPayload)),
      createFakeTransport(successOutcome({})),
    ];

    const [health, list, detail, create, retrieve, update] = fakes.map((fake) =>
      createApiClient({ baseUrl, transport: fake.transport }),
    );

    await health.health();
    await list.listTemplates();
    await detail.getTemplate('fantasy_v1');
    await create.createSheet({ templateId: 'fantasy_v1', sheetName: 'Aragorn' });
    await retrieve.getSheet('sheet-1');
    await update.patchSheet('sheet-1', { stats: { dexterity: 12 } });

    const requests = fakes.flatMap((fake) => fake.requests);
    expect(requests).toHaveLength(6);

    for (const request of requests) {
      expect(request.credentials).toBe('omit');
    }
  });
});
