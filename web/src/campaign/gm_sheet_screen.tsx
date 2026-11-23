import { useEffect } from 'react';
import type { ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { UseQueryResult } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { failureCopy, failureKind } from '../api/error_mapper';
import type { FieldNode, TemplateDetail } from '../api/models/template';
import { valueAtPath } from '../api/models/sheet';
import type { Sheet } from '../api/models/sheet';
import { useApiClient } from '../app/providers';
import { templateDetailKey } from '../catalog/queries';
import { message, messageWith } from '../l10n/strings';
import { Button } from '../ui/button';
import { PageHeader } from '../ui/page_header';
import { ErrorState, LoadingState } from '../ui/state';
import type { GmSheetSnapshot } from './campaign_queries';
import { npcLinksKey, useNpcSheet, useGmSheet } from './campaign_queries';
import './campaign.css';

export function formatValue(value: unknown): string {
  if (value === undefined || value === null || value === '') {
    return message('gm.emptyValue');
  }
  if (Array.isArray(value)) {
    return value.length === 0 ? message('gm.emptyValue') : value.map(formatValue).join(', ');
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>);
    return entries.length === 0
      ? message('gm.emptyValue')
      : entries.map(([key, item]) => `${key}: ${formatValue(item)}`).join(', ');
  }
  return String(value);
}

function readValue(data: Record<string, unknown>, path: string): unknown {
  try {
    return valueAtPath(data, path);
  } catch {
    return undefined;
  }
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
  const blocks: ReactNode[] = [];
  let leaves: ReactNode[] = [];

  const flushLeaves = () => {
    if (leaves.length === 0) {
      return;
    }
    blocks.push(
      <dl className="lg-campaign__field-list" key={`field-list-${blocks.length}`}>
        {leaves}
      </dl>,
    );
    leaves = [];
  };

  for (const [key, node] of Object.entries(nodes)) {
    const path = prefix === '' ? key : `${prefix}.${key}`;
    if (node.kind === 'group') {
      flushLeaves();
      blocks.push(
        <div className="lg-campaign__group" key={path}>
          <h3 className="lg-campaign__subheading">{node.label ?? key}</h3>
          <FieldRows nodes={node.children} prefix={path} data={data} />
        </div>,
      );
      continue;
    }
    if (node.kind === 'list') {
      flushLeaves();
      continue;
    }
    leaves.push(
      <div className="lg-campaign__field" key={path}>
        <dt className="lg-campaign__field-label">{node.schema.label}</dt>
        <dd className="lg-campaign__value">{formatValue(readValue(data, path))}</dd>
      </div>,
    );
  }

  flushLeaves();

  return <>{blocks}</>;
}

function useSheetTemplate(templateId: string | undefined) {
  const client = useApiClient();

  return useQuery({
    queryKey: templateDetailKey(templateId ?? ''),
    queryFn: () => client.getTemplate(templateId ?? ''),
    enabled: templateId !== undefined,
    retry: false,
  });
}

function SheetFields({
  template,
  sheet,
}: {
  template: UseQueryResult<TemplateDetail, Error>;
  sheet: Sheet;
}) {
  if (template.isError) {
    return <ErrorState message={failureCopy(template.error)} />;
  }
  if (template.data === undefined) {
    return <LoadingState label={message('gm.loading')} />;
  }

  return (
    <div className="lg-campaign__fields">
      <FieldRows nodes={template.data.fields} prefix="" data={sheet.data} />
    </div>
  );
}

export function GmSheetScreen() {
  const { id = '', sheetId = '' } = useParams();
  const sheet = useGmSheet(id, sheetId);

  return <GmSheetPage campaignId={id} heading={message('gm.heading')} sheet={sheet} />;
}

export function GmNpcSheetScreen() {
  const { id = '', sheetId = '' } = useParams();
  const sheet = useNpcSheet(id, sheetId);
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
  const templateId = sheet.data?.sheet.templateId;
  const template = useSheetTemplate(templateId);
  const templateLabel =
    templateId === undefined
      ? undefined
      : messageWith('gm.template', { name: template.data?.name ?? templateId });

  return (
    <div className="lg-campaign">
      <PageHeader
        title={heading}
        backTo={`/campaigns/${campaignId}`}
        backLabel={message('campaign.back')}
        meta={templateLabel === undefined ? undefined : <span>{templateLabel}</span>}
        actions={
          <Button
            variant="outline"
            size="sm"
            label={sheet.isFetching ? message('gm.refreshing') : message('gm.refresh')}
            loading={sheet.isFetching}
            onClick={() => void sheet.refetch()}
          />
        }
      />
      {sheet.isPending ? <LoadingState label={message('gm.loading')} /> : null}
      {unavailable ? (
        <ErrorState message={unavailable} />
      ) : sheet.isError ? (
        <ErrorState message={failureCopy(sheet.error)} />
      ) : null}
      {sheet.data === undefined || unavailable ? null : (
        <>
          <h2 className="lg-campaign__heading">{sheet.data.sheet.name}</h2>
          <p className="lg-campaign__caption">{message('gm.caption')}</p>
          <SheetFields template={template} sheet={sheet.data.sheet} />
        </>
      )}
    </div>
  );
}
