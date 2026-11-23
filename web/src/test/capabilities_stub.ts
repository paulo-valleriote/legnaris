import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { errorOutcome, PAID_PLAN_USAGE, successOutcome } from './fake_transport';
import {
  CAPABILITIES_ACTION_ID,
  CAPABILITIES_SHEET_ID,
  CAPABILITIES_TEMPLATE_ID,
  capabilitiesSheetPayload,
  capabilitiesTemplatePayload,
} from './capabilities_fixture';

export type CapabilitiesAnswer =
  HttpResponse | { readonly failure: unknown } | (() => Promise<HttpResponse>);

export interface CapabilitiesStubOptions {
  readonly template?: unknown;
  readonly sheet?: unknown;
  readonly refreshedSheet?: unknown;
  readonly catalog?: readonly unknown[];
  readonly createdSheet?: unknown;
  readonly resets?: readonly CapabilitiesAnswer[];
  readonly patch?: () => Promise<HttpResponse>;
}

export interface CapabilitiesStub {
  readonly transport: Transport;
  readonly requests: HttpRequest[];
  readonly resetRequests: () => HttpRequest[];
  readonly patchRequests: () => HttpRequest[];
  readonly createRequests: () => HttpRequest[];
  readonly sheetFetches: () => number;
}

function scripted(
  answers: readonly CapabilitiesAnswer[] | undefined,
  index: number,
): HttpResponse | { readonly failure: unknown } | (() => Promise<HttpResponse>) {
  return answers === undefined || answers.length === 0
    ? successOutcome({})
    : (answers[Math.min(index, answers.length - 1)] as CapabilitiesAnswer);
}

export function createCapabilitiesStub(options: CapabilitiesStubOptions = {}): CapabilitiesStub {
  const requests: HttpRequest[] = [];
  let resets = 0;
  let sheets = 0;

  const transport: Transport = (request) => {
    requests.push(request);
    const path = new URL(request.url).pathname;

    if (
      request.method === 'POST' &&
      path === `/sheets/${CAPABILITIES_SHEET_ID}/resets/${CAPABILITIES_ACTION_ID}`
    ) {
      const answer = scripted(options.resets, resets);
      resets += 1;
      if (typeof answer === 'function') {
        return answer();
      }
      return 'failure' in answer
        ? Promise.reject(answer.failure)
        : Promise.resolve(answer as HttpResponse);
    }

    if (request.method === 'GET' && path === `/sheets/${CAPABILITIES_SHEET_ID}`) {
      sheets += 1;
      const payload =
        sheets > 1 && options.refreshedSheet !== undefined
          ? options.refreshedSheet
          : (options.sheet ?? capabilitiesSheetPayload());
      return Promise.resolve(successOutcome(payload));
    }

    if (request.method === 'GET' && path === '/templates') {
      return Promise.resolve(successOutcome(options.catalog ?? [capabilitiesTemplatePayload()]));
    }

    if (request.method === 'GET' && path === `/templates/${CAPABILITIES_TEMPLATE_ID}`) {
      return Promise.resolve(successOutcome(options.template ?? capabilitiesTemplatePayload()));
    }

    if (request.method === 'POST' && path === '/sheets') {
      return Promise.resolve(
        successOutcome(options.createdSheet ?? capabilitiesSheetPayload(), 201),
      );
    }

    if (request.method === 'PATCH' && path === `/sheets/${CAPABILITIES_SHEET_ID}`) {
      return options.patch === undefined ? Promise.resolve(successOutcome({})) : options.patch();
    }

    if (request.method === 'GET' && path === '/me/plan') {
      return Promise.resolve(successOutcome(PAID_PLAN_USAGE));
    }

    return Promise.resolve(
      errorOutcome('NOT_FOUND', `no route for ${request.method} ${path}`, 404),
    );
  };

  return {
    transport,
    requests,
    resetRequests: () =>
      requests.filter((request) => request.method === 'POST' && request.url.includes('/resets/')),
    patchRequests: () => requests.filter((request) => request.method === 'PATCH'),
    createRequests: () =>
      requests.filter(
        (request) => request.method === 'POST' && new URL(request.url).pathname === '/sheets',
      ),
    sheetFetches: () =>
      requests.filter(
        (request) =>
          request.method === 'GET' &&
          new URL(request.url).pathname === `/sheets/${CAPABILITIES_SHEET_ID}`,
      ).length,
  };
}
