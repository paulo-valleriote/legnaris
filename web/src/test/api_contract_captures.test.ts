import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createApiClient } from '../api/client';
import { DefensiveParseFailure, DomainFailure } from '../api/errors';
import type { FieldSchema } from '../api/models/template';
import type { FieldNode } from '../api/models/template';
import { createFakeTransport } from './fake_transport';

const here = path.dirname(fileURLToPath(import.meta.url));
const fixturesDir = path.resolve(here, '../api/fixtures');
const baseUrl = 'https://api.example.test';

interface Capture {
  operation: string;
  provenance: string;
  request: { method: string; path: string; body?: unknown };
  response?: { status: number; body: unknown };
}

const fixtureNames = [
  'health',
  'templates',
  'template_detail',
  'sheet_create_request',
  'sheet_create',
  'sheet_get',
  'patch_delta_request',
  'patch_delta_response',
  'error_not_found',
  'error_invalid_request',
  'error_unknown_field',
  'error_invalid_json',
  'error_internal',
  'error_unauthorized',
] as const;

function loadCapture(name: string): Capture {
  return JSON.parse(readFileSync(path.join(fixturesDir, `${name}.json`), 'utf8')) as Capture;
}

const health = loadCapture('health');
const templates = loadCapture('templates');
const templateDetail = loadCapture('template_detail');
const sheetCreateRequest = loadCapture('sheet_create_request');
const sheetCreate = loadCapture('sheet_create');
const sheetGet = loadCapture('sheet_get');
const patchDeltaRequest = loadCapture('patch_delta_request');
const patchDeltaResponse = loadCapture('patch_delta_response');

function respond(capture: Capture): { status: number; body: string } {
  if (capture.response === undefined) {
    throw new Error(`capture "${capture.operation}" is a request-only file`);
  }

  return { status: capture.response.status, body: JSON.stringify(capture.response.body) };
}

function clientFor(capture: Capture) {
  return createApiClient({ baseUrl, transport: createFakeTransport(respond(capture)).transport });
}

function requestBodyOf(capture: Capture): Record<string, unknown> {
  return (capture.request.body ?? {}) as Record<string, unknown>;
}

function findLeaf(
  node: FieldNode,
  predicate: (schema: FieldSchema) => boolean,
): FieldSchema | undefined {
  if (node.kind === 'leaf') {
    return predicate(node.schema) ? node.schema : undefined;
  }

  for (const child of Object.values(node.children)) {
    const found = findLeaf(child, predicate);
    if (found !== undefined) {
      return found;
    }
  }

  return undefined;
}

function leafPaths(value: unknown, prefix = ''): string[] {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return [prefix];
  }

  return Object.entries(value).flatMap(([key, child]) =>
    leafPaths(child, prefix === '' ? key : `${prefix}.${key}`),
  );
}

describe('pinned contract captures', () => {
  it('Happy path: the health capture decodes through the health operation (API-34, API-35)', async () => {
    const fake = createFakeTransport(respond(health));
    const payload = await createApiClient({ baseUrl, transport: fake.transport }).health();

    expect(health.request.method).toBe('GET');
    expect(health.request.path).toBe('/health');
    expect(health.response?.status).toBe(200);
    expect(payload.status).toBe('healthy');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}/health`);
  });

  it('Happy path: the catalog capture decodes and preserves the declared section order (API-34, API-35)', async () => {
    const decoded = await clientFor(templates).listTemplates();
    const fantasy = decoded.find((entry) => entry.id === 'fantasy_v1');

    expect(fantasy).toBeDefined();
    expect(fantasy?.default).toBe(true);
    expect(Object.keys(fantasy?.fields ?? {})).toEqual([
      'identity',
      'stats',
      'modifiers',
      'saves',
      'skills',
      'combat',
      'attacks',
      'spells',
      'derived',
      'rules',
      'config',
    ]);
  });

  it('Happy path: the detail capture carries groups, formulas, dependencies and derived leaves (API-34, API-35)', async () => {
    const decoded = await clientFor(templateDetail).getTemplate('fantasy_v1');
    const nodes = Object.values(decoded.fields);

    expect(decoded.id).toBe('fantasy_v1');
    expect(decoded.default).toBe(true);
    expect(nodes.every((node) => node.kind === 'group')).toBe(true);

    const derived = nodes
      .map((node) => findLeaf(node, (schema) => schema.formula !== undefined))
      .find((schema) => schema !== undefined);
    expect(derived?.dependsOn?.[0]?.length).toBeGreaterThan(0);
    expect(derived?.isEditable).toBe(false);

    const exclusive = nodes
      .map((node) => findLeaf(node, (schema) => schema.exclusiveGroup !== undefined))
      .find((schema) => schema !== undefined);
    expect(exclusive?.exclusiveGroup).toBe('armorClassAbility');
  });

  it('Happy path: the creation capture decodes and the request carries exactly the template and name (API-34, API-35)', async () => {
    const fake = createFakeTransport(respond(sheetCreate));
    const decoded = await createApiClient({ baseUrl, transport: fake.transport }).createSheet({
      templateId: 'fantasy_v1',
      sheetName: 'Capture Hero',
    });

    expect(decoded.id).toBeTypeOf('string');
    expect(decoded.templateId).toBe('fantasy_v1');
    expect(decoded.name).toBe('Capture Hero');
    expect(decoded.data['modifiers']).toBeDefined();

    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual(requestBodyOf(sheetCreateRequest));
    expect(Object.keys(requestBodyOf(sheetCreateRequest))).toEqual(['template_id', 'sheet_name']);
  });

  it('Happy path: the retrieval capture decodes the stored sheet at the captured path (API-34, API-35)', async () => {
    const fake = createFakeTransport(respond(sheetGet));
    const sheetId = sheetGet.request.path.replace('/sheets/', '');
    const decoded = await createApiClient({ baseUrl, transport: fake.transport }).getSheet(sheetId);

    expect(sheetGet.response?.status).toBe(200);
    expect(decoded.id).toBe(sheetId);
    expect(decoded.name).toBe('Capture Hero');
    expect(decoded.createdAt).toBeTypeOf('string');
    expect(fake.lastRequest?.url).toBe(`${baseUrl}${sheetGet.request.path}`);
  });

  it('Happy path: the update pair sends one change and returns only recalculated effects (API-34, API-35)', async () => {
    const requestData = requestBodyOf(patchDeltaRequest)['data'];
    const changedPaths = leafPaths(requestData);
    const fake = createFakeTransport(respond(patchDeltaResponse));
    const sheetId = patchDeltaResponse.request.path.replace('/sheets/', '');

    const effects = await createApiClient({ baseUrl, transport: fake.transport }).patchSheet(
      sheetId,
      requestData as Record<string, unknown>,
    );

    expect(changedPaths).toHaveLength(1);
    expect(Object.keys(requestBodyOf(patchDeltaRequest))).toEqual(['data']);
    expect(fake.lastRequest?.method).toBe('PATCH');

    const effectPaths = leafPaths(effects);
    expect(effectPaths.length).toBeGreaterThan(0);
    expect(effectPaths).not.toContain(changedPaths[0]);
    expect(effects).toEqual((patchDeltaResponse.response?.body as { data: unknown }).data);
  });

  it('Happy path: every captured failure maps to the kind the interface branches on (API-34, API-35)', async () => {
    const cases: ReadonlyArray<readonly [Capture, string, string]> = [
      [loadCapture('error_not_found'), 'notFound', 'NOT_FOUND'],
      [loadCapture('error_invalid_request'), 'invalidEdit', 'INVALID_REQUEST'],
      [loadCapture('error_unknown_field'), 'renderMismatch', 'UNKNOWN_FIELD'],
      [loadCapture('error_invalid_json'), 'clientFault', 'INVALID_JSON'],
      [loadCapture('error_internal'), 'serverError', 'INTERNAL_ERROR'],
      [loadCapture('error_unauthorized'), 'unknown', 'UNAUTHORIZED'],
    ];

    for (const [capture, kind, code] of cases) {
      const failure = await clientFor(capture)
        .health()
        .catch((error: unknown) => error);

      expect(failure).toBeInstanceOf(DomainFailure);
      expect(failure).toMatchObject({ kind, code });
      expect((failure as DomainFailure).status).toBe(capture.response?.status);
    }
  });

  it('Error: altering a key in a capture turns the decode red on the first run (API-36)', async () => {
    const mutated = structuredClone(templateDetail);
    const body = mutated.response?.body as { data: Record<string, unknown> };
    delete body.data['default'];

    const fake = createFakeTransport({
      status: mutated.response?.status ?? 200,
      body: JSON.stringify(mutated.response?.body),
    });

    await expect(
      createApiClient({ baseUrl, transport: fake.transport }).getTemplate('fantasy_v1'),
    ).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: a capture whose field type drifts is refused rather than coerced (API-36)', async () => {
    const mutated = structuredClone(health);
    const body = mutated.response?.body as { data: Record<string, unknown> };
    body.data['status'] = 42;

    const fake = createFakeTransport({
      status: mutated.response?.status ?? 200,
      body: JSON.stringify(mutated.response?.body),
    });

    await expect(
      createApiClient({ baseUrl, transport: fake.transport }).health(),
    ).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Edge: no capture carries a credential (API-35)', () => {
    for (const name of fixtureNames) {
      const raw = readFileSync(path.join(fixturesDir, `${name}.json`), 'utf8');

      expect(raw).not.toMatch(/access_token|refresh_token|Bearer\s/);
    }
  });
});
