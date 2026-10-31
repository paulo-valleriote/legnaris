import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { errorOutcome, successOutcome } from './fake_transport';
import { SHEET_ID, TEMPLATE_ID, sheetPayload } from './sheet_fixtures';

function leaf(label: string, extra: Record<string, unknown> = {}) {
  return {
    schema: { type: 'number', label, is_editable: true, default: 0, ...extra },
  };
}

/** The wire template the Arrange suites open: Alpha(One, Two, Three), Beta(Note, Stats table, Pick), Gamma(Solo). */
export function arrangeTemplatePayload(labels: Partial<Record<'alpha' | 'one', string>> = {}) {
  return {
    id: TEMPLATE_ID,
    name: 'Fantasy',
    default: true,
    fields: {
      alpha: {
        label: labels.alpha ?? 'Alpha',
        children: {
          one: leaf(labels.one ?? 'One'),
          two: leaf('Two'),
          three: leaf('Three'),
        },
      },
      beta: {
        label: 'Beta',
        children: {
          note: { schema: { type: 'text', label: 'Note', is_editable: true, default: 0 } },
          stats: {
            label: 'Stats',
            columns: ['str', 'dex'],
            children: { str: leaf('Strength'), dex: leaf('Dexterity') },
          },
          pickA: leaf('Pick A', { exclusive_group: 'pick', option_label: 'A' }),
          pickB: leaf('Pick B', { exclusive_group: 'pick', option_label: 'B' }),
        },
      },
      gamma: { label: 'Gamma', children: { solo: leaf('Solo') } },
    },
  };
}

export interface ArrangeStub {
  readonly transport: Transport;
  readonly requests: HttpRequest[];
  puts(): HttpRequest[];
  patches(): HttpRequest[];
}

export interface ArrangeStubOptions {
  readonly layout?: unknown;
  readonly data?: Record<string, unknown>;
  readonly template?: unknown;
  /** Answers of successive PUTs; the last repeats. Defaults to echoing the request body. */
  readonly putOutcomes?: HttpResponse[];
}

/** A routing transport: GET sheet/template, PUT layout, PATCH values. */
export function createArrangeStub(options: ArrangeStubOptions = {}): ArrangeStub {
  const requests: HttpRequest[] = [];
  let putCount = 0;

  const transport: Transport = (request) => {
    requests.push(request);
    const url = request.url;
    if (request.method === 'PUT') {
      const scripted = options.putOutcomes;
      if (scripted !== undefined) {
        const outcome = scripted[Math.min(putCount, scripted.length - 1)];
        putCount += 1;
        return Promise.resolve(outcome);
      }
      const body = JSON.parse(request.body ?? '{}') as { layout: unknown };
      return Promise.resolve(successOutcome({ layout: body.layout }));
    }
    if (request.method === 'PATCH') {
      return Promise.resolve(successOutcome({}));
    }
    if (url.endsWith(`/sheets/${SHEET_ID}`)) {
      const overrides = options.layout === undefined ? {} : { layout: options.layout };
      return Promise.resolve(
        successOutcome(sheetPayload({ data: options.data ?? {}, ...overrides })),
      );
    }
    if (url.includes('/templates/')) {
      return Promise.resolve(successOutcome(options.template ?? arrangeTemplatePayload()));
    }
    return Promise.resolve(errorOutcome('NOT_FOUND', 'missing', 404));
  };

  return {
    transport,
    requests,
    puts: () => requests.filter((request) => request.method === 'PUT'),
    patches: () => requests.filter((request) => request.method === 'PATCH'),
  };
}
