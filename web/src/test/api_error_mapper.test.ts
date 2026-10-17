import { describe, expect, it } from 'vitest';
import {
  apperrKind,
  classifyDomainFailure,
  failureCopy,
  failureMessage,
  offlineFailure,
  parseFailureMessage,
} from '../api/error_mapper';
import { DomainFailure } from '../api/errors';
import type { FailureKind } from '../api/errors';
import { messages } from '../l10n/strings';
import type { MessageKey } from '../l10n/strings';

const dedicated: ReadonlyArray<readonly [string, FailureKind]> = [
  ['INVALID_JSON', 'clientFault'],
  ['INVALID_REQUEST', 'invalidEdit'],
  ['UNKNOWN_FIELD', 'renderMismatch'],
  ['NOT_FOUND', 'notFound'],
  ['INTERNAL_ERROR', 'serverError'],
  ['UNAUTHORIZED', 'unauthorized'],
  ['FORBIDDEN', 'forbidden'],
  ['RATE_LIMITED', 'rateLimited'],
];

const fallback = [
  'CONFLICT',
  'CYCLE_DETECTED',
  'INVALID_DEPENDENCY',
  'INVALID_PATH',
  'SHEET_LIMIT_REACHED',
] as const;

const keyByKind: Record<FailureKind, MessageKey> = {
  clientFault: 'api.error.clientFault',
  invalidEdit: 'api.error.invalidEdit',
  renderMismatch: 'api.error.renderMismatch',
  notFound: 'api.error.notFound',
  serverError: 'api.error.serverError',
  offline: 'api.error.offline',
  unauthorized: 'api.error.unauthorized',
  forbidden: 'api.error.forbidden',
  rateLimited: 'api.error.rateLimited',
  unknown: 'api.error.unknown',
};

describe('backend code to failure kind', () => {
  it('Happy path: each code with a dedicated kind maps to it (API-15, API-16, API-17, API-18, API-19)', () => {
    for (const [code, kind] of dedicated) {
      expect(apperrKind(code)).toBe(kind);
    }
  });

  it('Edge: every fallback code takes the generic kind (API-21)', () => {
    for (const code of fallback) {
      expect(apperrKind(code)).toBe('unknown');
    }
  });

  it('Edge: a code the backend does not define takes the generic kind (API-21)', () => {
    expect(apperrKind('SOMETHING_NEW')).toBe('unknown');
  });

  it('Edge: an empty code cannot masquerade as a mapped one (API-21)', () => {
    expect(apperrKind('')).toBe('unknown');
  });
});

describe('classifying a domain failure', () => {
  it('Happy path: a failure with a dedicated code gets that kind, keeping status, code and message (API-15, API-24)', () => {
    const classified = classifyDomainFailure(
      new DomainFailure({
        kind: 'unknown',
        status: 400,
        code: 'INVALID_REQUEST',
        message: 'bad edit',
      }),
    );

    expect(classified.kind).toBe('invalidEdit');
    expect(classified.status).toBe(400);
    expect(classified.code).toBe('INVALID_REQUEST');
    expect(classified.message).toBe('bad edit');
  });

  it('Edge: an unmapped code keeps its raw code and the server message with the generic kind (API-21)', () => {
    const classified = classifyDomainFailure(
      new DomainFailure({
        kind: 'unknown',
        status: 409,
        code: 'CONFLICT',
        message: 'already exists',
      }),
    );

    expect(classified.kind).toBe('unknown');
    expect(classified.code).toBe('CONFLICT');
    expect(classified.message).toBe('already exists');
  });

  it('Edge: a failure with no code is classified as generic without inventing one (API-21)', () => {
    const classified = classifyDomainFailure(new DomainFailure({ kind: 'unknown', status: 500 }));

    expect(classified.kind).toBe('unknown');
    expect(classified.code).toBeUndefined();
  });

  it('Happy path: the classified failure is a new domain failure, not the input (API-24)', () => {
    const original = new DomainFailure({
      kind: 'unknown',
      status: 404,
      code: 'NOT_FOUND',
      message: 'gone',
    });

    const classified = classifyDomainFailure(original);

    expect(classified).not.toBe(original);
    expect(classified).toBeInstanceOf(DomainFailure);
  });

  it('Happy path: a transport failure becomes the offline kind with no status or code (API-20)', () => {
    const offline = offlineFailure();

    expect(offline.kind).toBe('offline');
    expect(offline.status).toBeUndefined();
    expect(offline.code).toBeUndefined();
  });
});

describe('failure copy', () => {
  it('Happy path: each kind resolves to its catalog entry byte-for-byte (API-23)', () => {
    for (const kind of Object.keys(keyByKind) as FailureKind[]) {
      expect(failureMessage(kind)).toBe(messages[keyByKind[kind]]);
    }
  });

  it('Happy path: the defensive-parse copy resolves from the catalog (API-22, API-23)', () => {
    expect(parseFailureMessage()).toBe(messages['api.error.parse']);
  });

  it('Edge: no two kinds resolve to the same copy (API-23)', () => {
    const copy = (Object.keys(keyByKind) as FailureKind[]).map((kind) => failureMessage(kind));

    expect(new Set(copy).size).toBe(copy.length);
  });
});

describe('session code classification (WAS-30, SC-008, SC-009)', () => {
  it('Edge: UNAUTHORIZED, FORBIDDEN and RATE_LIMITED map to their dedicated kinds (WAS-30)', () => {
    expect(apperrKind('UNAUTHORIZED')).toBe('unauthorized');
    expect(apperrKind('FORBIDDEN')).toBe('forbidden');
    expect(apperrKind('RATE_LIMITED')).toBe('rateLimited');
  });

  it('Edge: the three session kinds resolve their catalog copy byte-for-byte (WAS-30, SC-008)', () => {
    expect(failureMessage('unauthorized')).toBe(messages['api.error.unauthorized']);
    expect(failureMessage('forbidden')).toBe(messages['api.error.forbidden']);
    expect(failureMessage('rateLimited')).toBe(messages['api.error.rateLimited']);
  });

  it('Edge: failureCopy resolves a catalog string and never echoes the server message (SC-009)', () => {
    const failure = classifyDomainFailure(
      new DomainFailure({
        kind: 'unknown',
        status: 500,
        code: 'INTERNAL_ERROR',
        message: 'traceback: secret internal detail',
      }),
    );

    expect(failureCopy(failure)).toBe(messages['api.error.serverError']);
    expect(failureCopy(failure)).not.toContain('traceback');
  });
});
