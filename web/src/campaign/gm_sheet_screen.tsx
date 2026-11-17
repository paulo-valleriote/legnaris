import { useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { UseQueryResult } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { FieldNode } from '../api/models/template';
import { valueAtPath } from '../api/models/sheet';
import type { Sheet } from '../api/models/sheet';
import { useApiClient } from '../app/providers';
import { templateDetailKey } from '../catalog/queries';
import { message, messageWith } from '../l10n/strings';
import type { GmSheetSnapshot } from './campaign_queries';
import { npcLinksKey, useGmNpcSheet, useGmSheet } from './campaign_queries';
import './campaign.css';

function formatValue(value: unknown): string {
  if (value === undefined || value === null || value === '') {
    return message('gm.emptyValue');
  }
  return typeof value === 'object' ? JSON.stringify(value) : String(value);
}

function FieldRows({
  nodes,
  prefix,
  data,
}: {
  nodes: Record<string, FieldNode>;
  prefix: string;
  data: Record<string, unknown>;
}) {
  return (
    <>
      {Object.entries(nodes).map(([key, node]) => {
        const path = prefix === '' ? key : `${prefix}.${key}`;
        if (node.kind === 'group') {
          return (
            <section className="lg-campaign__group" key={path} aria-label={node.label ?? key}>
              <h3 className="lg-campaign__subheading">{node.label ?? key}</h3>
              <FieldRows nodes={node.children} prefix={path} data={data} />
            </section>
          );
        }
        return (
          <div className="lg-campaign__field" key={path}>
            <dt className="lg-campaign__name">{node.schema.label}</dt>
            <dd className="lg-campaign__value">{formatValue(readValue(data, path))}</dd>
          </div>
        );
      })}
    </>
  );
}

function readValue(data: Record<string, unknown>, path: string): unknown {
  try {
    return valueAtPath(data, path);
  } catch {
    return undefined;
  }
}

function SheetFields({ sheet }: { sheet: Sheet }) {
  const client = useApiClient();
  const template = useQuery({
    queryKey: templateDetailKey(sheet.templateId),
    queryFn: () => client.getTemplate(sheet.templateId),
    retry: false,
  });

  if (template.isError) {
    return (
      <p className="lg-campaign__error" role="alert">
        {failureCopy(template.error)}
      </p>
    );
  }
  if (template.data === undefined) {
    return (
      <p className="lg-campaign__message" role="status">
        {message('gm.loading')}
      </p>
    );
  }

  return (
    <dl className="lg-campaign__fields">
      <FieldRows nodes={template.data.fields} prefix="" data={sheet.data} />
    </dl>
  );
}

export function GmSheetScreen() {
  const { id = '', sheetId = '' } = useParams();
  const sheet = useGmSheet(id, sheetId);

  return <GmSheetPage campaignId={id} heading={message('gm.heading')} sheet={sheet} />;
}

export function GmNpcSheetScreen() {
  const { id = '', sheetId = '' } = useParams();
  const sheet = useGmNpcSheet(id, sheetId);
  const queryClient = useQueryClient();
  const unavailable = sheet.isError && failureKind(sheet.error) === 'notFound';

  useEffect(() => {
    if (unavailable) {
      void queryClient.invalidateQueries({ queryKey: npcLinksKey(id) });
    }
  }, [id, queryClient, unavailable]);

  return (
    <GmSheetPage
      campaignId={id}
      heading={message('gm.npcHeading')}
      sheet={sheet}
      unavailable={unavailable ? message('gm.npcUnavailable') : undefined}
    />
  );
}

interface GmSheetPageProps {
  readonly campaignId: string;
  readonly heading: string;
  readonly sheet: UseQueryResult<GmSheetSnapshot, Error>;
  readonly unavailable?: string | undefined;
}

function GmSheetPage({ campaignId, heading, sheet, unavailable }: GmSheetPageProps) {
  return (
    <div className="lg-campaign">
      <p className="lg-campaign__back">
        <Link to={`/campaigns/${campaignId}`}>{message('campaign.back')}</Link>
      </p>
      <h1 className="lg-campaign__title">{heading}</h1>
      <p className="lg-campaign__caption">{message('gm.caption')}</p>
      <button
        type="button"
        className="lg-button lg-button--outline lg-button--sm"
        disabled={sheet.isFetching}
        aria-busy={sheet.isFetching ? true : undefined}
        onClick={() => void sheet.refetch()}
      >
        {sheet.isFetching ? message('gm.refreshing') : message('gm.refresh')}
      </button>
      {sheet.isPending ? (
        <p className="lg-campaign__message" role="status">
          {message('gm.loading')}
        </p>
      ) : null}
      {unavailable ? (
        <p className="lg-campaign__error" role="alert">
          {unavailable}
        </p>
      ) : sheet.isError ? (
        <p className="lg-campaign__error" role="alert">
          {failureCopy(sheet.error)}
        </p>
      ) : null}
      {sheet.data === undefined || unavailable ? null : (
        <>
          <h2 className="lg-campaign__heading">{sheet.data.sheet.name}</h2>
          <p className="lg-campaign__meta">
            {messageWith('gm.template', { name: sheet.data.sheet.templateId })}
          </p>
          <SheetFields sheet={sheet.data.sheet} />
        </>
      )}
    </div>
  );
}
